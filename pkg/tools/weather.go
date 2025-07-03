package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// WeatherTool is a tool that calls a weather API.
type WeatherTool struct {
	url    string
	apiKey string
}

// NewWeatherTool creates a new WeatherTool.
func NewWeatherTool(url, apiKey string) *WeatherTool {
	return &WeatherTool{url: url, apiKey: apiKey}
}

// Name returns the name of the tool.
func (t *WeatherTool) Name() string {
	return "weather"
}

// Call calls the weather API.
type weatherArgs struct {
	City string `json:"city"`
}

func (t *WeatherTool) Call(args json.RawMessage) (json.RawMessage, error) {
	var parsedArgs weatherArgs
	if err := json.Unmarshal(args, &parsedArgs); err != nil {
		return nil, fmt.Errorf("could not parse args: %w", err)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s?q=%s", t.url, parsedArgs.City), nil)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Set("X-API-Key", t.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body: %w", err)
	}

	return json.RawMessage(body), nil
}
