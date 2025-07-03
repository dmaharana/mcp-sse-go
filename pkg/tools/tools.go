package tools

import "encoding/json"

// Tool is the interface that all tools must implement.
// The argument is a json.RawMessage, which allows for flexible parameter passing.
// The return value is a json.RawMessage, which allows for flexible return values.
type Tool interface {
	Name() string
	Call(args json.RawMessage) (json.RawMessage, error)
}
