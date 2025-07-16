package telemetry

import (
	"context"
	"time"

	"mcp-sse-go/internal/session"
)

// SessionManagerWrapper wraps a session manager to add telemetry
type SessionManagerWrapper struct {
	session.SessionManager
	metrics *Metrics
}

// NewSessionManagerWrapper creates a new telemetry-aware session manager wrapper
func NewSessionManagerWrapper(manager session.SessionManager, metrics *Metrics) *SessionManagerWrapper {
	return &SessionManagerWrapper{
		SessionManager: manager,
		metrics:        metrics,
	}
}

// CreateSession wraps the original CreateSession to add telemetry
func (w *SessionManagerWrapper) CreateSession(ctx context.Context, clientInfo session.ClientInfo) (*session.Session, error) {
	sess, err := w.SessionManager.CreateSession(ctx, clientInfo)
	if err == nil {
		w.metrics.RecordSessionCreated()
	}
	return sess, err
}

// DeleteSession wraps the original DeleteSession to add telemetry
func (w *SessionManagerWrapper) DeleteSession(ctx context.Context, sessionID string) error {
	// Get session to calculate duration
	sess, getErr := w.SessionManager.ValidateSession(ctx, sessionID)
	
	err := w.SessionManager.DeleteSession(ctx, sessionID)
	if err == nil && getErr == nil {
		duration := time.Since(sess.CreatedAt)
		w.metrics.RecordSessionDeleted(duration)
	}
	return err
}

// CleanupServiceWrapper wraps a cleanup service to add telemetry
type CleanupServiceWrapper struct {
	session.CleanupService
	metrics *Metrics
}

// NewCleanupServiceWrapper creates a new telemetry-aware cleanup service wrapper
func NewCleanupServiceWrapper(service session.CleanupService, metrics *Metrics) *CleanupServiceWrapper {
	return &CleanupServiceWrapper{
		CleanupService: service,
		metrics:        metrics,
	}
}

// ExpireSession records telemetry when a session expires
func (w *CleanupServiceWrapper) ExpireSession(sessionID string, duration time.Duration) {
	w.metrics.RecordSessionExpired(duration)
}