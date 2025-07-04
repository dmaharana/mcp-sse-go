package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// DefaultTool is a base implementation of the Tool interface that can be embedded in other tools.
type DefaultTool struct {
	name        string
	description string
}

// NewDefaultTool creates a new DefaultTool with the given name and description.
func NewDefaultTool(name, description string) *DefaultTool {
	return &DefaultTool{
		name:        name,
		description: description,
	}
}

// Name returns the name of the tool.
func (t *DefaultTool) Name() string {
	return t.name
}

// Call is the default implementation of the Tool interface.
// Tools should override this method with their specific implementation.
func (t *DefaultTool) Call(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("method not implemented for tool: %s", t.name)
}

// GetToolDefinition returns the default tool definition in MCP format.
func (t *DefaultTool) GetToolDefinition() map[string]any {
	return map[string]any{
		"name":        t.name,
		"description": t.description,
		"annotations": map[string]any{
			"title":         fmt.Sprintf("%s Tool", t.name),
			"openWorldHint": true,
		},
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{
					"type":        "string",
					"description": "Input for the tool",
				},
			},
			"required": []string{"input"},
		},
	}
}
