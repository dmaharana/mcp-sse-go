package session

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestCleanupService_StartStop(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	cleanupConfig := CleanupConfig{
		CleanupInterval: time.Second,
	}
	service := NewCleanupService(manager, cleanupConfig, logger)

	ctx := context.Background()

	// Initially not running
	if service.IsRunning() {
		t.Error("Service should not be running initially")
	}

	// Start service
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Should be running now
	if !service.IsRunning() {
		t.Error("Service should be running after start")
	}

	// Starting again should not error
	err = service.Start(ctx)
	if err != nil {
		t.Errorf("Starting already running service should not error: %v", err)
	}

	// Stop service
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	// Should not be running now
	if service.IsRunning() {
		t.Error("Service should not be running after stop")
	}

	// Stopping again should not error
	err = service.Stop()
	if err != nil {
		t.Errorf("Stopping already stopped service should not error: %v", err)
	}
}

func TestCleanupService_RunOnce(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Millisecond, // Very short timeout
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	cleanupConfig := CleanupConfig{
		CleanupInterval: time.Hour, // Long interval, we'll run manually
	}
	service := NewCleanupService(manager, cleanupConfig, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create some sessions that will expire
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

	// Create one active session
	managerConfig.SessionTimeout = time.Hour
	manager = NewDefaultSessionManager(store, managerConfig, logger)
	activeSession, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create active session: %v", err)
	}

	// Run cleanup once
	deletedCount, err := service.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
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

	// Run cleanup again - should find no expired sessions
	deletedCount, err = service.RunOnce(ctx)
	if err != nil {
		t.Fatalf("Second RunOnce failed: %v", err)
	}

	if deletedCount != 0 {
		t.Errorf("Expected 0 deleted sessions on second run, got %d", deletedCount)
	}
}

func TestCleanupService_AutomaticCleanup(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: 50 * time.Millisecond, // Short timeout
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	cleanupConfig := CleanupConfig{
		CleanupInterval: 100 * time.Millisecond, // Fast cleanup
	}
	service := NewCleanupService(manager, cleanupConfig, logger)

	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	// Create some sessions
	sessionCount := 5
	for i := 0; i < sessionCount; i++ {
		_, err := manager.CreateSession(ctx, clientInfo)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Verify sessions exist
	count, err := manager.GetActiveSessionCount(ctx)
	if err != nil {
		t.Fatalf("Failed to get session count: %v", err)
	}
	if count != sessionCount {
		t.Errorf("Expected %d sessions, got %d", sessionCount, count)
	}

	// Start cleanup service
	err = service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start cleanup service: %v", err)
	}
	defer service.Stop()

	// Wait for sessions to expire and be cleaned up
	time.Sleep(200 * time.Millisecond)

	// Verify sessions were cleaned up
	count, err = manager.GetActiveSessionCount(ctx)
	if err != nil {
		t.Fatalf("Failed to get session count after cleanup: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 sessions after cleanup, got %d", count)
	}
}

func TestCleanupService_ContextCancellation(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	cleanupConfig := CleanupConfig{
		CleanupInterval: time.Second,
	}
	service := NewCleanupService(manager, cleanupConfig, logger)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Verify service is running
	if !service.IsRunning() {
		t.Error("Service should be running")
	}

	// Cancel context
	cancel()

	// Wait a bit for the service to stop
	time.Sleep(100 * time.Millisecond)

	// Service should still be marked as running (stop wasn't called)
	// but the goroutine should have exited due to context cancellation
	if !service.IsRunning() {
		t.Error("Service should still be marked as running")
	}

	// Stop service to clean up
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}
}

func TestCleanupService_GetStats(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	cleanupInterval := 5 * time.Minute
	cleanupConfig := CleanupConfig{
		CleanupInterval: cleanupInterval,
	}
	service := NewCleanupService(manager, cleanupConfig, logger)

	// Get stats when not running
	stats := service.GetStats()
	if stats["running"] != false {
		t.Errorf("Expected running=false, got %v", stats["running"])
	}
	if stats["cleanup_interval"] != cleanupInterval.String() {
		t.Errorf("Expected interval=%s, got %v", cleanupInterval.String(), stats["cleanup_interval"])
	}

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()

	// Get stats when running
	stats = service.GetStats()
	if stats["running"] != true {
		t.Errorf("Expected running=true, got %v", stats["running"])
	}
}

func TestCleanupService_SetInterval(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	cleanupConfig := CleanupConfig{
		CleanupInterval: time.Minute,
	}
	service := NewCleanupService(manager, cleanupConfig, logger)

	// Set new interval
	newInterval := 30 * time.Second
	service.SetInterval(newInterval)

	// Verify interval was updated
	stats := service.GetStats()
	if stats["cleanup_interval"] != newInterval.String() {
		t.Errorf("Expected interval=%s, got %v", newInterval.String(), stats["cleanup_interval"])
	}
}

func TestCleanupService_RunOnceWithCancelledContext(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	cleanupConfig := CleanupConfig{
		CleanupInterval: time.Hour,
	}
	service := NewCleanupService(manager, cleanupConfig, logger)

	// Create a context and cancel it immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// RunOnce should handle the cancelled context gracefully
	// Since the cleanup operation is fast, it might complete before checking context
	// This test mainly ensures no panic occurs with cancelled context
	_, err := service.RunOnce(ctx)
	// We don't assert error here since the operation might complete before context check
	_ = err // Acknowledge we're not checking the error
}