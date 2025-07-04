package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mcp-sse-go/internal/tools"
	"net/http"
	"net/url"
	"strings"
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
type WeatherTool struct {
	*tools.DefaultTool
}

// NewWeatherTool creates a new WeatherTool instance.
func NewWeatherTool() *WeatherTool {
	tool := &WeatherTool{
		DefaultTool: tools.NewDefaultTool("weather", "Get current weather for a city"),
	}
	// Log the creation of the weather tool
	log.Printf("Creating new WeatherTool instance with name: %s", tool.Name())
	return tool
}

// GetToolDefinition returns the tool definition in MCP format
func (t *WeatherTool) GetToolDefinition() map[string]any {
	// Get the default tool definition
	def := t.DefaultTool.GetToolDefinition()
	
	// Override with weather-specific schema
	def["inputSchema"] = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "The city to get weather for",
			},
		},
		"required": []string{"city"},
	}
	
	return def
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

	// Construct the full URL with query parameters
	fullURL := fmt.Sprintf("%s/current.json?key=%s&q=%s&aqi=no", 
		strings.TrimSuffix(apiURL, "/"),
		url.QueryEscape(apiKey),
		url.QueryEscape(params.City),
	)

	// Create request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")

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

	// Parse the weather data
	var weatherData struct {
		Location struct {
			Name    string `json:"name"`
			Region  string `json:"region"`
			Country string `json:"country"`
		} `json:"location"`
		Current struct {
			TempC     float64 `json:"temp_c"`
			TempF     float64 `json:"temp_f"`
			Condition struct {
				Text string `json:"text"`
			} `json:"condition"`
			Humidity  int     `json:"humidity"`
			WindKPH   float64 `json:"wind_kph"`
			FeelsLikeC float64 `json:"feelslike_c"`
		} `json:"current"`
	}

	if err := json.Unmarshal(body, &weatherData); err != nil {
		return nil, fmt.Errorf("failed to parse weather data: %w", err)
	}

	// Format the response as markdown
	markdown := fmt.Sprintf(`# 🌤️ Weather in %s, %s, %s
**Temperature:** %.1f°C (%.1f°F) - Feels like %.1f°C
**Condition:** %s
**Humidity:** %d%%
**Wind:** %.1f km/h`,
		weatherData.Location.Name,
		weatherData.Location.Region,
		weatherData.Location.Country,
		weatherData.Current.TempC,
		weatherData.Current.TempF,
		weatherData.Current.FeelsLikeC,
		weatherData.Current.Condition.Text,
		weatherData.Current.Humidity,
		weatherData.Current.WindKPH,
	)

	// The client expects a response with a specific structure
	// Create a response that matches the client's expected format
	response := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": markdown,
			},
		},
	}

	// Log the response for debugging
	log.Printf("Sending weather response: %+v", response)

	return json.Marshal(response)
}
