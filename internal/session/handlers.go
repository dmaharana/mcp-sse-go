package session

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// SessionHandler handles HTTP endpoints for session management
type SessionHandler struct {
	manager SessionManager
	logger  zerolog.Logger
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(manager SessionManager, logger zerolog.Logger) *SessionHandler {
	return &SessionHandler{
		manager: manager,
		logger:  logger.With().Str("component", "session_handler").Logger(),
	}
}

// CreateSessionRequest represents the request body for session creation
type CreateSessionRequest struct {
	ClientInfo *ClientInfo `json:"client_info,omitempty"`
}

// CreateSessionResponse represents the response for session creation
type CreateSessionResponse struct {
	Success bool        `json:"success"`
	Session SessionInfo `json:"session"`
	Message string      `json:"message"`
}

// SessionStatusResponse represents the response for session status
type SessionStatusResponse struct {
	Success bool        `json:"success"`
	Session SessionInfo `json:"session"`
	Message string      `json:"message"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success bool                   `json:"success"`
	Error   map[string]interface{} `json:"error"`
}

// CreateSession handles POST /sessions - creates a new session
func (h *SessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	h.logger.Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Msg("Session creation request")

	// Extract client info from request
	clientInfo := ClientInfo{
		RemoteAddr: r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}

	// Parse request body if provided (optional)
	var req CreateSessionRequest
	if r.Header.Get("Content-Type") == "application/json" && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Debug().
				Err(err).
				Msg("Failed to decode request body, using default client info")
		} else if req.ClientInfo != nil {
			// Use client info from request body if provided
			if req.ClientInfo.RemoteAddr != "" {
				clientInfo.RemoteAddr = req.ClientInfo.RemoteAddr
			}
			if req.ClientInfo.UserAgent != "" {
				clientInfo.UserAgent = req.ClientInfo.UserAgent
			}
		}
	}

	// Create session
	session, err := h.manager.CreateSession(r.Context(), clientInfo)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("remote_addr", clientInfo.RemoteAddr).
			Str("user_agent", clientInfo.UserAgent).
			Msg("Failed to create session")
		h.sendError(w, http.StatusInternalServerError, "Failed to create session", map[string]interface{}{
			"error_code": getErrorCode(err),
		})
		return
	}

	// Set session ID in response header
	w.Header().Set("Mcp-Session-Id", session.ID)

	// Send success response
	response := CreateSessionResponse{
		Success: true,
		Session: GetSessionInfo(session),
		Message: "Session created successfully",
	}

	h.sendJSON(w, http.StatusCreated, response)

	h.logger.Info().
		Str("session_id", session.ID).
		Str("remote_addr", clientInfo.RemoteAddr).
		Time("expires_at", session.ExpiresAt).
		Msg("Session created successfully")
}

// GetSession handles GET /sessions/{sessionId} - gets session status
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	// Extract session ID from URL path or header
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		// Try to extract from URL path (e.g., /sessions/sess.123.abc)
		// This would require URL routing, for now we require header
		h.sendError(w, http.StatusBadRequest, "Session ID required in Mcp-Session-Id header", nil)
		return
	}

	h.logger.Debug().
		Str("session_id", sessionID).
		Str("remote_addr", r.RemoteAddr).
		Msg("Session status request")

	// Validate session
	session, err := h.manager.ValidateSession(r.Context(), sessionID)
	if err != nil {
		h.logger.Debug().
			Err(err).
			Str("session_id", sessionID).
			Msg("Session validation failed")

		statusCode := http.StatusUnauthorized
		if sessionErr, ok := err.(*SessionError); ok {
			switch sessionErr.Code {
			case ErrSessionInvalid:
				statusCode = http.StatusBadRequest
			case ErrSessionNotFound, ErrSessionExpired:
				statusCode = http.StatusNotFound
			}
		}

		h.sendError(w, statusCode, err.Error(), map[string]interface{}{
			"session_id": sessionID,
			"error_code": getErrorCode(err),
		})
		return
	}

	// Send session info
	response := SessionStatusResponse{
		Success: true,
		Session: GetSessionInfo(session),
		Message: "Session is active",
	}

	h.sendJSON(w, http.StatusOK, response)

	h.logger.Debug().
		Str("session_id", sessionID).
		Time("expires_at", session.ExpiresAt).
		Msg("Session status retrieved")
}

// DeleteSession handles DELETE /sessions/{sessionId} - deletes a session
func (h *SessionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	// Extract session ID from header
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		h.sendError(w, http.StatusBadRequest, "Session ID required in Mcp-Session-Id header", nil)
		return
	}

	h.logger.Info().
		Str("session_id", sessionID).
		Str("remote_addr", r.RemoteAddr).
		Msg("Session deletion request")

	// Delete session
	err := h.manager.DeleteSession(r.Context(), sessionID)
	if err != nil {
		h.logger.Debug().
			Err(err).
			Str("session_id", sessionID).
			Msg("Session deletion failed")

		statusCode := http.StatusInternalServerError
		if sessionErr, ok := err.(*SessionError); ok {
			switch sessionErr.Code {
			case ErrSessionNotFound:
				statusCode = http.StatusNotFound
			}
		}

		h.sendError(w, statusCode, err.Error(), map[string]interface{}{
			"session_id": sessionID,
			"error_code": getErrorCode(err),
		})
		return
	}

	// Send success response
	response := map[string]interface{}{
		"success": true,
		"message": "Session deleted successfully",
		"session_id": sessionID,
	}

	h.sendJSON(w, http.StatusOK, response)

	h.logger.Info().
		Str("session_id", sessionID).
		Msg("Session deleted successfully")
}

// RefreshSession handles PUT /sessions/{sessionId}/refresh - refreshes a session
func (h *SessionHandler) RefreshSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	// Extract session ID from header
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		h.sendError(w, http.StatusBadRequest, "Session ID required in Mcp-Session-Id header", nil)
		return
	}

	h.logger.Debug().
		Str("session_id", sessionID).
		Str("remote_addr", r.RemoteAddr).
		Msg("Session refresh request")

	// Refresh session
	err := h.manager.RefreshSession(r.Context(), sessionID)
	if err != nil {
		h.logger.Debug().
			Err(err).
			Str("session_id", sessionID).
			Msg("Session refresh failed")

		statusCode := http.StatusInternalServerError
		if sessionErr, ok := err.(*SessionError); ok {
			switch sessionErr.Code {
			case ErrSessionInvalid:
				statusCode = http.StatusBadRequest
			case ErrSessionNotFound, ErrSessionExpired:
				statusCode = http.StatusNotFound
			}
		}

		h.sendError(w, statusCode, err.Error(), map[string]interface{}{
			"session_id": sessionID,
			"error_code": getErrorCode(err),
		})
		return
	}

	// Get updated session info
	session, err := h.manager.ValidateSession(r.Context(), sessionID)
	if err != nil {
		// This shouldn't happen after successful refresh, but handle it
		h.logger.Error().
			Err(err).
			Str("session_id", sessionID).
			Msg("Failed to get session after refresh")
		h.sendError(w, http.StatusInternalServerError, "Session refresh succeeded but failed to retrieve updated session", nil)
		return
	}

	// Send updated session info
	response := SessionStatusResponse{
		Success: true,
		Session: GetSessionInfo(session),
		Message: "Session refreshed successfully",
	}

	h.sendJSON(w, http.StatusOK, response)

	h.logger.Debug().
		Str("session_id", sessionID).
		Time("new_expires_at", session.ExpiresAt).
		Msg("Session refreshed successfully")
}

// GetSessionStats handles GET /sessions/stats - gets session statistics
func (h *SessionHandler) GetSessionStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	h.logger.Debug().
		Str("remote_addr", r.RemoteAddr).
		Msg("Session stats request")

	// Get session statistics
	stats, err := h.manager.GetSessionStats(r.Context())
	if err != nil {
		h.logger.Error().
			Err(err).
			Msg("Failed to get session stats")
		h.sendError(w, http.StatusInternalServerError, "Failed to get session statistics", map[string]interface{}{
			"error_code": getErrorCode(err),
		})
		return
	}

	// Add timestamp
	stats["timestamp"] = time.Now().Format(time.RFC3339)

	response := map[string]interface{}{
		"success": true,
		"stats":   stats,
		"message": "Session statistics retrieved successfully",
	}

	h.sendJSON(w, http.StatusOK, response)

	h.logger.Debug().
		Interface("stats", stats).
		Msg("Session stats retrieved")
}

// sendJSON sends a JSON response
func (h *SessionHandler) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().
			Err(err).
			Int("status_code", statusCode).
			Msg("Failed to encode JSON response")
	}
}

// sendError sends a JSON error response
func (h *SessionHandler) sendError(w http.ResponseWriter, statusCode int, message string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)

	errorResponse := ErrorResponse{
		Success: false,
		Error: map[string]interface{}{
			"message": message,
			"code":    statusCode,
		},
	}

	if details != nil {
		errorResponse.Error["details"] = details
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		h.logger.Error().
			Err(err).
			Int("status_code", statusCode).
			Msg("Failed to encode error response")
	}
}