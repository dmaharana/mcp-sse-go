package session

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestDefaultSessionManager_CreateSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create session
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.ID == "" {
		t.Fatal("Session ID should not be empty")
	}

	if session.ClientInfo.RemoteAddr != clientInfo.RemoteAddr {
		t.Errorf("Expected remote addr %s, got %s", clientInfo.RemoteAddr, session.ClientInfo.RemoteAddr)
	}

	if session.ClientInfo.UserAgent != clientInfo.UserAgent {
		t.Errorf("Expected user agent %s, got %s", clientInfo.UserAgent, session.ClientInfo.UserAgent)
	}

	// Verify session is stored
	storedSession, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Session should be stored: %v", err)
	}

	if storedSession.ID != session.ID {
		t.Errorf("Stored session ID mismatch: expected %s, got %s", session.ID, storedSession.ID)
	}
}

func TestDefaultSessionManager_ValidateSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create session
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Validate session
	validatedSession, err := manager.ValidateSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to validate session: %v", err)
	}

	if validatedSession.ID != session.ID {
		t.Errorf("Validated session ID mismatch: expected %s, got %s", session.ID, validatedSession.ID)
	}
}

func TestDefaultSessionManager_ValidateSession_NotFound(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()

	// Create a valid session ID format but non-existent session
	generator := NewSessionIDGenerator()
	validButNonExistentID, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate valid session ID: %v", err)
	}

	// Try to validate non-existent session
	_, err = manager.ValidateSession(ctx, validButNonExistentID)
	if err == nil {
		t.Fatal("Expected error when validating non-existent session")
	}

	sessionErr, ok := err.(*SessionError)
	if !ok {
		t.Fatalf("Expected SessionError, got %T", err)
	}

	if sessionErr.Code != ErrSessionNotFound {
		t.Errorf("Expected error code %s, got %s", ErrSessionNotFound, sessionErr.Code)
	}
}

func TestDefaultSessionManager_ValidateSession_InvalidFormat(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()

	// Try to validate invalid session ID
	_, err := manager.ValidateSession(ctx, "invalid-session-id")
	if err == nil {
		t.Fatal("Expected error when validating invalid session ID")
	}

	sessionErr, ok := err.(*SessionError)
	if !ok {
		t.Fatalf("Expected SessionError, got %T", err)
	}

	if sessionErr.Code != ErrSessionInvalid {
		t.Errorf("Expected error code %s, got %s", ErrSessionInvalid, sessionErr.Code)
	}
}

func TestDefaultSessionManager_ValidateSession_Expired(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Millisecond, // Very short timeout
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create session
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Wait for session to expire
	time.Sleep(10 * time.Millisecond)

	// Try to validate expired session
	_, err = manager.ValidateSession(ctx, session.ID)
	if err == nil {
		t.Fatal("Expected error when validating expired session")
	}

	sessionErr, ok := err.(*SessionError)
	if !ok {
		t.Fatalf("Expected SessionError, got %T", err)
	}

	if sessionErr.Code != ErrSessionExpired {
		t.Errorf("Expected error code %s, got %s", ErrSessionExpired, sessionErr.Code)
	}

	// Verify expired session was cleaned up
	_, err = store.Get(ctx, session.ID)
	if err == nil {
		t.Error("Expired session should have been cleaned up")
	}
}

func TestDefaultSessionManager_RefreshSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create session
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	originalExpiresAt := session.ExpiresAt
	originalLastAccess := session.LastAccess

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Refresh session
	err = manager.RefreshSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to refresh session: %v", err)
	}

	// Get updated session
	updatedSession, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get updated session: %v", err)
	}

	// Verify expiration time was updated
	if !updatedSession.ExpiresAt.After(originalExpiresAt) {
		t.Error("Session expiration time should have been updated")
	}

	// Verify last access time was updated
	if !updatedSession.LastAccess.After(originalLastAccess) {
		t.Error("Session last access time should have been updated")
	}
}

func TestDefaultSessionManager_DeleteSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create session
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session exists
	_, err = manager.ValidateSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Session should exist: %v", err)
	}

	// Delete session
	err = manager.DeleteSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify session no longer exists
	_, err = manager.ValidateSession(ctx, session.ID)
	if err == nil {
		t.Fatal("Session should not exist after deletion")
	}
}

func TestDefaultSessionManager_CleanupExpiredSessions(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Millisecond, // Very short timeout
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create multiple sessions
	sessionIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		session, err := manager.CreateSession(ctx, clientInfo)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
		sessionIDs[i] = session.ID
	}

	// Wait for sessions to expire
	time.Sleep(10 * time.Millisecond)

	// Create one more session that won't be expired
	config.SessionTimeout = time.Hour
	manager = NewDefaultSessionManager(store, config, logger)
	activeSession, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create active session: %v", err)
	}

	// Run cleanup
	deletedCount, err := manager.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("Failed to cleanup expired sessions: %v", err)
	}

	if deletedCount != 3 {
		t.Errorf("Expected 3 deleted sessions, got %d", deletedCount)
	}

	// Verify expired sessions are gone
	for i, sessionID := range sessionIDs {
		_, err := store.Get(ctx, sessionID)
		if err == nil {
			t.Errorf("Expired session %d should have been deleted", i)
		}
	}

	// Verify active session still exists
	_, err = store.Get(ctx, activeSession.ID)
	if err != nil {
		t.Errorf("Active session should still exist: %v", err)
	}
}

func TestDefaultSessionManager_GetActiveSessionCount(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Initially no sessions
	count, err := manager.GetActiveSessionCount(ctx)
	if err != nil {
		t.Fatalf("Failed to get session count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 sessions, got %d", count)
	}

	// Create sessions
	for i := 0; i < 5; i++ {
		_, err := manager.CreateSession(ctx, clientInfo)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Check count
	count, err = manager.GetActiveSessionCount(ctx)
	if err != nil {
		t.Fatalf("Failed to get session count: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 sessions, got %d", count)
	}
}

func TestDefaultSessionManager_GetSessionStats(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	config := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, config, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create some sessions
	for i := 0; i < 3; i++ {
		_, err := manager.CreateSession(ctx, clientInfo)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Get stats
	stats, err := manager.GetSessionStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get session stats: %v", err)
	}

	if stats["total_sessions"] != 3 {
		t.Errorf("Expected 3 total sessions, got %v", stats["total_sessions"])
	}

	if stats["active_sessions"] != 3 {
		t.Errorf("Expected 3 active sessions, got %v", stats["active_sessions"])
	}

	if stats["expired_sessions"] != 0 {
		t.Errorf("Expected 0 expired sessions, got %v", stats["expired_sessions"])
	}

	if stats["session_timeout"] != config.SessionTimeout.String() {
		t.Errorf("Expected timeout %s, got %v", config.SessionTimeout.String(), stats["session_timeout"])
	}

	// Should include store stats
	if stats["store_total_sessions"] != 3 {
		t.Errorf("Expected store stats to be included")
	}
}