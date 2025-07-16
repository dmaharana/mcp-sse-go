package server

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test session timeout
	if cfg.SessionTimeout != time.Hour {
		t.Errorf("Expected SessionTimeout to be 1 hour, got %v", cfg.SessionTimeout)
	}

	// Test cleanup interval
	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("Expected CleanupInterval to be 5 minutes, got %v", cfg.CleanupInterval)
	}

	// Test require session
	if !cfg.RequireSession {
		t.Error("Expected RequireSession to be true")
	}

	// Test log level
	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel to be 'info', got %s", cfg.LogLevel)
	}
}

func TestConfigNonZeroValues(t *testing.T) {
	cfg := DefaultConfig()

	// Ensure no zero values that would cause panics
	if cfg.SessionTimeout <= 0 {
		t.Error("SessionTimeout should be positive")
	}

	if cfg.CleanupInterval <= 0 {
		t.Error("CleanupInterval should be positive")
	}
}