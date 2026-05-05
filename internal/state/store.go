package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"windowsuseruptimecontrol/internal/model"
)

type JSONStore struct {
	path string
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

func (s *JSONStore) LoadOrCreate(now time.Time, defaultDailyAllowanceSec int64) (model.StateFile, error) {
	date := now.Format("2006-01-02")
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.StateFile{ServiceDate: date, Users: map[string]model.UserDayState{}}, nil
		}
		return model.StateFile{}, fmt.Errorf("read state: %w", err)
	}

	var state model.StateFile
	if err := json.Unmarshal(data, &state); err != nil {
		backup := fmt.Sprintf("%s.corrupt-%d", s.path, now.Unix())
		if renameErr := os.Rename(s.path, backup); renameErr != nil {
			return model.StateFile{}, fmt.Errorf("rename corrupt state: %w", renameErr)
		}
		return model.StateFile{ServiceDate: date, Users: map[string]model.UserDayState{}}, nil
	}

	if state.Users == nil {
		state.Users = map[string]model.UserDayState{}
	}

	for sid, user := range state.Users {
		if user.Date == date {
			user.RecalculateRemaining()
			state.Users[sid] = user
			continue
		}

		user.Date = date
		user.ConsumedSec = 0
		if user.DailyAllowanceSec == 0 {
			user.DailyAllowanceSec = defaultDailyAllowanceSec
		}
		user.RecalculateRemaining()
		user.Exhausted = false
		user.StartupWarningSent = false
		user.HalfwayWarningSent = false
		user.FiveMinWarningSent = false
		user.CustomConsumedWarningsSent = nil
		user.ReenforcementPending = false
		user.ReenforcementDeadline = time.Time{}
		user.LastEnforcementReason = ""
		state.Users[sid] = user
	}

	state.ServiceDate = date
	return state, nil
}

func (s *JSONStore) Save(state model.StateFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return os.WriteFile(s.path, data, 0o600)
}
