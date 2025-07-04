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
		Caller().
		Logger()

	// Configure caller marshaling to show relative paths
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		// Get relative path from the project root
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		return fmt.Sprintf("%s:%d", short, line)
	}

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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Weather-API-URL, X-Weather-API-Key, Accept, Cache-Control")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Handle CORS preflight requests
	if r.Method == http.MethodOptions {
		h.logger.Info().Msg("Handling OPTIONS preflight request")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Weather-API-URL, X-Weather-API-Key, Accept, Cache-Control")
		w.Header().Set("Access-Control-Max-Age", "86400")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type, Authorization, X-Weather-API-URL, X-Weather-API-Key, Accept, Cache-Control")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check if this is an SSE connection
	isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream")

	// Set up response headers for SSE if this is an SSE connection
	if isSSE {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
	}

	// Get flusher for SSE if this is an SSE connection
	var flusher http.Flusher
	if isSSE {
		var ok bool
		flusher, ok = w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}
	}

	// Create context with request
	ctx := WithRequest(r.Context(), r)

	// Handle POST requests (JSON-RPC messages)
	if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		h.logger.Info().
			Bool("isSSE", isSSE).
			Str("content-type", r.Header.Get("Content-Type")).
			Msg("Handling JSON-RPC request")
		
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

		// Use the existing context and flusher

		// Handle the initialization request
		if req.Method == "initialize" {
			h.logger.Info().Msg("Handling initialize request")
			h.handleInitialize(w, flusher, &req, ctx)
			return
		}

		// Handle the tools/list request
		if req.Method == "tools/list" {
			h.logger.Info().Msg("Handling tools/list request")
			h.handleToolsList(w, &req, ctx)
			return
		}

		// Handle other JSON-RPC methods
		h.logger.Info().Str("method", req.Method).Msg("Handling JSON-RPC method")
		h.handleRequest(w, flusher, &req, ctx)
		return
	}

	// Handle GET requests (SSE connection)
	if r.Method == http.MethodGet && isSSE {
		// Handle SSE connection
		h.logger.Info().Msg("Handling SSE connection")

		// Keep the connection open
		for {
			select {
			case <-r.Context().Done():
				h.logger.Info().Msg("SSE connection closed by client")
				return
			case <-time.After(30 * time.Second):
				// Send a keep-alive comment
				_, err := fmt.Fprintf(w, ":keep-alive\n\n")
				if err != nil {
					h.logger.Error().Err(err).Msg("Failed to send keep-alive")
					return
				}
				flusher.Flush()
			}
		}
	}

	h.logger.Warn().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Method not allowed")
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return
}

// handleRequestWithContext processes JSON-RPC requests with the provided context.
func (h *Handler) handleRequestWithContext(w http.ResponseWriter, flusher http.Flusher, req *jsonrpc.Request, ctx context.Context) {
	h.handleRequest(w, flusher, req, ctx)
}



