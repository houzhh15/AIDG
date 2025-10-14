package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

// ConcurrencyLimiter controls the maximum number of concurrent executions per command.
// Each command has its own semaphore configured by max_concurrent in the config.
type ConcurrencyLimiter struct {
	semaphores map[string]*semaphore.Weighted
	config     *Config
	mu         sync.RWMutex
}

// NewConcurrencyLimiter creates a new limiter based on the provided configuration.
// It initializes a semaphore for each command with its max_concurrent limit.
func NewConcurrencyLimiter(config *Config) *ConcurrencyLimiter {
	limiter := &ConcurrencyLimiter{
		semaphores: make(map[string]*semaphore.Weighted),
		config:     config,
	}

	// Create a semaphore for each command
	for _, cmd := range config.Commands {
		limiter.semaphores[cmd.Name] = semaphore.NewWeighted(int64(cmd.MaxConcurrent))
	}

	return limiter
}

// Acquire attempts to acquire a slot for the given command.
// It blocks until a slot is available or the 30-second timeout is reached.
// Returns an error if the command is not configured or the timeout is exceeded.
func (l *ConcurrencyLimiter) Acquire(ctx context.Context, commandName string) error {
	// Get the semaphore for this command
	l.mu.RLock()
	sem, exists := l.semaphores[commandName]
	l.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no semaphore configured for command: %s", commandName)
	}

	// Create a 30-second timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Try to acquire the semaphore
	if err := sem.Acquire(timeoutCtx, 1); err != nil {
		return fmt.Errorf("failed to acquire semaphore for command %s: %w", commandName, err)
	}

	return nil
}

// Release releases a slot for the given command.
// This should be called after the command execution completes (in a defer statement).
func (l *ConcurrencyLimiter) Release(commandName string) {
	// Get the semaphore for this command
	l.mu.RLock()
	sem, exists := l.semaphores[commandName]
	l.mu.RUnlock()

	if exists {
		sem.Release(1)
	}
}
