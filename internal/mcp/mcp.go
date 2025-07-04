package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"mcp-sse-go/internal/jsonrpc"
	"mcp-sse-go/internal/tools"
	"mcp-sse-go/internal/tools/weather"
)

// contextKey is a type for context keys.
type contextKey string

const (
	// HTTPRequestContextKey is the key used to store the HTTP request in the context.
	HTTPRequestContextKey contextKey = "http_request"
)

// Handler handles MCP protocol messages over HTTP.
type Handler struct {
	toolRegistry *tools.Registry
	logger       zerolog.Logger
}

// WithRequest adds the HTTP request to the context and returns the new context.
func WithRequest(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, HTTPRequestContextKey, req)
}

// GetRequestFromContext retrieves the HTTP request from the context.
func GetRequestFromContext(ctx context.Context) (*http.Request, bool) {
	req, ok := ctx.Value(HTTPRequestContextKey).(*http.Request)
	return req, ok
}

// NewHandler creates a new MCP handler.
func NewHandler(toolRegistry *tools.Registry) *Handler {
	// Log the number of tools registered
	toolList := toolRegistry.List()
	logger := log.With().
		Str("component", "mcp_handler").
		Int("tool_count", len(toolList)).
		Logger()

	// Log each registered tool
	for name := range toolList {
		logger = logger.With().Str("tool_"+name, "registered").Logger()
	}

	logger.Info().Msg("Created new MCP handler")

	return &Handler{
		toolRegistry: toolRegistry,
		logger:       logger,
	}
}

// Handle handles incoming HTTP requests.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	h.logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote", r.RemoteAddr).
		Str("user-agent", r.UserAgent()).
		Msg("Incoming request")

	// Log all headers for debugging
	headers := make(map[string]string)
	for k, v := range r.Header {
		headers[k] = strings.Join(v, ", ")
	}
	h.logger.Debug().
		Interface("headers", headers).
		Msg("Request headers")

	// Set CORS headers for all responses
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Handle CORS preflight requests
	if r.Method == http.MethodOptions {
		h.logger.Info().Msg("Handling OPTIONS preflight request")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Weather-API-URL, X-Weather-API-Key, Accept, Cache-Control")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Handle POST requests (JSON-RPC messages)
	if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		h.logger.Info().Msg("Handling JSON-RPC request")
		
		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to read request body")
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		h.logger.Debug().
			Str("body", string(body)).
			Msg("Raw request body")
		
		// Parse the JSON-RPC request
		var req jsonrpc.Request
		if err := json.Unmarshal(body, &req); err != nil {
			h.logger.Error().Err(err).Msg("Failed to decode JSON-RPC request")
			http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
			return
		}

		h.logger.Info().
			Str("method", req.Method).
			Interface("id", req.ID).
			Msg("Parsed JSON-RPC request")

		// Set up response headers
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Get flusher for SSE if this is an SSE connection
		var flusher http.Flusher
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			var ok bool
			flusher, ok = w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming not supported", http.StatusInternalServerError)
				return
			}
		}

		// Handle the initialization request
		if req.Method == "initialize" {
			h.logger.Info().Msg("Handling initialize request")
			// For initialization, we don't need SSE, just a regular HTTP response
			h.handleInitialize(w, nil, &req)
			return
		}

		// Handle the tools/list request
		if req.Method == "tools/list" {
			h.logger.Info().Msg("Handling tools/list request")
			h.handleToolsList(w, nil, &req) // Pass nil flusher for direct HTTP response
			return
		}

		// For other methods, set up SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Get flusher for SSE
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Handle other JSON-RPC methods
		h.logger.Info().Str("method", req.Method).Msg("Handling JSON-RPC method")
		h.handleRequest(w, flusher, &req, r.Context())
		return
	}

	// Handle GET requests (SSE connection)
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
	w.Header().Set("X-Accel-Buffering", "no") // Disable buffering for Nginx

	// Ensure we're dealing with an SSE request
	acceptHeader := r.Header.Get("Accept")
	if acceptHeader != "text/event-stream" {
		h.logger.Warn().
			Str("accept_header", acceptHeader).
			Msg("Request is not an SSE request")
		http.Error(w, "This endpoint requires an SSE connection (Accept: text/event-stream)", http.StatusBadRequest)
		return
	}

	h.logger.Info().Msg("Starting SSE connection")

	// Get flusher for SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create a channel to handle client disconnection
	notify := r.Context().Done()

	// Send initial SSE handshake
	h.logger.Debug().Msg("Sending SSE handshake")
	_, err := fmt.Fprintf(w, ": Welcome to MCP SSE Server\n\n")
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to send welcome message")
		return
	}
	flusher.Flush()

	// Send initialization response
	initResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"capabilities": map[string]interface{}{
				"toolUse": map[string]bool{
					"enabled": true,
				},
			},
		},
	}

	resp, err := json.Marshal(initResponse)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal initialization response")
		return
	}

	h.logger.Info().Str("response", string(resp)).Msg("Sending initialization response")
	_, err = fmt.Fprintf(w, "data: %s\n\n", resp)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to send initialization response")
		return
	}
	flusher.Flush()

	// Send a ping every 30 seconds to keep the connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Send initial ping
	_, err = fmt.Fprintf(w, "event: ping\ndata: %s\n\n", time.Now().Format(time.RFC3339))
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to send initial ping")
		return
	}
	flusher.Flush()

	h.logger.Info().Msg("SSE connection established")

	go func() {
		<-notify
		h.logger.Info().Msg("Client disconnected")
	}()

	// Check if this is an initialization request
	if r.Method == http.MethodPost && r.Header.Get("Content-Type") == "application/json" {
		// Handle JSON-RPC request
		var req jsonrpc.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Error().Err(err).Msg("Failed to decode JSON-RPC request")
			http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
			return
		}

		// Handle initialization request
		if req.Method == "initialize" {
			h.handleInitialize(w, flusher, &req)
			return
		}

		// Handle other JSON-RPC methods
		h.handleRequest(w, flusher, &req, r.Context())
		return
	}

	// Handle SSE connection
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Main connection loop
	for {
		select {
		case <-notify:
			h.logger.Info().Msg("Client disconnected")
			return
		case t := <-ticker.C:
			// Send periodic ping
			_, err = fmt.Fprintf(w, "event: ping\ndata: %s\n\n", t.Format(time.RFC3339))
			if err != nil {
				h.logger.Error().Err(err).Msg("Failed to send ping")
				return
			}
			flusher.Flush()
		}
	}
}

