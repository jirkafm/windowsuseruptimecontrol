package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileLogger struct {
	mu          sync.Mutex
	servicePath string
	apiPath     string
}

func New(servicePath, apiPath string) *FileLogger {
	return &FileLogger{
		servicePath: servicePath,
		apiPath:     apiPath,
	}
}

func (l *FileLogger) Servicef(format string, args ...any) {
	l.write(l.servicePath, format, args...)
}

func (l *FileLogger) APIf(format string, args ...any) {
	l.write(l.apiPath, format, args...)
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

func (l *FileLogger) write(path, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	_, _ = f.WriteString(line)
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
