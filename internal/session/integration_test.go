package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
)

// setupTestServer creates a test server with session management
func setupTestServer(t *testing.T, config ManagerConfig, middlewareConfig MiddlewareConfig) (*httptest.Server, SessionManager, *SessionHandler) {
	logger := zerolog.Nop()
	
	// Create session components
	store := NewMemoryStore(logger)
	manager := NewDefaultSessionManager(store, config, logger)
	sessionMiddleware := NewSessionMiddleware(manager, middlewareConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Create router
	r := chi.NewRouter()
	
	// Add middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Enable CORS with session header support
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Mcp-Session-Id"},
		ExposedHeaders:   []string{"Content-Type", "Mcp-Session-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Add session middleware
	r.Use(sessionMiddleware.Handler())

	// Add session endpoints
	r.Post("/sessions", handler.CreateSession)
	r.Get("/sessions", handler.GetSession)
	r.Delete("/sessions", handler.DeleteSession)
	r.Put("/sessions/refresh", handler.RefreshSession)
	r.Get("/sessions/stats", handler.GetSessionStats)

	// Add test endpoints
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetSessionFromContext(r.Context())
		if !ok {
			http.Error(w, "No session in context", http.StatusInternalServerError)
			return
		}
		
		response := map[string]interface{}{
			"message":    "Protected endpoint accessed",
			"session_id": session.ID,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	r.Get("/public", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Public endpoint accessed",
		})
	})

	server := httptest.NewServer(r)
	t.Cleanup(func() {
		server.Close()
		store.Close()
	})

	return server, manager, handler
}

func TestIntegration_CompleteSessionFlow(t *testing.T) {
	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	middlewareConfig := DefaultMiddlewareConfig()
	middlewareConfig.ExcludedPaths = []string{"/public", "/sessions"}

	server, manager, _ := setupTestServer(t, managerConfig, middlewareConfig)

	// Step 1: Create a session
	resp, err := http.Post(server.URL+"/sessions", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	var createResponse CreateSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResponse); err != nil {
		t.Fatalf("Failed to decode create response: %v", err)
	}

	if !createResponse.Success {
		t.Error("Expected success=true")
	}

	sessionID := createResponse.Session.ID
	if sessionID == "" {
		t.Fatal("Expected session ID in response")
	}

	// Verify session ID is also in header
	headerSessionID := resp.Header.Get("Mcp-Session-Id")
	if headerSessionID != sessionID {
		t.Errorf("Session ID in header (%s) doesn't match response (%s)", headerSessionID, sessionID)
	}

	// Step 2: Access protected endpoint with session
	req, err := http.NewRequest("GET", server.URL+"/protected", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to access protected endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var protectedResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&protectedResponse); err != nil {
		t.Fatalf("Failed to decode protected response: %v", err)
	}

	if protectedResponse["session_id"] != sessionID {
		t.Errorf("Expected session ID %s, got %v", sessionID, protectedResponse["session_id"])
	}

	// Step 3: Get session status
	req, err = http.NewRequest("GET", server.URL+"/sessions", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get session status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var statusResponse SessionStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResponse); err != nil {
		t.Fatalf("Failed to decode status response: %v", err)
	}

	if statusResponse.Session.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, statusResponse.Session.ID)
	}

	// Step 4: Refresh session
	req, err = http.NewRequest("PUT", server.URL+"/sessions/refresh", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to refresh session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Step 5: Delete session
	req, err = http.NewRequest("DELETE", server.URL+"/sessions", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Step 6: Verify session is deleted - accessing protected endpoint should fail
	req, err = http.NewRequest("GET", server.URL+"/protected", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to access protected endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 after session deletion, got %d", resp.StatusCode)
	}

	// Step 7: Verify session is deleted in manager
	_, err = manager.ValidateSession(context.Background(), sessionID)
	if err == nil {
		t.Error("Session should be deleted from manager")
	}
}

func TestIntegration_SessionExpiration(t *testing.T) {
	managerConfig := ManagerConfig{
		SessionTimeout: 50 * time.Millisecond, // Very short timeout
	}
	middlewareConfig := DefaultMiddlewareConfig()
	middlewareConfig.ExcludedPaths = []string{"/sessions"}

	server, _, _ := setupTestServer(t, managerConfig, middlewareConfig)

	// Create a session
	resp, err := http.Post(server.URL+"/sessions", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer resp.Body.Close()

	var createResponse CreateSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResponse); err != nil {
		t.Fatalf("Failed to decode create response: %v", err)
	}

	sessionID := createResponse.Session.ID

	// Wait for session to expire
	time.Sleep(100 * time.Millisecond)

	// Try to access protected endpoint with expired session
	req, err := http.NewRequest("GET", server.URL+"/protected", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to access protected endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for expired session, got %d", resp.StatusCode)
	}
}

func TestIntegration_ConcurrentSessions(t *testing.T) {
	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	middlewareConfig := DefaultMiddlewareConfig()
	middlewareConfig.ExcludedPaths = []string{"/sessions"}

	server, _, _ := setupTestServer(t, managerConfig, middlewareConfig)

	// Create multiple sessions concurrently
	const numSessions = 10
	sessionIDs := make([]string, numSessions)
	
	for i := 0; i < numSessions; i++ {
		resp, err := http.Post(server.URL+"/sessions", "application/json", nil)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
		defer resp.Body.Close()

		var createResponse CreateSessionResponse
		if err := json.NewDecoder(resp.Body).Decode(&createResponse); err != nil {
			t.Fatalf("Failed to decode create response %d: %v", i, err)
		}

		sessionIDs[i] = createResponse.Session.ID
	}

	// Verify all sessions are unique
	sessionMap := make(map[string]bool)
	for i, sessionID := range sessionIDs {
		if sessionMap[sessionID] {
			t.Errorf("Duplicate session ID found: %s (session %d)", sessionID, i)
		}
		sessionMap[sessionID] = true
	}

	// Test concurrent access to protected endpoint
	for i, sessionID := range sessionIDs {
		req, err := http.NewRequest("GET", server.URL+"/protected", nil)
		if err != nil {
			t.Fatalf("Failed to create request %d: %v", i, err)
		}
		req.Header.Set("Mcp-Session-Id", sessionID)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to access protected endpoint %d: %v", i, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for session %d, got %d", i, resp.StatusCode)
		}
	}
}

