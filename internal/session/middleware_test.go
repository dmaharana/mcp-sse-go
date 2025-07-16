package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestSessionMiddleware_Handler_ExcludedPaths(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	middlewareConfig := DefaultMiddlewareConfig()
	middleware := NewSessionMiddleware(manager, middlewareConfig, logger)

	// Test handler that sets a flag when called
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Test excluded paths
	excludedPaths := []string{"/health", "/config", "/static/test.css", "/.mcp/ide-config"}
	
	for _, path := range excludedPaths {
		t.Run("excluded_path_"+path, func(t *testing.T) {
			called = false
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if !called {
				t.Error("Handler should have been called for excluded path")
			}
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestSessionMiddleware_Handler_OptionsRequest(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	middlewareConfig := DefaultMiddlewareConfig()
	middleware := NewSessionMiddleware(manager, middlewareConfig, logger)

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Test OPTIONS request
	req := httptest.NewRequest("OPTIONS", "/sse", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Handler should have been called for OPTIONS request")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSessionMiddleware_Handler_MissingSessionHeader(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	middlewareConfig := DefaultMiddlewareConfig()
	middleware := NewSessionMiddleware(manager, middlewareConfig, logger)

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Test request without session header
	req := httptest.NewRequest("POST", "/sse", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Handler should not have been called without session header")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Check error response
	var errorResponse map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&errorResponse)
	if err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	errorObj, ok := errorResponse["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Error response should contain error object")
	}

	if errorObj["message"] != "Missing session ID header" {
		t.Errorf("Expected error message about missing header, got %v", errorObj["message"])
	}
}

func TestSessionMiddleware_Handler_InvalidSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	middlewareConfig := DefaultMiddlewareConfig()
	middleware := NewSessionMiddleware(manager, middlewareConfig, logger)

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Test request with invalid session ID
	req := httptest.NewRequest("POST", "/sse", nil)
	req.Header.Set("Mcp-Session-Id", "invalid-session-id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Handler should not have been called with invalid session")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSessionMiddleware_Handler_NonExistentSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	middlewareConfig := DefaultMiddlewareConfig()
	middleware := NewSessionMiddleware(manager, middlewareConfig, logger)

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Generate a valid session ID format but don't store it
	generator := NewSessionIDGenerator()
	validSessionID, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}

	// Test request with non-existent session ID
	req := httptest.NewRequest("POST", "/sse", nil)
	req.Header.Set("Mcp-Session-Id", validSessionID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Handler should not have been called with non-existent session")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestSessionMiddleware_Handler_ExpiredSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Millisecond, // Very short timeout
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	middlewareConfig := DefaultMiddlewareConfig()
	middleware := NewSessionMiddleware(manager, middlewareConfig, logger)

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Create a session
	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Wait for session to expire
	time.Sleep(10 * time.Millisecond)

	// Test request with expired session
	req := httptest.NewRequest("POST", "/sse", nil)
	req.Header.Set("Mcp-Session-Id", session.ID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Handler should not have been called with expired session")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestSessionMiddleware_Handler_ValidSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	middlewareConfig := DefaultMiddlewareConfig()
	middleware := NewSessionMiddleware(manager, middlewareConfig, logger)

	var capturedSession *Session
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract session from context
		session, ok := GetSessionFromContext(r.Context())
		if ok {
			capturedSession = session
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Create a session
	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test request with valid session
	req := httptest.NewRequest("POST", "/sse", nil)
	req.Header.Set("Mcp-Session-Id", session.ID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if capturedSession == nil {
		t.Fatal("Session should have been added to context")
	}

	if capturedSession.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, capturedSession.ID)
	}
}

func TestSessionMiddleware_Handler_SessionNotRequired(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)

	middlewareConfig := DefaultMiddlewareConfig()
	middlewareConfig.RequireSession = false // Don't require session
	middleware := NewSessionMiddleware(manager, middlewareConfig, logger)

	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Handler()(testHandler)

	// Test request without session header when session is not required
	req := httptest.NewRequest("POST", "/sse", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Handler should have been called when session is not required")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetSessionFromContext(t *testing.T) {
	// Test with no session in context
	ctx := context.Background()
	session, ok := GetSessionFromContext(ctx)
	if ok || session != nil {
		t.Error("Should return false and nil when no session in context")
	}

	// Test with session in context
	testSession := &Session{
		ID:         "test-session",
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
		ClientInfo: ClientInfo{
			RemoteAddr: "127.0.0.1:12345",
			UserAgent:  "test-client/1.0",
		},
	}

	ctx = context.WithValue(ctx, SessionContextKey, testSession)
	session, ok = GetSessionFromContext(ctx)
	if !ok || session == nil {
		t.Error("Should return true and session when session exists in context")
	}
	if session.ID != testSession.ID {
		t.Errorf("Expected session ID %s, got %s", testSession.ID, session.ID)
	}
}

func TestRequireSession(t *testing.T) {
	called := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireSession(testHandler)

	// Test without session in context
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Handler should not have been called without session")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	// Test with session in context
	called = false
	testSession := &Session{
		ID:         "test-session",
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
		ClientInfo: ClientInfo{
			RemoteAddr: "127.0.0.1:12345",
			UserAgent:  "test-client/1.0",
		},
	}

	ctx := context.WithValue(req.Context(), SessionContextKey, testSession)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Handler should have been called with session")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetSessionInfo(t *testing.T) {
	session := &Session{
		ID:         "test-session-123",
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
		ExpiresAt:  time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		ClientInfo: ClientInfo{
			RemoteAddr: "192.168.1.100:54321",
			UserAgent:  "test-client/2.0",
		},
	}

	info := GetSessionInfo(session)

	if info.ID != session.ID {
		t.Errorf("Expected ID %s, got %s", session.ID, info.ID)
	}
	if info.RemoteAddr != session.ClientInfo.RemoteAddr {
		t.Errorf("Expected remote addr %s, got %s", session.ClientInfo.RemoteAddr, info.RemoteAddr)
	}
	if info.UserAgent != session.ClientInfo.UserAgent {
		t.Errorf("Expected user agent %s, got %s", session.ClientInfo.UserAgent, info.UserAgent)
	}
	if info.ExpiresAt != "2024-01-01T12:00:00Z" {
		t.Errorf("Expected expires at 2024-01-01T12:00:00Z, got %s", info.ExpiresAt)
	}
}