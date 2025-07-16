package session

import "fmt"

// SessionError represents a session-related error
type SessionError struct {
	Code    string
	Message string
	Cause   error
}

// Error implements the error interface
func (e *SessionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *SessionError) Unwrap() error {
	return e.Cause
}

// Error codes for session operations
const (
	ErrSessionNotFound   = "SESSION_NOT_FOUND"
	ErrSessionExpired    = "SESSION_EXPIRED"
	ErrSessionInvalid    = "SESSION_INVALID"
	ErrSessionGeneration = "SESSION_GENERATION_FAILED"
	ErrSessionStorage    = "SESSION_STORAGE_ERROR"
	ErrSessionConcurrency = "SESSION_CONCURRENCY_ERROR"
)

// NewSessionError creates a new session error
func NewSessionError(code, message string, cause error) *SessionError {
	return &SessionError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewSessionNotFoundError creates a session not found error
func NewSessionNotFoundError(sessionID string) *SessionError {
	return &SessionError{
		Code:    ErrSessionNotFound,
		Message: fmt.Sprintf("session not found: %s", sessionID),
	}
}

// NewSessionExpiredError creates a session expired error
func NewSessionExpiredError(sessionID string) *SessionError {
	return &SessionError{
		Code:    ErrSessionExpired,
		Message: fmt.Sprintf("session expired: %s", sessionID),
	}
}

// NewSessionInvalidError creates a session invalid error
func NewSessionInvalidError(sessionID string) *SessionError {
	return &SessionError{
		Code:    ErrSessionInvalid,
		Message: fmt.Sprintf("session invalid: %s", sessionID),
	}
}

// NewSessionGenerationError creates a session generation error
func NewSessionGenerationError(cause error) *SessionError {
	return &SessionError{
		Code:    ErrSessionGeneration,
		Message: "failed to generate session ID",
		Cause:   cause,
	}
}

// NewSessionStorageError creates a session storage error
func NewSessionStorageError(operation string, cause error) *SessionError {
	return &SessionError{
		Code:    ErrSessionStorage,
		Message: fmt.Sprintf("session storage error during %s", operation),
		Cause:   cause,
	}
}