// handleInitialize handles the initialize request according to MCP specification
func (h *Handler) handleInitialize(w http.ResponseWriter, flusher http.Flusher, req *jsonrpc.Request, ctx context.Context) {
    // Get the request from context
    httpReq, _ := GetRequestFromContext(ctx)
    
    // Log detailed information about the initialize request
    h.logger.Info().
        Str("method", req.Method).
        Interface("id", req.ID).
        Str("remote_addr", httpReq.RemoteAddr).
        Str("user_agent", httpReq.UserAgent()).
        Msg("Handling initialize request")
        
    // Log all headers for debugging
    headers := make(map[string]string)
    for k, v := range httpReq.Header {
        headers[k] = strings.Join(v, ", ")
    }
    h.logger.Debug().
        Interface("headers", headers).
        Msg("Initialize request headers")

    // List all registered tools
    toolList := h.toolRegistry.List()
    h.logger.Info().
        Int("tool_count", len(toolList)).
        Msg("Found registered tools")

    tools := make([]map[string]interface{}, 0, len(toolList))
    for _, tool := range toolList {
        toolName := tool.Name()
        h.logger.Debug().
            Str("tool_name", toolName).
            Msg("Including tool in list")

        // Create a tool definition according to MCP specification
        toolDef := map[string]interface{}{
            "name": toolName,
            "annotations": map[string]interface{}{
                "title":       fmt.Sprintf("%s Tool", toolName),
                "openWorldHint": true,  // Indicates the tool interacts with external services
            },
        }

        // Special case for weather tool
        if toolName == "weather" {
            toolDef["description"] = "Get current weather for a city"
            toolDef["inputSchema"] = map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "city": map[string]interface{}{
                        "type":        "string",
                        "description": "The city to get weather for",
                    },
                },
                "required": []string{"city"},
            }
        } else {
            // Default schema for other tools
            toolDef["description"] = fmt.Sprintf("A tool named %s", toolName)
            toolDef["inputSchema"] = map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "input": map[string]interface{}{
                        "type":        "string",
                        "description": "Input for the tool",
                    },
                },
                "required": []string{"input"},
            }
        }
        tools = append(tools, toolDef)
    }

    // Create the response with the expected MCP structure
    response := map[string]any{
        "jsonrpc": "2.0",
        "id":      req.ID,
        "result": map[string]any{
            "protocolVersion": "2025-03-26",
            "capabilities": map[string]any{
                "tools": map[string]any{
                    "listChanged": true,
                },
                "toolUse": map[string]any{
                    "enabled": true,
                },
            },
            "serverInfo": map[string]any{
                "name":    "mcp-sse-go",
                "version": "0.1.0",
            },
            "tools": tools,  // Include tools in the initialization response
        },
    }

    h.logger.Info().
        Interface("response", response).
        Msg("Sending initialize response")

    // Set response headers
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Weather-API-Key, X-Weather-API-URL")
    w.Header().Set("Access-Control-Allow-Credentials", "true")
    w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")  // Disable buffering for Nginx
    
    // Set status code before writing the body
    w.WriteHeader(http.StatusOK)
    
    // Check if this is an OPTIONS preflight request
    if httpReq != nil && httpReq.Method == "OPTIONS" {
        h.logger.Info().Msg("Skipping response body for OPTIONS request")
        return
    }
    
    // Encode and send the response
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")  // Pretty print for debugging
    if err := enc.Encode(response); err != nil {
        h.logger.Error().Err(err).Msg("Failed to write initialize response")
        return
    }
    
    // Flush the response if we have a flusher
    if flusher != nil {
        flusher.Flush()
    }

    h.logger.Info().
        Int("tool_count", len(tools)).
        Interface("tools", tools).
        Msg("Successfully sent initialize response with tools")
}

// handleToolsList handles the tools/list request according to MCP specification
func (h *Handler) handleToolsList(w http.ResponseWriter, req *jsonrpc.Request, ctx context.Context) {
    h.logger.Info().
        Str("method", req.Method).
        Interface("id", req.ID).
        Msg("Handling tools/list request")

    // List all registered tools
    toolList := h.toolRegistry.List()
    h.logger.Info().
        Int("tool_count", len(toolList)).
        Msg("Found registered tools")

    tools := make([]map[string]any, 0, len(toolList))
    for _, tool := range toolList {
        toolName := tool.Name()
        h.logger.Debug().
            Str("tool_name", toolName).
            Msg("Including tool in list")

        // Create a tool definition according to MCP specification
        toolDef := map[string]any{
            "name": toolName,
            "annotations": map[string]any{
                "title":       fmt.Sprintf("%s Tool", toolName),
                "openWorldHint": true,  // Indicates the tool interacts with external services
            },
        }

        // Special case for weather tool
        if toolName == "weather" {
            toolDef["description"] = "Get current weather for a city"
            toolDef["inputSchema"] = map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "city": map[string]any{
                        "type":        "string",
                        "description": "The city to get weather for",
                    },
                },
                "required": []string{"city"},
            }
        } else {
            // Default schema for other tools
            toolDef["description"] = fmt.Sprintf("A tool named %s", toolName)
            toolDef["inputSchema"] = map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "input": map[string]any{
                        "type":        "string",
                        "description": "Input for the tool",
                    },
                },
                "required": []string{"input"},
            }
        }
        tools = append(tools, toolDef)
    }

    // Create the response according to MCP specification
    response := map[string]any{
        "jsonrpc": "2.0",
        "id":      req.ID,
        "result": map[string]any{
            "tools": tools,
        },
    }

    h.logger.Debug().
        Interface("response", response).
        Msg("Sending tools/list response")

    // Send the response as raw JSON
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.WriteHeader(http.StatusOK)
    
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")  // Pretty print for debugging
    if err := enc.Encode(response); err != nil {
        h.logger.Error().Err(err).Msg("Failed to write tools/list response")
        return
    }

    h.logger.Info().
        Int("tool_count", len(tools)).
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
		h.handleInitialize(w, flusher, req, ctx)
	case "tools/execute":
	case "tools/call":
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
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
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

	h.logger.Info().
		Str("tool_name", params.Name).
		Interface("arguments", params.Arguments).
		Interface("api_url", apiURL).
		Interface("api_key", apiKey).
		Msg("Executing tool")


	// Execute the tool with the context
	result, err := h.toolRegistry.Call(ctx, params.Name, params.Arguments)
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