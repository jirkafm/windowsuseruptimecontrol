package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type RotationConfig struct {
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

type FileLogger struct {
	mu            sync.Mutex
	servicePath   string
	apiPath       string
	serviceWriter io.Writer
	apiWriter     io.Writer
}

func New(servicePath, apiPath string) *FileLogger {
	return NewWithRotation(servicePath, apiPath, RotationConfig{
		MaxSizeMB:  10,
		MaxBackups: 10,
		MaxAgeDays: 365,
		Compress:   true,
	})
}

func NewWithRotation(servicePath, apiPath string, cfg RotationConfig) *FileLogger {
	cfg = normalizeRotationConfig(cfg)
	return &FileLogger{
		servicePath: servicePath,
		apiPath:     apiPath,
		serviceWriter: &lumberjack.Logger{
			Filename:   servicePath,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			LocalTime:  true,
			Compress:   cfg.Compress,
		},
		apiWriter: &lumberjack.Logger{
			Filename:   apiPath,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			LocalTime:  true,
			Compress:   cfg.Compress,
		},
	}
}

func (l *FileLogger) Servicef(format string, args ...any) {
	l.write(l.servicePath, l.serviceWriter, format, args...)
}

func (l *FileLogger) APIf(format string, args ...any) {
	l.write(l.apiPath, l.apiWriter, format, args...)
}

func (l *FileLogger) Recent(limit int) ([]string, error) {
	lines, err := readLines(l.servicePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	apiLines, err := readLines(l.apiPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	lines = append(lines, apiLines...)
	if len(lines) <= limit {
		return lines, nil
	}
	return lines[len(lines)-limit:], nil
}

func (l *FileLogger) write(path string, writer io.Writer, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	_, _ = writer.Write([]byte(line))
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func normalizeRotationConfig(cfg RotationConfig) RotationConfig {
	if cfg.MaxSizeMB == 0 {
		cfg.MaxSizeMB = 10
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 10
	}
	if cfg.MaxAgeDays == 0 {
		cfg.MaxAgeDays = 365
	}
	return cfg
}
