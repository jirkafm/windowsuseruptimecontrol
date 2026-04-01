package helperipc

import (
	"context"
	"errors"
	"sync"
)

type Server struct {
	mu      sync.RWMutex
	helpers map[string]chan Command
}

func NewServer() *Server {
	return &Server{helpers: map[string]chan Command{}}
}

func (s *Server) Register(userSID string, commands chan Command) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.helpers[userSID] = commands
}

func (s *Server) Send(ctx context.Context, userSID string, cmd Command) error {
	s.mu.RLock()
	helper, ok := s.helpers[userSID]
	s.mu.RUnlock()
	if !ok {
		return errors.New("helper not connected")
	}

	select {
	case helper <- cmd:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
