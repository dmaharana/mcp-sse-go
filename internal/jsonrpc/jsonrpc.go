package jsonrpc

import (
	"encoding/json"
	"fmt"
)

const Version = "2.0"

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any     `json:"id,omitempty"`
	Result  any     `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type ErrorCode int

const (
	ParseError     ErrorCode = -32700
	InvalidRequest ErrorCode = -32600
	MethodNotFound ErrorCode = -32601
	InvalidParams  ErrorCode = -32602
	InternalError  ErrorCode = -32603
)

type Error struct {
	Code    ErrorCode   `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

func NewError(code ErrorCode, message string, data interface{}) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func ParseMessage(data []byte) (interface{}, error) {
	var msg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      any     `json:"id,omitempty"`
		Method  string          `json:"method,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
		Error   *Error          `json:"error,omitempty"`
		Result  json.RawMessage `json:"result,omitempty"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, NewError(ParseError, "Parse error", nil)
	}

	if msg.JSONRPC != Version {
		return nil, NewError(InvalidRequest, "Invalid JSON-RPC version", nil)
	}

	// Check if it's a notification
	if msg.ID == nil && msg.Method != "" {
		return &Notification{
			JSONRPC: msg.JSONRPC,
			Method:  msg.Method,
			Params:  msg.Params,
		}, nil
	}

	// Check if it's a request
	if msg.ID != nil && msg.Method != "" {
		return &Request{
			JSONRPC: msg.JSONRPC,
			ID:      msg.ID,
			Method:  msg.Method,
			Params:  msg.Params,
		}, nil
	}

	// Check if it's a response
	if msg.ID != nil && (msg.Result != nil || msg.Error != nil) {
		return &Response{
			JSONRPC: msg.JSONRPC,
			ID:      msg.ID,
			Result:  msg.Result,
			Error:   msg.Error,
		}, nil
	}

	return nil, NewError(InvalidRequest, "Invalid message", nil)
}
