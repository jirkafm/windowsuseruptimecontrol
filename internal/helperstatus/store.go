package helperstatus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Heartbeat struct {
	UserSID   string    `json:"user_sid"`
	SessionID uint32    `json:"session_id"`
	PID       int       `json:"pid"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: root}
}

func (s *Store) Record(userSID string, hb Heartbeat) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(hb, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(userSID), data, 0o644)
}

func (s *Store) Load(userSID string) (Heartbeat, error) {
	data, err := os.ReadFile(s.path(userSID))
	if err != nil {
		return Heartbeat{}, err
	}
	var hb Heartbeat
	if err := json.Unmarshal(data, &hb); err != nil {
		return Heartbeat{}, err
	}
	return hb, nil
}

func IsFresh(hb Heartbeat, now time.Time, ttl time.Duration) bool {
	return !hb.UpdatedAt.IsZero() && now.Sub(hb.UpdatedAt) <= ttl
}

func (s *Store) path(userSID string) string {
	return filepath.Join(s.root, userSID+".json")
}
