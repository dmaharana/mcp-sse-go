package session

import (
	"context"
	"time"
)

// Session represents an active client session
type Session struct {
	ID         string     `json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	LastAccess time.Time  `json:"last_access"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ClientInfo ClientInfo `json:"client_info"`
}

// ClientInfo contains information about the client
type ClientInfo struct {
	RemoteAddr string `json:"remote_addr"`
	UserAgent  string `json:"user_agent"`
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// Refresh updates the last access time and extends expiration
func (s *Session) Refresh(timeout time.Duration) {
	now := time.Now()
	s.LastAccess = now
	s.ExpiresAt = now.Add(timeout)
}

// SessionManager defines the interface for session management operations
type SessionManager interface {
	// CreateSession generates a new session ID and stores it
	CreateSession(ctx context.Context, clientInfo ClientInfo) (*Session, error)

	// ValidateSession checks if a session ID is valid and active
	ValidateSession(ctx context.Context, sessionID string) (*Session, error)

	// RefreshSession updates the last activity timestamp
	RefreshSession(ctx context.Context, sessionID string) error

	// DeleteSession removes a session from the store
	DeleteSession(ctx context.Context, sessionID string) error

	// CleanupExpiredSessions removes all expired sessions
	CleanupExpiredSessions(ctx context.Context) (int, error)

	// GetActiveSessionCount returns the number of active sessions
	GetActiveSessionCount(ctx context.Context) (int, error)

	// GetSessionStats returns detailed statistics about sessions
	GetSessionStats(ctx context.Context) (map[string]interface{}, error)
}

// SessionStore defines the interface for session storage operations
type SessionStore interface {
	// Set stores a session with expiration
	Set(ctx context.Context, sessionID string, session *Session) error

	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID string) (*Session, error)

	// Delete removes a session
	Delete(ctx context.Context, sessionID string) error

	// List returns all active sessions (for cleanup)
	List(ctx context.Context) ([]*Session, error)

	// Count returns the number of stored sessions
	Count(ctx context.Context) (int, error)

	// Close cleans up resources
	Close() error
}