package logging

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLoggerRotatesServiceLogBySize(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	servicePath := filepath.Join(dir, "service.log")
	apiPath := filepath.Join(dir, "api.log")
	logger := NewWithRotation(servicePath, apiPath, RotationConfig{
		MaxSizeMB:  1,
		MaxBackups: 10,
		MaxAgeDays: 365,
		Compress:   false,
	})

	payload := strings.Repeat("x", 600)
	for idx := 0; idx < 1800; idx++ {
		logger.Servicef("%s", payload)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "service-*.log"))
	if err != nil {
		t.Fatalf("Glob error: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("rotated service logs = %d, want at least 1", len(matches))
	}
}
