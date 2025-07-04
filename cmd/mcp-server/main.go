package main

import (
	"net/http"
	"os"

	"github.com/rs/zerolog"

	"mcp-sse-go/internal/server"
)

func main() {
	// Configure logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel) // Set to DebugLevel to see all logs
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).
		With().
		Timestamp().
		Logger()

	logger.Info().Msg("Starting MCP SSE server with debug logging")

	// Configuration
	cfg := server.Config{}

	// Create server
	handler, err := server.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create server")
	}

	// Start server
	addr := ":8080"
	server := &http.Server{
		Addr:     addr,
		Handler:  handler,
	}

	logger.Info().Str("addr", addr).Msg("Starting server")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}
