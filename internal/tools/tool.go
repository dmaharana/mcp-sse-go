package tools

import (
	"context"
	"encoding/json"
)

// Tool is the interface that all tools must implement.
type Tool interface {
	// Name returns the name of the tool.
	Name() string

	// Call executes the tool with the given arguments and context.
	// The arguments and return value are JSON-encoded data.
	// The context can be used to pass request-specific values like API keys.
	Call(ctx context.Context, args json.RawMessage) (json.RawMessage, error)

	// GetToolDefinition returns the tool definition in MCP format.
	// The definition includes the tool's name, description, and input schema.
	GetToolDefinition() map[string]any
}
