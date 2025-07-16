package session

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
)

// MemoryStore implements SessionStore using in-memory storage
type MemoryStore struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
	logger   zerolog.Logger
}

// NewMemoryStore creates a new in-memory session store
func NewMemoryStore(logger zerolog.Logger) *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*Session),
		logger:   logger.With().Str("component", "memory_store").Logger(),
	}
}

// Set stores a session with expiration
func (s *MemoryStore) Set(ctx context.Context, sessionID string, session *Session) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.logger.Debug().
		Str("session_id", sessionID).
		Time("expires_at", session.ExpiresAt).
		Msg("Storing session")

	s.sessions[sessionID] = session
	return nil
}

// Get retrieves a session by ID
func (s *MemoryStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		s.logger.Debug().
			Str("session_id", sessionID).
			Msg("Session not found")
		return nil, NewSessionNotFoundError(sessionID)
	}

	s.logger.Debug().
		Str("session_id", sessionID).
		Time("expires_at", session.ExpiresAt).
		Bool("expired", session.IsExpired()).
		Msg("Retrieved session")

	return session, nil
}

// Delete removes a session
func (s *MemoryStore) Delete(ctx context.Context, sessionID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, exists := s.sessions[sessionID]
	if !exists {
		s.logger.Debug().
			Str("session_id", sessionID).
			Msg("Session not found for deletion")
		return NewSessionNotFoundError(sessionID)
	}

	delete(s.sessions, sessionID)
	s.logger.Debug().
		Str("session_id", sessionID).
		Msg("Session deleted")

	return nil
}

// List returns all active sessions (for cleanup)
func (s *MemoryStore) List(ctx context.Context) ([]*Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		// Create a copy to avoid race conditions
		sessionCopy := *session
		sessions = append(sessions, &sessionCopy)
	}

	s.logger.Debug().
		Int("count", len(sessions)).
		Msg("Listed all sessions")

	return sessions, nil
}

// Count returns the number of stored sessions
func (s *MemoryStore) Count(ctx context.Context) (int, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	count := len(s.sessions)
	s.logger.Debug().
		Int("count", count).
		Msg("Session count requested")

	return count, nil
}

// Close cleans up resources
func (s *MemoryStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sessionCount := len(s.sessions)
	s.sessions = make(map[string]*Session)

	s.logger.Info().
		Int("cleared_sessions", sessionCount).
		Msg("Memory store closed and cleared")

	return nil
}

// GetStats returns statistics about the store
func (s *MemoryStore) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return map[string]interface{}{
		"total_sessions": len(s.sessions),
		"store_type":     "memory",
	}
}