// handleRequestWithContext processes JSON-RPC requests with the provided context.
func (h *Handler) handleRequestWithContext(w http.ResponseWriter, flusher http.Flusher, req *jsonrpc.Request, ctx context.Context) {
	h.handleRequest(w, flusher, req, ctx)
}

// handleInitialize handles the initialize request according to MCP specification
func (h *Handler) handleInitialize(w http.ResponseWriter, flusher http.Flusher, req *jsonrpc.Request) {
	h.logger.Info().
		Str("method", req.Method).
		Interface("id", req.ID).
		Msg("Handling initialize request")

	// List all registered tools
	toolList := h.toolRegistry.List()
	h.logger.Info().
		Int("tool_count", len(toolList)).
		Msg("Found registered tools")

	toolDefs := make([]map[string]interface{}, 0, len(toolList))
	for _, tool := range toolList {
		toolName := tool.Name()
		h.logger.Info().
			Str("tool_name", toolName).
			Msg("Registering tool")

		// Create a tool definition according to MCP specification
		toolDef := map[string]interface{}{
			"name":        toolName,
			"title":       fmt.Sprintf("%s Tool", toolName),
			"description": fmt.Sprintf("A tool named %s", toolName),
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type":        "string",
						"description": "Input for the tool",
					},
				},
				"required": []string{"input"},
			},
		}
		toolDefs = append(toolDefs, toolDef)

		h.logger.Debug().
			Str("tool_name", toolName).
			Interface("definition", toolDef).
			Msg("Tool definition")
	}

	// Create the response with the expected MCP structure
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{
					"listChanged": true,
				},
			},
			"serverInfo": map[string]string{
				"name":    "mcp-sse-go",
				"version": "0.1.0",
			},
			"tools": toolDefs,
		},
	}

	h.logger.Info().
		Interface("response", response).
		Msg("Sending initialize response")

	// For initialize, always send as direct JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to write initialize response")
		return
	}

	h.logger.Debug().
		Interface("response", response).
		Msg("Sent initialize response")

	h.logger.Info().
		Int("tool_count", len(toolDefs)).
		Msg("Successfully sent initialize response with tools")
}

// handleToolsList handles the tools/list request according to MCP specification
func (h *Handler) handleToolsList(w http.ResponseWriter, flusher http.Flusher, req *jsonrpc.Request) {
	h.logger.Info().
		Str("method", req.Method).
		Interface("id", req.ID).
		Msg("Handling tools/list request")

	// List all registered tools
	toolList := h.toolRegistry.List()
	h.logger.Info().
		Int("tool_count", len(toolList)).
		Msg("Found registered tools")

	toolDefs := make([]map[string]interface{}, 0, len(toolList))
	for _, tool := range toolList {
		toolName := tool.Name()
		h.logger.Debug().
			Str("tool_name", toolName).
			Msg("Including tool in list")

		// Create a tool definition according to MCP specification
		toolDef := map[string]interface{}{
			"name":        toolName,
			"title":       fmt.Sprintf("%s Tool", toolName),
			"description": fmt.Sprintf("A tool named %s", toolName),
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type":        "string",
						"description": "Input for the tool",
					},
				},
				"required": []string{"input"},
			},
		}
		toolDefs = append(toolDefs, toolDef)

		h.logger.Debug().
			Str("tool_name", toolName).
			Interface("definition", toolDef).
			Msg("Tool definition")
	}

	// Create the response
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]interface{}{
			"tools": toolDefs,
		},
	}

	h.logger.Debug().
		Interface("response", response).
		Msg("Sending tools/list response")

	// Send response as direct JSON
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to write tools/list response")
		return
	}

	h.logger.Info().
		Int("tool_count", len(toolDefs)).
		Msg("Successfully sent tools list")
}

