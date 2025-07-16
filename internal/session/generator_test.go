package session

import (
	"strings"
	"testing"
	"time"
)

func TestSessionIDGenerator_Generate(t *testing.T) {
	generator := NewSessionIDGenerator()

	// Test basic generation
	sessionID, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}

	if sessionID == "" {
		t.Fatal("Generated session ID is empty")
	}

	// Test format
	parts := strings.Split(sessionID, ".")
	if len(parts) != 3 {
		t.Fatalf("Expected 3 parts in session ID, got %d: %s", len(parts), sessionID)
	}

	if parts[0] != SessionIDPrefix {
		t.Fatalf("Expected prefix %s, got %s", SessionIDPrefix, parts[0])
	}

	// Test uniqueness
	sessionID2, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate second session ID: %v", err)
	}

	if sessionID == sessionID2 {
		t.Fatal("Generated session IDs are not unique")
	}

	// Test multiple generations for uniqueness
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generator.Generate()
		if err != nil {
			t.Fatalf("Failed to generate session ID %d: %v", i, err)
		}
		if ids[id] {
			t.Fatalf("Duplicate session ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestSessionIDGenerator_Validate(t *testing.T) {
	generator := NewSessionIDGenerator()

	tests := []struct {
		name      string
		sessionID string
		wantError bool
	}{
		{
			name:      "empty session ID",
			sessionID: "",
			wantError: true,
		},
		{
			name:      "invalid format - too few parts",
			sessionID: "sess.123",
			wantError: true,
		},
		{
			name:      "invalid format - too many parts",
			sessionID: "sess.123.abc.def",
			wantError: true,
		},
		{
			name:      "invalid prefix",
			sessionID: "invalid.123.abc",
			wantError: true,
		},
		{
			name:      "invalid timestamp - non-numeric",
			sessionID: "sess.abc.def",
			wantError: true,
		},
		{
			name:      "missing random part",
			sessionID: "sess.123.",
			wantError: true,
		},
		{
			name:      "invalid characters in random part",
			sessionID: "sess.123.abc@def",
			wantError: true,
		},
		{
			name:      "random part too short",
			sessionID: "sess.123.a",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := generator.Validate(tt.sessionID)
			if tt.wantError && err == nil {
				t.Errorf("Expected error for session ID %s, got nil", tt.sessionID)
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error for session ID %s, got %v", tt.sessionID, err)
			}
		})
	}

	// Test valid generated session ID
	validID, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate valid session ID: %v", err)
	}

	t.Logf("Generated session ID: %s", validID)
	if err := generator.Validate(validID); err != nil {
		t.Errorf("Valid generated session ID failed validation: %v (ID: %s)", err, validID)
	}
}

func TestSessionIDGenerator_ExtractTimestamp(t *testing.T) {
	generator := NewSessionIDGenerator()

	// Generate a session ID
	sessionID, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}

	// Extract timestamp
	timestamp, err := generator.ExtractTimestamp(sessionID)
	if err != nil {
		t.Fatalf("Failed to extract timestamp: %v", err)
	}

	// Check timestamp is reasonable (within last minute)
	now := time.Now().Unix()
	if timestamp > now || timestamp < now-60 {
		t.Errorf("Extracted timestamp %d is not reasonable (now: %d)", timestamp, now)
	}

	// Test with invalid session ID
	_, err = generator.ExtractTimestamp("invalid_session_id")
	if err == nil {
		t.Error("Expected error when extracting timestamp from invalid session ID")
	}
}

func BenchmarkSessionIDGenerator_Generate(b *testing.B) {
	generator := NewSessionIDGenerator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.Generate()
		if err != nil {
			b.Fatalf("Failed to generate session ID: %v", err)
		}
	}
}

func BenchmarkSessionIDGenerator_Validate(b *testing.B) {
	generator := NewSessionIDGenerator()

	// Generate a valid session ID for benchmarking
	sessionID, err := generator.Generate()
	if err != nil {
		b.Fatalf("Failed to generate session ID: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := generator.Validate(sessionID)
		if err != nil {
			b.Fatalf("Validation failed: %v", err)
		}
	}
}