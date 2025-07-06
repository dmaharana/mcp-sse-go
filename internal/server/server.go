package server

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/render"

	"mcp-sse-go/internal/mcp"
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
	// No configuration needed as API key and URL come from headers
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
	// Create tool registry
	toolRegistry := tools.NewRegistry()

	// Register weather tool
	weatherTool := weather.NewWeatherTool()
	toolRegistry.Register(weatherTool)
	log.Printf("Registered tool: %s", weatherTool.Name())

	// List all registered tools for debugging
	toolList := toolRegistry.List()
	log.Printf("Total tools registered: %d", len(toolList))
	for name, tool := range toolList {
		log.Printf(" - %s (%T)", name, tool)
	}

	// Create MCP handler
	mcpHandler := mcp.NewHandler(toolRegistry)

	// Create router
	r := chi.NewRouter()

	// Add middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// Enable CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Weather-API-URL", "X-Weather-API-Key"},
		ExposedHeaders:   []string{"Link", "Content-Type", "Cache-Control", "Connection"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

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

	// Serve static files
	fileServer(r, "/static", staticRoot)

	// Handle both GET and POST for MCP endpoint
	r.Get("/sse", mcpHandler.Handle)
	r.Post("/sse", mcpHandler.Handle)

	return r, nil
}