// handleRequest handles a single JSON-RPC request.
func (h *Handler) handleRequest(w http.ResponseWriter, flusher http.Flusher, req *jsonrpc.Request, ctx context.Context) {
	h.logger.Info().
		Str("method", req.Method).
		Interface("id", req.ID).
		Msg("Handling JSON-RPC request")

	// Handle different methods
	switch req.Method {
	case "initialize":
		h.handleInitialize(w, flusher, req)
	case "tools/execute":
		h.handleToolExecution(w, flusher, req, ctx)
	default:
		h.sendError(w, flusher, jsonrpc.NewError(
			jsonrpc.MethodNotFound,
			fmt.Sprintf("Method not found: %s", req.Method),
			nil,
		))
	}
}



// handleToolExecution handles tool execution requests.
func (h *Handler) handleToolExecution(w http.ResponseWriter, flusher http.Flusher, req *jsonrpc.Request, ctx context.Context) {
	// Parse tool execution parameters
	var params struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		h.sendError(w, flusher, jsonrpc.NewError(
			jsonrpc.InvalidParams,
			"Invalid parameters",
			err.Error(),
		))
		return
	}

	// Get the HTTP request from the context
	httpReq, ok := GetRequestFromContext(ctx)
	if !ok {
		h.sendError(w, flusher, jsonrpc.NewError(
			jsonrpc.InternalError,
			"Failed to get HTTP request from context",
			nil,
		))
		return
	}

	// Get API key and URL from headers
	apiURL := httpReq.Header.Get("X-Weather-API-URL")
	apiKey := httpReq.Header.Get("X-Weather-API-Key")

	// Add API key and URL to the context
	if apiURL != "" {
		ctx = context.WithValue(ctx, weather.ContextKeyAPIURL, apiURL)
	}
	if apiKey != "" {
		ctx = context.WithValue(ctx, weather.ContextKeyAPIKey, apiKey)
	}

	// Execute the tool with the context
	result, err := h.toolRegistry.Call(ctx, params.Name, params.Args)
	if err != nil {
		h.sendError(w, flusher, jsonrpc.NewError(
			jsonrpc.InternalError,
			fmt.Sprintf("Failed to execute tool '%s'", params.Name),
			err.Error(),
		))
		return
	}

	// Send success response
	h.sendResponse(w, flusher, req.ID, result)
}

// handleNotification processes JSON-RPC notifications.
func (h *Handler) handleNotification(notif *jsonrpc.Notification) {
	h.logger.Info().
		Str("method", notif.Method).
		Msg("Received notification")

	// Handle different notification types
	switch notif.Method {
	// Add notification handlers here
	}
}

// sendResponse sends a JSON-RPC response.
func (h *Handler) sendResponse(w http.ResponseWriter, flusher http.Flusher, id interface{}, result interface{}) {
	resp := &jsonrpc.Response{
		JSONRPC: jsonrpc.Version,
		ID:      id,
		Result:  result,
	}
	if err := h.sendJSON(w, flusher, resp); err != nil {
		h.logger.Error().Err(err).Msg("Failed to send response")
	}
}

// sendError sends a JSON-RPC error response.
func (h *Handler) sendError(w http.ResponseWriter, flusher http.Flusher, err *jsonrpc.Error) {
	resp := &jsonrpc.Response{
		JSONRPC: jsonrpc.Version,
		Error:   err,
	}
	if sendErr := h.sendJSONResponse(w, flusher, resp, "JSON-RPC error response"); sendErr != nil {
		h.logger.Error().Err(sendErr).Msg("Failed to send error response")
	}
}

// sendJSON sends a JSON response as an SSE message
func (h *Handler) sendJSON(w http.ResponseWriter, flusher http.Flusher, v interface{}) error {
	return h.sendJSONResponse(w, flusher, v, "SSE message")
}

// sendJSONResponse sends a JSON-RPC response, handling both direct HTTP and SSE responses
func (h *Handler) sendJSONResponse(w http.ResponseWriter, flusher http.Flusher, response interface{}, responseType string) error {
	jsonData, err := json.Marshal(response)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to marshal JSON response")
		return err
	}

	h.logger.Debug().
		Str("response", string(jsonData)).
		Msg(fmt.Sprintf("Sending %s", responseType))

	if flusher != nil {
		// For SSE, send as an event
		_, err = fmt.Fprintf(w, "data: %s\n\n", jsonData)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to write SSE message")
			return err
		}
		flusher.Flush()
	} else {
		// For direct HTTP, send as JSON
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(jsonData)
		if err != nil {
			h.logger.Error().Err(err).Msg("Failed to write HTTP response")
			return err
		}
	}

	return nil
}