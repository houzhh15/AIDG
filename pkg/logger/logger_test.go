package logger

import (
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestLevelFromString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expect  slog.Level
		expectErr bool
	}{
		{"debug", "debug", slog.LevelDebug, false},
		{"default-info", "", slog.LevelInfo, false},
		{"warn", "warn", slog.LevelWarn, false},
		{"error", "error", slog.LevelError, false},
		{"invalid", "verbose", slog.LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, err := levelFromString(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tt.input)
				}
				if !strings.Contains(err.Error(), "invalid log level") {
					t.Fatalf("unexpected error message: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if level != tt.expect {
				t.Fatalf("expected %v, got %v", tt.expect, level)
			}
		})
	}
}

func TestInitAndL(t *testing.T) {
	t.Cleanup(func() {
		// reset singleton for other tests
		once = sync.Once{}
		global = nil
	})

	logger, err := Init(Config{Level: "debug", Environment: "dev", WithSource: true})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if logger == nil {
		t.Fatalf("Init returned nil logger")
	}

	if L() != logger {
		t.Fatalf("L did not return initialized logger")
	}

	// second init should return same instance without error
	logger2, err := Init(Config{Level: "info", Environment: "prod"})
	if err != nil {
		t.Fatalf("unexpected error on second init: %v", err)
	}
	if logger2 != logger {
		t.Fatalf("expected same logger instance on re-init")
	}
}
