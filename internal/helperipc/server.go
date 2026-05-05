package helperipc

import (
	"context"
	"errors"
	"sync"
)

type Server struct {
	mu      sync.RWMutex
	helpers map[string]helperConnection
}

type helperConnection struct {
	sessionID uint32
	commands  chan Command
}

func NewServer() *Server {
	return &Server{helpers: map[string]helperConnection{}}
}

func (s *Server) Register(userSID string, sessionID uint32) (<-chan Command, func()) {
	commands := make(chan Command, 16)

	s.mu.Lock()
	if current, ok := s.helpers[userSID]; ok {
		close(current.commands)
	}
	s.helpers[userSID] = helperConnection{sessionID: sessionID, commands: commands}
	s.mu.Unlock()

	unregister := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		current, ok := s.helpers[userSID]
		if ok && current.commands == commands {
			delete(s.helpers, userSID)
			close(commands)
		}
	}
	return commands, unregister
}

func (s *Server) Connected(userSID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.helpers[userSID]
	return ok
}

func (s *Server) Speak(ctx context.Context, userSID, message string) error {
	return s.Send(ctx, userSID, Command{Type: CommandSpeak, Message: message})
}

func (s *Server) Send(ctx context.Context, userSID string, cmd Command) error {
	s.mu.RLock()
	helper, ok := s.helpers[userSID]
	if !ok {
		s.mu.RUnlock()
		return errors.New("helper not connected")
	}

	select {
	case helper.commands <- cmd:
		s.mu.RUnlock()
		return nil
	case <-ctx.Done():
		s.mu.RUnlock()
		return ctx.Err()
	}
}
