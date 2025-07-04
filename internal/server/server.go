package server

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"mcp-sse-go/internal/mcp"
	"mcp-sse-go/internal/tools"
	"mcp-sse-go/internal/tools/weather"
)

// Config contains the server configuration.
type Config struct {
	// No configuration needed as API key and URL come from headers
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

	// Add routes
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Handle both GET and POST for MCP endpoint
	r.Get("/sse", mcpHandler.Handle)
	r.Post("/sse", mcpHandler.Handle)

	return r, nil
}
