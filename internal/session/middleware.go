package session

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
)

// SessionMiddleware handles HTTP session validation and management
type SessionMiddleware struct {
	manager SessionManager
	config  MiddlewareConfig
	logger  zerolog.Logger
}

// MiddlewareConfig contains configuration for the session middleware
type MiddlewareConfig struct {
	RequireSession bool     // Whether to require session for all requests
	ExcludedPaths  []string // Paths that don't require session validation
	HeaderName     string   // Name of the session header (default: "Mcp-Session-Id")
}

// DefaultMiddlewareConfig returns default middleware configuration
func DefaultMiddlewareConfig() MiddlewareConfig {
	return MiddlewareConfig{
		RequireSession: true,
		ExcludedPaths: []string{
			"/health",
			"/config",
			"/static/",
			"/.mcp/ide-config",
			"/sessions",
			"/sse",
			"/metrics",
		},
		HeaderName: "Mcp-Session-Id",
	}
}

// NewSessionMiddleware creates a new session middleware
func NewSessionMiddleware(manager SessionManager, config MiddlewareConfig, logger zerolog.Logger) *SessionMiddleware {
	return &SessionMiddleware{
		manager: manager,
		config:  config,
		logger:  logger.With().Str("component", "session_middleware").Logger(),
	}
}

// contextKey for storing session in request context
type sessionContextKey string

const (
	SessionContextKey sessionContextKey = "session"
)

// Handler returns the HTTP middleware handler function
func (m *SessionMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is excluded from session validation
			if m.isPathExcluded(r.URL.Path) {
				m.logger.Debug().
					Str("path", r.URL.Path).
					Msg("Path excluded from session validation")
				next.ServeHTTP(w, r)
				return
			}

			// Handle OPTIONS requests (CORS preflight)
			if r.Method == http.MethodOptions {
				m.logger.Debug().
					Str("path", r.URL.Path).
					Msg("OPTIONS request - skipping session validation")
				next.ServeHTTP(w, r)
				return
			}

			// Extract session ID from header
			sessionID := r.Header.Get(m.config.HeaderName)
			
			if sessionID == "" {
				if m.config.RequireSession {
					m.logger.Debug().
						Str("path", r.URL.Path).
						Str("method", r.Method).
						Str("header_name", m.config.HeaderName).
						Msg("Missing session ID header")
					m.sendError(w, http.StatusBadRequest, "Missing session ID header", map[string]interface{}{
						"required_header": m.config.HeaderName,
					})
					return
				}
				// Session not required, continue without session
				next.ServeHTTP(w, r)
				return
			}

			// Validate session
			session, err := m.manager.ValidateSession(r.Context(), sessionID)
			if err != nil {
				m.logger.Debug().
					Err(err).
					Str("session_id", sessionID).
					Str("path", r.URL.Path).
					Msg("Session validation failed")
				
				// Determine appropriate HTTP status code based on error type
				statusCode := http.StatusUnauthorized
				if sessionErr, ok := err.(*SessionError); ok {
					switch sessionErr.Code {
					case ErrSessionInvalid:
						statusCode = http.StatusBadRequest
					case ErrSessionNotFound, ErrSessionExpired:
						statusCode = http.StatusUnauthorized
					default:
						statusCode = http.StatusInternalServerError
					}
				}

				m.sendError(w, statusCode, err.Error(), map[string]interface{}{
					"session_id": sessionID,
					"error_code": getErrorCode(err),
				})
				return
			}

			// Refresh session on successful validation
			if err := m.manager.RefreshSession(r.Context(), sessionID); err != nil {
				m.logger.Warn().
					Err(err).
					Str("session_id", sessionID).
					Msg("Failed to refresh session")
				// Continue processing - refresh failure shouldn't block the request
			}

			m.logger.Debug().
				Str("session_id", sessionID).
				Str("path", r.URL.Path).
				Str("remote_addr", session.ClientInfo.RemoteAddr).
				Msg("Session validation successful")

			// Add session to request context
			ctx := context.WithValue(r.Context(), SessionContextKey, session)
			r = r.WithContext(ctx)

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// isPathExcluded checks if a path should be excluded from session validation
func (m *SessionMiddleware) isPathExcluded(path string) bool {
	for _, excludedPath := range m.config.ExcludedPaths {
		if strings.HasPrefix(path, excludedPath) {
			return true
		}
	}
	return false
}

// sendError sends a JSON error response
func (m *SessionMiddleware) sendError(w http.ResponseWriter, statusCode int, message string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"code":    statusCode,
		},
	}

	if details != nil {
		errorResponse["error"].(map[string]interface{})["details"] = details
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		m.logger.Error().
			Err(err).
			Int("status_code", statusCode).
			Msg("Failed to encode error response")
	}
}

// getErrorCode extracts error code from SessionError
func getErrorCode(err error) string {
	if sessionErr, ok := err.(*SessionError); ok {
		return sessionErr.Code
	}
	return "UNKNOWN_ERROR"
}

// GetSessionFromContext retrieves the session from request context
func GetSessionFromContext(ctx context.Context) (*Session, bool) {
	session, ok := ctx.Value(SessionContextKey).(*Session)
	return session, ok
}

// RequireSession is a helper middleware that ensures a session exists in context
func RequireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetSessionFromContext(r.Context())
		if !ok || session == nil {
			http.Error(w, "Session required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// SessionInfo represents session information for responses
type SessionInfo struct {
	ID         string    `json:"id"`
	ExpiresAt  string    `json:"expires_at"`
	RemoteAddr string    `json:"remote_addr"`
	UserAgent  string    `json:"user_agent"`
}

// GetSessionInfo extracts session information for API responses
func GetSessionInfo(session *Session) SessionInfo {
	return SessionInfo{
		ID:         session.ID,
		ExpiresAt:  session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		RemoteAddr: session.ClientInfo.RemoteAddr,
		UserAgent:  session.ClientInfo.UserAgent,
	}
}