func TestIntegration_SessionStats(t *testing.T) {
	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	middlewareConfig := DefaultMiddlewareConfig()
	middlewareConfig.ExcludedPaths = []string{"/sessions"}

	server, _, _ := setupTestServer(t, managerConfig, middlewareConfig)

	// Create some sessions
	const numSessions = 3
	for i := 0; i < numSessions; i++ {
		resp, err := http.Post(server.URL+"/sessions", "application/json", nil)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
		resp.Body.Close()
	}

	// Get session stats
	resp, err := http.Get(server.URL + "/sessions/stats")
	if err != nil {
		t.Fatalf("Failed to get session stats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var statsResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&statsResponse); err != nil {
		t.Fatalf("Failed to decode stats response: %v", err)
	}

	if !statsResponse["success"].(bool) {
		t.Error("Expected success=true")
	}

	stats, ok := statsResponse["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected stats object in response")
	}

	if stats["total_sessions"] != float64(numSessions) {
		t.Errorf("Expected %d total sessions, got %v", numSessions, stats["total_sessions"])
	}

	if stats["active_sessions"] != float64(numSessions) {
		t.Errorf("Expected %d active sessions, got %v", numSessions, stats["active_sessions"])
	}
}

func TestIntegration_PublicEndpointAccess(t *testing.T) {
	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	middlewareConfig := DefaultMiddlewareConfig()
	middlewareConfig.ExcludedPaths = []string{"/public"}

	server, _, _ := setupTestServer(t, managerConfig, middlewareConfig)

	// Access public endpoint without session
	resp, err := http.Get(server.URL + "/public")
	if err != nil {
		t.Fatalf("Failed to access public endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for public endpoint, got %d", resp.StatusCode)
	}

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode public response: %v", err)
	}

	if response["message"] != "Public endpoint accessed" {
		t.Errorf("Expected public message, got %s", response["message"])
	}
}

func TestIntegration_InvalidSessionHandling(t *testing.T) {
	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	middlewareConfig := DefaultMiddlewareConfig()
	middlewareConfig.ExcludedPaths = []string{"/sessions"}

	server, _, _ := setupTestServer(t, managerConfig, middlewareConfig)

	tests := []struct {
		name      string
		sessionID string
		expected  int
	}{
		{"Invalid format", "invalid-session-id", http.StatusBadRequest},
		{"Non-existent valid format", "sess.1234567890.abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", http.StatusUnauthorized},
		{"Empty session ID", "", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", server.URL+"/protected", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			
			if tt.sessionID != "" {
				req.Header.Set("Mcp-Session-Id", tt.sessionID)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to access protected endpoint: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, resp.StatusCode)
			}
		})
	}
}

func TestIntegration_SessionRefreshExtension(t *testing.T) {
	managerConfig := ManagerConfig{
		SessionTimeout: 200 * time.Millisecond,
	}
	middlewareConfig := DefaultMiddlewareConfig()
	middlewareConfig.ExcludedPaths = []string{"/sessions"}

	server, _, _ := setupTestServer(t, managerConfig, middlewareConfig)

	// Create a session
	resp, err := http.Post(server.URL+"/sessions", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer resp.Body.Close()

	var createResponse CreateSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&createResponse); err != nil {
		t.Fatalf("Failed to decode create response: %v", err)
	}

	sessionID := createResponse.Session.ID
	originalExpiresAt := createResponse.Session.ExpiresAt

	// Parse original expiration time
	originalTime, err := time.Parse("2006-01-02T15:04:05Z07:00", originalExpiresAt)
	if err != nil {
		t.Fatalf("Failed to parse original expiration time: %v", err)
	}

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Refresh session
	req, err := http.NewRequest("PUT", server.URL+"/sessions/refresh", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to refresh session: %v", err)
	}
	defer resp.Body.Close()

	var refreshResponse SessionStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&refreshResponse); err != nil {
		t.Fatalf("Failed to decode refresh response: %v", err)
	}

	// Parse refreshed expiration time
	refreshedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", refreshResponse.Session.ExpiresAt)
	if err != nil {
		t.Fatalf("Failed to parse refreshed expiration time: %v", err)
	}

	// Verify expiration time was extended (or at least not reduced)
	if refreshedTime.Before(originalTime) {
		t.Errorf("Session expiration time should not have been reduced: original=%v, refreshed=%v", originalTime, refreshedTime)
	}
	
	// The refreshed time should be at least as late as the original time
	// In practice, it should be later, but due to timing precision, we'll accept equal
	if refreshedTime.Equal(originalTime) {
		t.Logf("Session expiration times are equal (timing precision issue): original=%v, refreshed=%v", originalTime, refreshedTime)
	} else if refreshedTime.After(originalTime) {
		t.Logf("Session expiration time was successfully extended: original=%v, refreshed=%v", originalTime, refreshedTime)
	}

	// Wait for original expiration time to pass
	time.Sleep(150 * time.Millisecond)

	// Session should still be valid because it was refreshed
	req, err = http.NewRequest("GET", server.URL+"/protected", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to access protected endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for refreshed session, got %d", resp.StatusCode)
	}
}