package session

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestMemoryStore_SetAndGet(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	ctx := context.Background()
	sessionID := "test-session-123"
	
	// Create test session
	session := &Session{
		ID:         sessionID,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
		ClientInfo: ClientInfo{
			RemoteAddr: "127.0.0.1:12345",
			UserAgent:  "test-client/1.0",
		},
	}

	// Test Set
	err := store.Set(ctx, sessionID, session)
	if err != nil {
		t.Fatalf("Failed to set session: %v", err)
	}

	// Test Get
	retrieved, err := store.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, retrieved.ID)
	}

	if retrieved.ClientInfo.RemoteAddr != session.ClientInfo.RemoteAddr {
		t.Errorf("Expected remote addr %s, got %s", session.ClientInfo.RemoteAddr, retrieved.ClientInfo.RemoteAddr)
	}
}

func TestMemoryStore_GetNonExistent(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	ctx := context.Background()
	
	_, err := store.Get(ctx, "non-existent-session")
	if err == nil {
		t.Fatal("Expected error when getting non-existent session")
	}

	sessionErr, ok := err.(*SessionError)
	if !ok {
		t.Fatalf("Expected SessionError, got %T", err)
	}

	if sessionErr.Code != ErrSessionNotFound {
		t.Errorf("Expected error code %s, got %s", ErrSessionNotFound, sessionErr.Code)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	ctx := context.Background()
	sessionID := "test-session-delete"
	
	// Create and store session
	session := &Session{
		ID:         sessionID,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
		ClientInfo: ClientInfo{
			RemoteAddr: "127.0.0.1:12345",
			UserAgent:  "test-client/1.0",
		},
	}

	err := store.Set(ctx, sessionID, session)
	if err != nil {
		t.Fatalf("Failed to set session: %v", err)
	}

	// Verify session exists
	_, err = store.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Session should exist before deletion: %v", err)
	}

	// Delete session
	err = store.Delete(ctx, sessionID)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify session no longer exists
	_, err = store.Get(ctx, sessionID)
	if err == nil {
		t.Fatal("Session should not exist after deletion")
	}
}

func TestMemoryStore_DeleteNonExistent(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	ctx := context.Background()
	
	err := store.Delete(ctx, "non-existent-session")
	if err == nil {
		t.Fatal("Expected error when deleting non-existent session")
	}

	sessionErr, ok := err.(*SessionError)
	if !ok {
		t.Fatalf("Expected SessionError, got %T", err)
	}

	if sessionErr.Code != ErrSessionNotFound {
		t.Errorf("Expected error code %s, got %s", ErrSessionNotFound, sessionErr.Code)
	}
}

func TestMemoryStore_List(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	ctx := context.Background()
	
	// Initially empty
	sessions, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(sessions))
	}

	// Add some sessions
	sessionIDs := []string{"session-1", "session-2", "session-3"}
	for _, id := range sessionIDs {
		session := &Session{
			ID:         id,
			CreatedAt:  time.Now(),
			LastAccess: time.Now(),
			ExpiresAt:  time.Now().Add(time.Hour),
			ClientInfo: ClientInfo{
				RemoteAddr: "127.0.0.1:12345",
				UserAgent:  "test-client/1.0",
			},
		}
		err := store.Set(ctx, id, session)
		if err != nil {
			t.Fatalf("Failed to set session %s: %v", id, err)
		}
	}

	// List sessions
	sessions, err = store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != len(sessionIDs) {
		t.Errorf("Expected %d sessions, got %d", len(sessionIDs), len(sessions))
	}

	// Verify all sessions are present
	foundIDs := make(map[string]bool)
	for _, session := range sessions {
		foundIDs[session.ID] = true
	}

	for _, expectedID := range sessionIDs {
		if !foundIDs[expectedID] {
			t.Errorf("Expected session ID %s not found in list", expectedID)
		}
	}
}

func TestMemoryStore_Count(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	ctx := context.Background()
	
	// Initially empty
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// Add sessions and check count
	for i := 1; i <= 5; i++ {
		session := &Session{
			ID:         "session-" + string(rune('0'+i)),
			CreatedAt:  time.Now(),
			LastAccess: time.Now(),
			ExpiresAt:  time.Now().Add(time.Hour),
			ClientInfo: ClientInfo{
				RemoteAddr: "127.0.0.1:12345",
				UserAgent:  "test-client/1.0",
			},
		}
		err := store.Set(ctx, session.ID, session)
		if err != nil {
			t.Fatalf("Failed to set session: %v", err)
		}

		count, err = store.Count(ctx)
		if err != nil {
			t.Fatalf("Failed to get count: %v", err)
		}
		if count != i {
			t.Errorf("Expected count %d, got %d", i, count)
		}
	}
}

func TestMemoryStore_Close(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)

	ctx := context.Background()
	
	// Add some sessions
	for i := 1; i <= 3; i++ {
		session := &Session{
			ID:         "session-" + string(rune('0'+i)),
			CreatedAt:  time.Now(),
			LastAccess: time.Now(),
			ExpiresAt:  time.Now().Add(time.Hour),
			ClientInfo: ClientInfo{
				RemoteAddr: "127.0.0.1:12345",
				UserAgent:  "test-client/1.0",
			},
		}
		err := store.Set(ctx, session.ID, session)
		if err != nil {
			t.Fatalf("Failed to set session: %v", err)
		}
	}

	// Verify sessions exist
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get count: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}

	// Close store
	err = store.Close()
	if err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	// Verify sessions are cleared
	count, err = store.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get count after close: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 after close, got %d", count)
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	ctx := context.Background()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	
	// Test concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				sessionID := fmt.Sprintf("session-%d-%d", goroutineID, j)
				session := &Session{
					ID:         sessionID,
					CreatedAt:  time.Now(),
					LastAccess: time.Now(),
					ExpiresAt:  time.Now().Add(time.Hour),
					ClientInfo: ClientInfo{
						RemoteAddr: "127.0.0.1:12345",
						UserAgent:  "test-client/1.0",
					},
				}
				err := store.Set(ctx, sessionID, session)
				if err != nil {
					t.Errorf("Failed to set session %s: %v", sessionID, err)
				}
			}
		}(i)
	}
	wg.Wait()

	// Verify all sessions were stored
	expectedCount := numGoroutines * numOperations
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to get count: %v", err)
	}
	if count != expectedCount {
		t.Errorf("Expected count %d, got %d", expectedCount, count)
	}

	// Test concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				sessionID := fmt.Sprintf("session-%d-%d", goroutineID, j)
				_, err := store.Get(ctx, sessionID)
				if err != nil {
					t.Errorf("Failed to get session %s: %v", sessionID, err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestMemoryStore_GetStats(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	ctx := context.Background()
	
	// Initially empty
	stats := store.GetStats()
	if stats["total_sessions"] != 0 {
		t.Errorf("Expected 0 total sessions, got %v", stats["total_sessions"])
	}
	if stats["store_type"] != "memory" {
		t.Errorf("Expected store_type 'memory', got %v", stats["store_type"])
	}

	// Add a session
	session := &Session{
		ID:         "test-session",
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
		ClientInfo: ClientInfo{
			RemoteAddr: "127.0.0.1:12345",
			UserAgent:  "test-client/1.0",
		},
	}
	err := store.Set(ctx, session.ID, session)
	if err != nil {
		t.Fatalf("Failed to set session: %v", err)
	}

	// Check stats again
	stats = store.GetStats()
	if stats["total_sessions"] != 1 {
		t.Errorf("Expected 1 total session, got %v", stats["total_sessions"])
	}
}