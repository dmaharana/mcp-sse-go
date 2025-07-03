package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"mcp-sse-go/pkg/tools"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	weatherURL := r.Header.Get("X-Weather-URL")

	// In a real application, you would validate the API key
	log.Info().Str("apiKey", apiKey).Msg("API Key received")

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// For now, we'll just echo back the request body as a demonstration
	// In a real implementation, you would process the request and call tools
	fmt.Fprintf(w, "data: %s\n\n", body)
	flusher.Flush()

	// Example of calling the weather tool
	if weatherURL != "" {
		weatherTool := tools.NewWeatherTool(weatherURL, apiKey)
		weather, err := weatherTool.Call(json.RawMessage(`{"city": "London"}`))
		if err != nil {
			log.Error().Err(err).Msg("could not call weather tool")
			fmt.Fprintf(w, "data: {\"error\": \"could not call weather tool\"}\n\n")
		} else {
			fmt.Fprintf(w, "data: %s\n\n", weather)
		}
		flusher.Flush()
	}
}