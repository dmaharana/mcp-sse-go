package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// SessionIDLength is the length of the random part in bytes
	SessionIDLength = 32
	// SessionIDPrefix is the prefix for session IDs
	SessionIDPrefix = "sess"
)

// SessionIDGenerator handles secure session ID generation
type SessionIDGenerator struct{}

// NewSessionIDGenerator creates a new session ID generator
func NewSessionIDGenerator() *SessionIDGenerator {
	return &SessionIDGenerator{}
}

// Generate creates a new cryptographically secure session ID
func (g *SessionIDGenerator) Generate() (string, error) {
	// Generate random bytes
	randomBytes := make([]byte, SessionIDLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", NewSessionGenerationError(err)
	}

	// Encode to base64 URL-safe format (no padding)
	randomPart := base64.RawURLEncoding.EncodeToString(randomBytes)

	// Create timestamp part for debugging/ordering
	timestamp := time.Now().Unix()

	// Combine parts: sess.timestamp.randompart (using dots to avoid conflict with base64)
	sessionID := fmt.Sprintf("%s.%d.%s", SessionIDPrefix, timestamp, randomPart)

	return sessionID, nil
}

// Validate checks if a session ID has the correct format
func (g *SessionIDGenerator) Validate(sessionID string) error {
	if sessionID == "" {
		return NewSessionInvalidError("empty session ID")
	}

	// Check basic format: sess.timestamp.randompart
	parts := strings.Split(sessionID, ".")
	if len(parts) != 3 {
		return NewSessionInvalidError("invalid session ID format")
	}

	// Check prefix
	if parts[0] != SessionIDPrefix {
		return NewSessionInvalidError("invalid session ID prefix")
	}

	// Check timestamp part (should be numeric)
	timestampRegex := regexp.MustCompile(`^\d+$`)
	if !timestampRegex.MatchString(parts[1]) {
		return NewSessionInvalidError("invalid timestamp in session ID")
	}

	// Check random part (should be base64 URL-safe)
	randomPart := parts[2]
	if len(randomPart) == 0 {
		return NewSessionInvalidError("missing random part in session ID")
	}

	// Validate base64 URL-safe encoding
	base64Regex := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if !base64Regex.MatchString(randomPart) {
		return NewSessionInvalidError("invalid characters in session ID")
	}

	// Check minimum length for security
	expectedLength := base64.RawURLEncoding.EncodedLen(SessionIDLength)
	if len(randomPart) < expectedLength {
		return NewSessionInvalidError("session ID random part too short")
	}

	return nil
}

// ExtractTimestamp extracts the timestamp from a session ID for debugging
func (g *SessionIDGenerator) ExtractTimestamp(sessionID string) (int64, error) {
	if err := g.Validate(sessionID); err != nil {
		return 0, err
	}

	parts := strings.Split(sessionID, ".")
	var timestamp int64
	if _, err := fmt.Sscanf(parts[1], "%d", &timestamp); err != nil {
		return 0, NewSessionInvalidError("failed to parse timestamp")
	}

	return timestamp, nil
}