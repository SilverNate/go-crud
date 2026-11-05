package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewMapsLevels(t *testing.T) {
	tests := []struct {
		in       string
		enabled  slog.Level
		disabled slog.Level
	}{
		{"debug", slog.LevelDebug, -100},
		{"DEBUG", slog.LevelDebug, -100},
		{"info", slog.LevelInfo, slog.LevelDebug},
		{"", slog.LevelInfo, slog.LevelDebug},
		{"unknown", slog.LevelInfo, slog.LevelDebug},
		{"warn", slog.LevelWarn, slog.LevelInfo},
		{"warning", slog.LevelWarn, slog.LevelInfo},
		{"error", slog.LevelError, slog.LevelWarn},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			log := New(tt.in)
			ctx := context.Background()
			if !log.Enabled(ctx, tt.enabled) {
				t.Fatalf("level %v should be enabled for %q", tt.enabled, tt.in)
			}
			if tt.disabled != -100 && log.Enabled(ctx, tt.disabled) {
				t.Fatalf("level %v should be disabled for %q", tt.disabled, tt.in)
			}
		})
	}
}
