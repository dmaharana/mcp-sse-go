package session

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// CleanupService handles automatic cleanup of expired sessions
type CleanupService struct {
	manager  SessionManager
	interval time.Duration
	logger   zerolog.Logger
	
	// Control channels
	stopCh   chan struct{}
	stoppedCh chan struct{}
	
	// State management
	mutex   sync.RWMutex
	running bool
}

// CleanupConfig contains configuration for the cleanup service
type CleanupConfig struct {
	CleanupInterval time.Duration
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(manager SessionManager, config CleanupConfig, logger zerolog.Logger) *CleanupService {
	return &CleanupService{
		manager:   manager,
		interval:  config.CleanupInterval,
		logger:    logger.With().Str("component", "cleanup_service").Logger(),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
		running:   false,
	}
}

// Start begins the cleanup service background goroutine
func (c *CleanupService) Start(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		c.logger.Warn().Msg("Cleanup service is already running")
		return nil
	}

	c.logger.Info().
		Dur("interval", c.interval).
		Msg("Starting session cleanup service")

	c.running = true
	go c.run(ctx)

	return nil
}

// Stop gracefully stops the cleanup service
func (c *CleanupService) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		c.logger.Debug().Msg("Cleanup service is not running")
		return nil
	}

	c.logger.Info().Msg("Stopping session cleanup service")

	// Signal stop
	close(c.stopCh)

	// Wait for goroutine to finish
	<-c.stoppedCh

	c.running = false
	c.logger.Info().Msg("Session cleanup service stopped")

	return nil
}

// IsRunning returns whether the cleanup service is currently running
func (c *CleanupService) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.running
}

// RunOnce performs a single cleanup operation
func (c *CleanupService) RunOnce(ctx context.Context) (int, error) {
	c.logger.Debug().Msg("Running session cleanup")

	startTime := time.Now()
	deletedCount, err := c.manager.CleanupExpiredSessions(ctx)
	duration := time.Since(startTime)

	if err != nil {
		c.logger.Error().
			Err(err).
			Dur("duration", duration).
			Msg("Session cleanup failed")
		return 0, err
	}

	if deletedCount > 0 {
		c.logger.Info().
			Int("deleted_count", deletedCount).
			Dur("duration", duration).
			Msg("Session cleanup completed")
	} else {
		c.logger.Debug().
			Dur("duration", duration).
			Msg("Session cleanup completed - no expired sessions found")
	}

	return deletedCount, nil
}

// run is the main cleanup loop
func (c *CleanupService) run(ctx context.Context) {
	defer close(c.stoppedCh)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.logger.Debug().Msg("Cleanup service goroutine started")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("Cleanup service stopping due to context cancellation")
			return

		case <-c.stopCh:
			c.logger.Info().Msg("Cleanup service stopping due to stop signal")
			return

		case <-ticker.C:
			// Create a timeout context for the cleanup operation
			cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			
			_, err := c.RunOnce(cleanupCtx)
			if err != nil {
				c.logger.Error().
					Err(err).
					Msg("Cleanup operation failed")
			}
			
			cancel()
		}
	}
}

// GetStats returns statistics about the cleanup service
func (c *CleanupService) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return map[string]interface{}{
		"running":          c.running,
		"cleanup_interval": c.interval.String(),
	}
}

// SetInterval updates the cleanup interval (requires restart to take effect)
func (c *CleanupService) SetInterval(interval time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.interval = interval
	c.logger.Info().
		Dur("new_interval", interval).
		Msg("Cleanup interval updated")
}