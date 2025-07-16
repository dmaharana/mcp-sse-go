package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"mcp-sse-go/internal/mcp"
	"mcp-sse-go/internal/session"
	"mcp-sse-go/internal/telemetry"
	"mcp-sse-go/internal/tools"
	"mcp-sse-go/internal/tools/weather"
)

//go:embed web/static/*
var staticFS embed.FS

// IDEConfig represents the IDE configuration structure
type IDEConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// Config contains the server configuration.
type Config struct {
	// Session configuration
	SessionTimeout    time.Duration `yaml:"session_timeout" default:"1h"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval" default:"5m"`
	RequireSession    bool          `yaml:"require_session" default:"true"`
	LogLevel          string        `yaml:"log_level" default:"info"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		SessionTimeout:  time.Hour,
		CleanupInterval: 5 * time.Minute,
		RequireSession:  true,
		LogLevel:        "info",
	}
}

// fileServer is a wrapper around http.FileServer that works with embedded files
func fileServer(r chi.Router, path string, root fs.FS) {
	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(http.FS(root)))
		fs.ServeHTTP(w, r)
	})
}

// getBaseURL extracts the base URL from the request
func getBaseURL(r *http.Request) string {
	scheme := "http://"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https://"
	}
	return scheme + r.Host
}

// New creates a new HTTP handler with the given configuration.
func New(cfg Config) (http.Handler, error) {
	// Set up logger
	logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	if cfg.LogLevel != "" {
		level, err := zerolog.ParseLevel(cfg.LogLevel)
		if err == nil {
			logger = logger.Level(level)
		}
	}

	// Create telemetry metrics
	metrics := telemetry.NewMetrics()

	// Create system metrics collector
	ctx := context.Background()
	systemCollector := telemetry.NewSystemMetricsCollector(metrics, logger, 30*time.Second)
	go systemCollector.Start(ctx)

	// Create session management components
	sessionStore := session.NewMemoryStore(logger)
	sessionManagerConfig := session.ManagerConfig{
		SessionTimeout: cfg.SessionTimeout,
	}
	baseSessionManager := session.NewDefaultSessionManager(sessionStore, sessionManagerConfig, logger)
	
	// Wrap session manager with telemetry
	sessionManager := telemetry.NewSessionManagerWrapper(baseSessionManager, metrics)

	// Create cleanup service
	cleanupConfig := session.CleanupConfig{
		CleanupInterval: cfg.CleanupInterval,
	}
	cleanupService := session.NewCleanupService(sessionManager, cleanupConfig, logger)

	// Start cleanup service
	if err := cleanupService.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start cleanup service: %w", err)
	}

	// Create session middleware
	middlewareConfig := session.DefaultMiddlewareConfig()
	middlewareConfig.RequireSession = cfg.RequireSession
	sessionMiddleware := session.NewSessionMiddleware(sessionManager, middlewareConfig, logger)

	// Create session handler
	sessionHandler := session.NewSessionHandler(sessionManager, logger)

	// Create tool registry
	baseToolRegistry := tools.NewRegistry()
	
	// Wrap tool registry with telemetry
	toolRegistry := telemetry.NewToolRegistryWrapper(baseToolRegistry, metrics)

	// Register weather tool
	weatherTool := weather.NewWeatherTool()
	baseToolRegistry.Register(weatherTool)
	log.Printf("Registered tool: %s", weatherTool.Name())

	// List all registered tools for debugging
	toolList := toolRegistry.List()
	log.Printf("Total tools registered: %d", len(toolList))
	for name, tool := range toolList {
		log.Printf(" - %s (%T)", name, tool)
	}

	// Create MCP handler
	mcpHandler := mcp.NewHandler(baseToolRegistry, metrics)

	// Create router
	r := chi.NewRouter()

	// Add middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(telemetry.HTTPMetricsMiddleware(metrics))

	// Enable CORS with session header support
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Weather-API-URL", "X-Weather-API-Key", "Mcp-Session-Id"},
		ExposedHeaders:   []string{"Link", "Content-Type", "Cache-Control", "Connection", "Mcp-Session-Id"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Add session middleware (after CORS but before protected routes)
	r.Use(sessionMiddleware.Handler())

	// Serve static files from embedded filesystem
	staticRoot, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		return nil, fmt.Errorf("failed to create sub filesystem: %w", err)
	}

	// Add routes
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// IDE Configuration endpoint
	r.Get("/.mcp/ide-config", func(w http.ResponseWriter, r *http.Request) {
		baseURL := getBaseURL(r)
		config := IDEConfig{
			URL: baseURL + "/sse",
			Headers: map[string]string{
				"X-Weather-API-URL": "https://api.weatherapi.com/v1",
				"X-Weather-API-Key":  "YOUR_TOKEN",
			},
		}
		render.JSON(w, r, map[string]interface{}{
			"my-mcp-server": config,
		})
	})

	// Configuration page
	r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile("web/static/config.xhtml")
		if err != nil {
			http.Error(w, "Configuration page not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/xhtml+xml")
		w.Write(data)
	})

	// Session management endpoints
	r.Post("/sessions", sessionHandler.CreateSession)
	r.Get("/sessions", sessionHandler.GetSession)
	r.Delete("/sessions", sessionHandler.DeleteSession)
	r.Put("/sessions/refresh", sessionHandler.RefreshSession)
	r.Get("/sessions/stats", sessionHandler.GetSessionStats)

	// Serve static files
	fileServer(r, "/static", staticRoot)

	// Handle both GET and POST for MCP endpoint
	r.Get("/sse", mcpHandler.Handle)
	r.Post("/sse", mcpHandler.Handle)

	return r, nil
}
