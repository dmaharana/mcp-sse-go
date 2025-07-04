package tools

import (
	"context"
	"encoding/json"
	"sync"
)

// Registry manages the collection of available tools.
type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a new tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Name()] = tool
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools.
func (r *Registry) List() map[string]Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make(map[string]Tool, len(r.tools))
	for name, tool := range r.tools {
		tools[name] = tool
	}
	return tools
}

// Call executes a tool with the given arguments and context.
func (r *Registry) Call(ctx context.Context, toolName string, args json.RawMessage) (json.RawMessage, error) {
	tool, exists := r.Get(toolName)
	if !exists {
		return nil, &Error{Code: "tool_not_found", Message: "Tool not found"}
	}

	return tool.Call(ctx, args)
}

// Error represents a tool execution error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Message
}
