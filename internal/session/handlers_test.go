package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestSessionHandler_CreateSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Test POST request
	req := httptest.NewRequest("POST", "/sessions", nil)
	w := httptest.NewRecorder()

	handler.CreateSession(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	// Check response
	var response CreateSessionResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success=true")
	}

	if response.Session.ID == "" {
		t.Error("Expected session ID in response")
	}

	if response.Message != "Session created successfully" {
		t.Errorf("Expected success message, got %s", response.Message)
	}

	// Check session ID header
	sessionID := w.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Error("Expected Mcp-Session-Id header in response")
	}

	if sessionID != response.Session.ID {
		t.Errorf("Session ID in header (%s) doesn't match response (%s)", sessionID, response.Session.ID)
	}

	// Verify session was actually created
	_, err = manager.ValidateSession(context.Background(), sessionID)
	if err != nil {
		t.Errorf("Created session should be valid: %v", err)
	}
}

func TestSessionHandler_CreateSession_WithRequestBody(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Create request with custom client info
	reqBody := CreateSessionRequest{
		ClientInfo: &ClientInfo{
			RemoteAddr: "192.168.1.100:54321",
			UserAgent:  "custom-client/1.0",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/sessions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateSession(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	// Check response
	var response CreateSessionResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Session.RemoteAddr != reqBody.ClientInfo.RemoteAddr {
		t.Errorf("Expected remote addr %s, got %s", reqBody.ClientInfo.RemoteAddr, response.Session.RemoteAddr)
	}

	if response.Session.UserAgent != reqBody.ClientInfo.UserAgent {
		t.Errorf("Expected user agent %s, got %s", reqBody.ClientInfo.UserAgent, response.Session.UserAgent)
	}
}

func TestSessionHandler_CreateSession_WrongMethod(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Test GET request (should be POST)
	req := httptest.NewRequest("GET", "/sessions", nil)
	w := httptest.NewRecorder()

	handler.CreateSession(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestSessionHandler_GetSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Create a session first
	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test GET request
	req := httptest.NewRequest("GET", "/sessions/"+session.ID, nil)
	req.Header.Set("Mcp-Session-Id", session.ID)
	w := httptest.NewRecorder()

	handler.GetSession(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response
	var response SessionStatusResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success=true")
	}

	if response.Session.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, response.Session.ID)
	}

	if response.Message != "Session is active" {
		t.Errorf("Expected active message, got %s", response.Message)
	}
}

func TestSessionHandler_GetSession_MissingHeader(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Test GET request without session header
	req := httptest.NewRequest("GET", "/sessions", nil)
	w := httptest.NewRecorder()

	handler.GetSession(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSessionHandler_GetSession_InvalidSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Test GET request with invalid session
	req := httptest.NewRequest("GET", "/sessions/invalid", nil)
	req.Header.Set("Mcp-Session-Id", "invalid-session")
	w := httptest.NewRecorder()

	handler.GetSession(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSessionHandler_DeleteSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Create a session first
	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test DELETE request
	req := httptest.NewRequest("DELETE", "/sessions/"+session.ID, nil)
	req.Header.Set("Mcp-Session-Id", session.ID)
	w := httptest.NewRecorder()

	handler.DeleteSession(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response
	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true")
	}

	if response["session_id"] != session.ID {
		t.Errorf("Expected session ID %s, got %v", session.ID, response["session_id"])
	}

	// Verify session was actually deleted
	_, err = manager.ValidateSession(ctx, session.ID)
	if err == nil {
		t.Error("Session should have been deleted")
	}
}

func TestSessionHandler_RefreshSession(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Create a session first
	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}
	session, err := manager.CreateSession(ctx, clientInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	originalExpiresAt := session.ExpiresAt

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	// Test PUT request
	req := httptest.NewRequest("PUT", "/sessions/"+session.ID+"/refresh", nil)
	req.Header.Set("Mcp-Session-Id", session.ID)
	w := httptest.NewRecorder()

	handler.RefreshSession(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response
	var response SessionStatusResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Expected success=true")
	}

	if response.Session.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, response.Session.ID)
	}

	if response.Message != "Session refreshed successfully" {
		t.Errorf("Expected refresh message, got %s", response.Message)
	}

	// Verify session was actually refreshed
	updatedSession, err := manager.ValidateSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Session should still be valid after refresh: %v", err)
	}

	if !updatedSession.ExpiresAt.After(originalExpiresAt) {
		t.Error("Session expiration should have been updated")
	}
}

func TestSessionHandler_GetSessionStats(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Create some sessions
	ctx := context.Background()
	clientInfo := ClientInfo{
		RemoteAddr: "127.0.0.1:12345",
		UserAgent:  "test-client/1.0",
	}

	for i := 0; i < 3; i++ {
		_, err := manager.CreateSession(ctx, clientInfo)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Test GET request
	req := httptest.NewRequest("GET", "/sessions/stats", nil)
	w := httptest.NewRecorder()

	handler.GetSessionStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check response
	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["success"] != true {
		t.Error("Expected success=true")
	}

	stats, ok := response["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected stats object in response")
	}

	if stats["total_sessions"] != float64(3) {
		t.Errorf("Expected 3 total sessions, got %v", stats["total_sessions"])
	}

	if stats["active_sessions"] != float64(3) {
		t.Errorf("Expected 3 active sessions, got %v", stats["active_sessions"])
	}

	// Should have timestamp
	if stats["timestamp"] == nil {
		t.Error("Expected timestamp in stats")
	}
}

func TestSessionHandler_WrongMethods(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	tests := []struct {
		name     string
		method   string
		handler  func(http.ResponseWriter, *http.Request)
		expected int
	}{
		{"GetSession with POST", "POST", handler.GetSession, http.StatusMethodNotAllowed},
		{"DeleteSession with GET", "GET", handler.DeleteSession, http.StatusMethodNotAllowed},
		{"RefreshSession with GET", "GET", handler.RefreshSession, http.StatusMethodNotAllowed},
		{"GetSessionStats with POST", "POST", handler.GetSessionStats, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()

			tt.handler(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestSessionHandler_ErrorResponses(t *testing.T) {
	logger := zerolog.Nop()
	store := NewMemoryStore(logger)
	defer store.Close()

	managerConfig := ManagerConfig{
		SessionTimeout: time.Hour,
	}
	manager := NewDefaultSessionManager(store, managerConfig, logger)
	handler := NewSessionHandler(manager, logger)

	// Test invalid JSON in create request
	req := httptest.NewRequest("POST", "/sessions", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateSession(w, req)

	// Should still succeed (invalid JSON is ignored, defaults are used)
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201 even with invalid JSON, got %d", w.Code)
	}
}