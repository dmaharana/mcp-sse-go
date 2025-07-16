package session

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// DefaultSessionManager implements SessionManager interface
type DefaultSessionManager struct {
	store     SessionStore
	generator *SessionIDGenerator
	timeout   time.Duration
	logger    zerolog.Logger
}

// ManagerConfig contains configuration for the session manager
type ManagerConfig struct {
	SessionTimeout time.Duration
}

// NewDefaultSessionManager creates a new session manager
func NewDefaultSessionManager(store SessionStore, config ManagerConfig, logger zerolog.Logger) *DefaultSessionManager {
	return &DefaultSessionManager{
		store:     store,
		generator: NewSessionIDGenerator(),
		timeout:   config.SessionTimeout,
		logger:    logger.With().Str("component", "session_manager").Logger(),
	}
}

// CreateSession generates a new session ID and stores it
func (m *DefaultSessionManager) CreateSession(ctx context.Context, clientInfo ClientInfo) (*Session, error) {
	// Generate session ID
	sessionID, err := m.generator.Generate()
	if err != nil {
		m.logger.Error().
			Err(err).
			Str("remote_addr", clientInfo.RemoteAddr).
			Str("user_agent", clientInfo.UserAgent).
			Msg("Failed to generate session ID")
		return nil, err
	}

	// Create session
	now := time.Now()
	session := &Session{
		ID:         sessionID,
		CreatedAt:  now,
		LastAccess: now,
		ExpiresAt:  now.Add(m.timeout),
		ClientInfo: clientInfo,
	}

	// Store session
	if err := m.store.Set(ctx, sessionID, session); err != nil {
		m.logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Str("remote_addr", clientInfo.RemoteAddr).
			Msg("Failed to store session")
		return nil, NewSessionStorageError("create", err)
	}

	m.logger.Info().
		Str("session_id", sessionID).
		Str("remote_addr", clientInfo.RemoteAddr).
		Str("user_agent", clientInfo.UserAgent).
		Time("expires_at", session.ExpiresAt).
		Msg("Session created successfully")

	return session, nil
}

// ValidateSession checks if a session ID is valid and active
func (m *DefaultSessionManager) ValidateSession(ctx context.Context, sessionID string) (*Session, error) {
	// Validate session ID format
	if err := m.generator.Validate(sessionID); err != nil {
		m.logger.Debug().
			Str("session_id", sessionID).
			Err(err).
			Msg("Session ID format validation failed")
		return nil, err
	}

	// Retrieve session from store
	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		m.logger.Debug().
			Str("session_id", sessionID).
			Err(err).
			Msg("Session not found in store")
		return nil, err
	}

	// Check if session is expired
	if session.IsExpired() {
		m.logger.Debug().
			Str("session_id", sessionID).
			Time("expires_at", session.ExpiresAt).
			Msg("Session has expired")
		
		// Clean up expired session
		if deleteErr := m.store.Delete(ctx, sessionID); deleteErr != nil {
			m.logger.Warn().
				Err(deleteErr).
				Str("session_id", sessionID).
				Msg("Failed to delete expired session")
		}
		
		return nil, NewSessionExpiredError(sessionID)
	}

	m.logger.Debug().
		Str("session_id", sessionID).
		Time("last_access", session.LastAccess).
		Time("expires_at", session.ExpiresAt).
		Msg("Session validation successful")

	return session, nil
}

// RefreshSession updates the last activity timestamp
func (m *DefaultSessionManager) RefreshSession(ctx context.Context, sessionID string) error {
	// Get current session
	session, err := m.ValidateSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Update timestamps
	session.Refresh(m.timeout)

	// Store updated session
	if err := m.store.Set(ctx, sessionID, session); err != nil {
		m.logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Msg("Failed to refresh session")
		return NewSessionStorageError("refresh", err)
	}

	m.logger.Debug().
		Str("session_id", sessionID).
		Time("new_expires_at", session.ExpiresAt).
		Msg("Session refreshed successfully")

	return nil
}

// DeleteSession removes a session from the store
func (m *DefaultSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	if err := m.store.Delete(ctx, sessionID); err != nil {
		m.logger.Debug().
			Err(err).
			Str("session_id", sessionID).
			Msg("Failed to delete session")
		return err
	}

	m.logger.Info().
		Str("session_id", sessionID).
		Msg("Session deleted successfully")

	return nil
}

// CleanupExpiredSessions removes all expired sessions
func (m *DefaultSessionManager) CleanupExpiredSessions(ctx context.Context) (int, error) {
	// Get all sessions
	sessions, err := m.store.List(ctx)
	if err != nil {
		m.logger.Error().
			Err(err).
			Msg("Failed to list sessions for cleanup")
		return 0, NewSessionStorageError("cleanup_list", err)
	}

	// Find expired sessions
	var expiredSessions []string
	now := time.Now()
	
	for _, session := range sessions {
		if now.After(session.ExpiresAt) {
			expiredSessions = append(expiredSessions, session.ID)
		}
	}

	// Delete expired sessions
	deletedCount := 0
	for _, sessionID := range expiredSessions {
		if err := m.store.Delete(ctx, sessionID); err != nil {
			m.logger.Warn().
				Err(err).
				Str("session_id", sessionID).
				Msg("Failed to delete expired session during cleanup")
			continue
		}
		deletedCount++
	}

	if deletedCount > 0 {
		m.logger.Info().
			Int("deleted_count", deletedCount).
			Int("total_expired", len(expiredSessions)).
			Msg("Cleanup completed")
	} else {
		m.logger.Debug().
			Int("total_sessions", len(sessions)).
			Msg("No expired sessions found during cleanup")
	}

	return deletedCount, nil
}

// GetActiveSessionCount returns the number of active sessions
func (m *DefaultSessionManager) GetActiveSessionCount(ctx context.Context) (int, error) {
	count, err := m.store.Count(ctx)
	if err != nil {
		m.logger.Error().
			Err(err).
			Msg("Failed to get active session count")
		return 0, NewSessionStorageError("count", err)
	}

	return count, nil
}

// GetSessionStats returns statistics about sessions
func (m *DefaultSessionManager) GetSessionStats(ctx context.Context) (map[string]interface{}, error) {
	// Get all sessions for detailed stats
	sessions, err := m.store.List(ctx)
	if err != nil {
		m.logger.Error().
			Err(err).
			Msg("Failed to get sessions for stats")
		return nil, NewSessionStorageError("stats", err)
	}

	now := time.Now()
	activeCount := 0
	expiredCount := 0
	
	for _, session := range sessions {
		if now.After(session.ExpiresAt) {
			expiredCount++
		} else {
			activeCount++
		}
	}

	stats := map[string]interface{}{
		"total_sessions":   len(sessions),
		"active_sessions":  activeCount,
		"expired_sessions": expiredCount,
		"session_timeout":  m.timeout.String(),
	}

	// Add store-specific stats if available
	if storeStats, ok := m.store.(interface{ GetStats() map[string]interface{} }); ok {
		for k, v := range storeStats.GetStats() {
			stats["store_"+k] = v
		}
	}

	return stats, nil
}