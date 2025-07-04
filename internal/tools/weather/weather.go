package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Args represents the arguments for the weather tool.
type Args struct {
	City string `json:"city"`
}

// Context keys for storing request-specific values
type contextKey string

const (
	// ContextKeyAPIURL is the key for the API URL in the context
	ContextKeyAPIURL contextKey = "api_url"
	// ContextKeyAPIKey is the key for the API key in the context
	ContextKeyAPIKey contextKey = "api_key"
)

// WeatherTool is a tool that provides weather information.
type WeatherTool struct{}

// NewWeatherTool creates a new WeatherTool instance.
func NewWeatherTool() *WeatherTool {
	tool := &WeatherTool{}
	// Log the creation of the weather tool
	// Note: In a production environment, you might want to use a proper logger
	// For now, we'll use the standard log package
	log.Printf("Creating new WeatherTool instance with name: %s", tool.Name())
	return tool
}

// Name returns the name of the tool.
func (t *WeatherTool) Name() string {
	return "weather"
}

// Call executes the weather tool with the given arguments.
func (t *WeatherTool) Call(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	// Parse arguments
	var params Args
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.City == "" {
		return nil, fmt.Errorf("city is required")
	}

	// Get API URL and key from context
	apiURL, ok := ctx.Value(ContextKeyAPIURL).(string)
	if !ok || apiURL == "" {
		return nil, fmt.Errorf("missing or invalid API URL in context")
	}

	apiKey, ok := ctx.Value(ContextKeyAPIKey).(string)
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("missing or invalid API key in context")
	}

	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add query parameters
	q := req.URL.Query()
	q.Add("q", params.City)
	req.URL.RawQuery = q.Encode()

	// Add API key header
	req.Header.Add("X-API-Key", apiKey)

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
