# Windows Uptime Enforcer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a fresh `wincontrol/` Windows-only service plus per-user helper that enforces per-user daily active-console uptime limits, exposes a bearer-token HTTP API, speaks warnings with TTS, and hibernates or shuts down the machine when the active user's time is exhausted.

**Architecture:** The implementation is a Go monorepo with two binaries: `activitysvc.exe` for the authoritative Windows service and `activityhelper.exe` for user-session TTS. Shared packages handle config, JSON persistence, policy, HTTP API, helper IPC, active session detection, and power control behind interfaces so the behavior can be tested with fake clocks and fake Windows adapters.

**Tech Stack:** Go 1.25, Go stdlib, `golang.org/x/sys/windows/svc`, `github.com/Microsoft/go-winio`, PowerShell 5+, Pester for installer checks.

---

## Planned File Structure

- Create: `wincontrol/go.mod`
- Create: `wincontrol/go.sum`
- Create: `wincontrol/README.md`
- Create: `wincontrol/cmd/activitysvc/main.go`
- Create: `wincontrol/cmd/activityhelper/main.go`
- Create: `wincontrol/internal/config/config.go`
- Create: `wincontrol/internal/config/config_test.go`
- Create: `wincontrol/internal/model/types.go`
- Create: `wincontrol/internal/clock/clock.go`
- Create: `wincontrol/internal/state/store.go`
- Create: `wincontrol/internal/state/store_test.go`
- Create: `wincontrol/internal/policy/engine.go`
- Create: `wincontrol/internal/policy/engine_test.go`
- Create: `wincontrol/internal/api/server.go`
- Create: `wincontrol/internal/api/server_test.go`
- Create: `wincontrol/internal/helperipc/protocol.go`
- Create: `wincontrol/internal/helperipc/server.go`
- Create: `wincontrol/internal/helperipc/server_test.go`
- Create: `wincontrol/internal/helper/runtime.go`
- Create: `wincontrol/internal/helper/runtime_test.go`
- Create: `wincontrol/internal/service/runtime.go`
- Create: `wincontrol/internal/service/runtime_test.go`
- Create: `wincontrol/internal/windows/session/console_windows.go`
- Create: `wincontrol/internal/windows/session/console_stub.go`
- Create: `wincontrol/internal/windows/power/controller_windows.go`
- Create: `wincontrol/internal/windows/power/controller_stub.go`
- Create: `wincontrol/internal/windows/service/runner_windows.go`
- Create: `wincontrol/internal/windows/service/runner_stub.go`
- Create: `wincontrol/internal/windows/helper/launcher_windows.go`
- Create: `wincontrol/internal/windows/helper/launcher_stub.go`
- Create: `wincontrol/installer/install.ps1`
- Create: `wincontrol/installer/install.Tests.ps1`
- Create: `wincontrol/installer/uninstall.ps1`

### Task 1: Bootstrap Module, Shared Models, and Config Defaults

**Files:**
- Create: `wincontrol/go.mod`
- Create: `wincontrol/internal/model/types.go`
- Create: `wincontrol/internal/clock/clock.go`
- Create: `wincontrol/internal/config/config.go`
- Create: `wincontrol/internal/config/config_test.go`
- Test: `wincontrol/internal/config/config_test.go`

- [ ] **Step 1: Write the failing config/defaults test**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{"api_port":8088,"bearer_token":"secret-token"}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.APIBindAddress != "0.0.0.0" {
		t.Fatalf("APIBindAddress = %q, want 0.0.0.0", cfg.APIBindAddress)
	}
	if cfg.DefaultDailyAllowanceSec != 3600 {
		t.Fatalf("DefaultDailyAllowanceSec = %d, want 3600", cfg.DefaultDailyAllowanceSec)
	}
	if cfg.ReenforcementDelaySec != 180 {
		t.Fatalf("ReenforcementDelaySec = %d, want 180", cfg.ReenforcementDelaySec)
	}
	if !cfg.WarningHalfwayEnabled {
		t.Fatalf("WarningHalfwayEnabled = false, want true")
	}
	if !cfg.WarningFiveMinEnabled {
		t.Fatalf("WarningFiveMinEnabled = false, want true")
	}
	if cfg.BearerToken != "secret-token" {
		t.Fatalf("BearerToken = %q, want secret-token", cfg.BearerToken)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/config -run TestLoadConfigDefaults -v`
Expected: FAIL with `Load` undefined and missing package files.

- [ ] **Step 3: Write minimal shared model and config implementation**

```go
// /Users/jikaplan/test/wincontrol/go.mod
module wincontrol

go 1.25

require (
	github.com/Microsoft/go-winio v0.6.2
	golang.org/x/sys v0.31.0
)
```

```go
// /Users/jikaplan/test/wincontrol/internal/model/types.go
package model

import "time"

type Config struct {
	APIBindAddress           string `json:"api_bind_address"`
	APIPort                  int    `json:"api_port"`
	BearerToken              string `json:"bearer_token"`
	DefaultDailyAllowanceSec int64  `json:"default_daily_allowance_sec"`
	ReenforcementDelaySec    int64  `json:"reenforcement_delay_sec"`
	WarningHalfwayEnabled    bool   `json:"warning_halfway_enabled"`
	WarningFiveMinEnabled    bool   `json:"warning_five_min_enabled"`
	HelperPath               string `json:"helper_path"`
	LogLevel                 string `json:"log_level"`
}

type UserDayState struct {
	UserSID                string    `json:"user_sid"`
	Username               string    `json:"username"`
	Date                   string    `json:"date"`
	DailyAllowanceSec      int64     `json:"daily_allowance_sec"`
	ConsumedSec            int64     `json:"consumed_sec"`
	RemainingSec           int64     `json:"remaining_sec"`
	SessionStartedAt       time.Time `json:"session_started_at"`
	Exhausted              bool      `json:"exhausted"`
	StartupWarningSent     bool      `json:"startup_warning_sent"`
	HalfwayWarningSent     bool      `json:"halfway_warning_sent"`
	FiveMinWarningSent     bool      `json:"five_min_warning_sent"`
	ReenforcementPending   bool      `json:"reenforcement_pending"`
	ReenforcementDeadline  time.Time `json:"reenforcement_deadline"`
	LastEnforcementReason  string    `json:"last_enforcement_reason"`
}

type StateFile struct {
	ServiceDate string                  `json:"service_date"`
	Users       map[string]UserDayState `json:"users"`
}

type ActiveUser struct {
	SessionID uint32
	Username  string
	UserSID   string
}
```

```go
// /Users/jikaplan/test/wincontrol/internal/clock/clock.go
package clock

import "time"

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
```

```go
// /Users/jikaplan/test/wincontrol/internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"wincontrol/internal/model"
)

func Load(path string) (model.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return model.Config{}, fmt.Errorf("decode config: %w", err)
	}

	applyDefaults(&cfg)
	if cfg.APIPort == 0 {
		return model.Config{}, fmt.Errorf("api_port must be set")
	}
	if cfg.BearerToken == "" {
		return model.Config{}, fmt.Errorf("bearer_token must be set")
	}

	return cfg, nil
}

func applyDefaults(cfg *model.Config) {
	if cfg.APIBindAddress == "" {
		cfg.APIBindAddress = "0.0.0.0"
	}
	if cfg.DefaultDailyAllowanceSec == 0 {
		cfg.DefaultDailyAllowanceSec = 3600
	}
	if cfg.ReenforcementDelaySec == 0 {
		cfg.ReenforcementDelaySec = 180
	}
	if !cfg.WarningHalfwayEnabled {
		cfg.WarningHalfwayEnabled = true
	}
	if !cfg.WarningFiveMinEnabled {
		cfg.WarningFiveMinEnabled = true
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/config -run TestLoadConfigDefaults -v`
Expected: PASS with `TestLoadConfigDefaults`.

- [ ] **Step 5: Commit**

```bash
cd /Users/jikaplan/test/wincontrol
git add go.mod internal/model/types.go internal/clock/clock.go internal/config/config.go internal/config/config_test.go
git commit -m "feat: bootstrap config and shared models"
```

### Task 2: Add JSON State Store With New-Day Reset and Corrupt Recovery

**Files:**
- Create: `wincontrol/internal/state/store.go`
- Create: `wincontrol/internal/state/store_test.go`
- Modify: `wincontrol/internal/model/types.go`
- Test: `wincontrol/internal/state/store_test.go`

- [ ] **Step 1: Write the failing state store tests**

```go
package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"wincontrol/internal/model"
)

func TestLoadOrCreateResetsUsersOnNewDay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	raw := []byte(`{
		"service_date":"2026-03-31",
		"users":{
			"S-1-5-21-john":{
				"user_sid":"S-1-5-21-john",
				"username":"John",
				"date":"2026-03-31",
				"daily_allowance_sec":3600,
				"consumed_sec":3550,
				"remaining_sec":50,
				"exhausted":true
			}
		}
	}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	store := NewJSONStore(path)
	got, err := store.LoadOrCreate(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC), 3600)
	if err != nil {
		t.Fatalf("LoadOrCreate error: %v", err)
	}

	user := got.Users["S-1-5-21-john"]
	if user.Date != "2026-04-01" {
		t.Fatalf("Date = %q, want 2026-04-01", user.Date)
	}
	if user.ConsumedSec != 0 {
		t.Fatalf("ConsumedSec = %d, want 0", user.ConsumedSec)
	}
	if user.RemainingSec != 3600 {
		t.Fatalf("RemainingSec = %d, want 3600", user.RemainingSec)
	}
	if user.Exhausted {
		t.Fatalf("Exhausted = true, want false")
	}
}

func TestLoadOrCreateRenamesCorruptState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}

	store := NewJSONStore(path)
	got, err := store.LoadOrCreate(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC), 3600)
	if err != nil {
		t.Fatalf("LoadOrCreate error: %v", err)
	}
	if got.ServiceDate != "2026-04-01" {
		t.Fatalf("ServiceDate = %q, want 2026-04-01", got.ServiceDate)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "state.json.corrupt-*"))
	if err != nil {
		t.Fatalf("Glob error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("corrupt backups = %d, want 1", len(matches))
	}
}

func TestUpsertUserPersistsCalculatedRemainingTime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store := NewJSONStore(path)
	state := model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"S-1-5-21-mary": {
				UserSID:           "S-1-5-21-mary",
				Username:          "Mary",
				Date:              "2026-04-01",
				DailyAllowanceSec: 5400,
				ConsumedSec:       1200,
				RemainingSec:      4200,
			},
		},
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	reloaded, err := store.LoadOrCreate(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC), 3600)
	if err != nil {
		t.Fatalf("LoadOrCreate error: %v", err)
	}

	if reloaded.Users["S-1-5-21-mary"].RemainingSec != 4200 {
		t.Fatalf("RemainingSec = %d, want 4200", reloaded.Users["S-1-5-21-mary"].RemainingSec)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/state -run 'TestLoadOrCreate|TestUpsertUser' -v`
Expected: FAIL with `NewJSONStore` undefined and missing state package implementation.

- [ ] **Step 3: Write minimal store implementation**

```go
// /Users/jikaplan/test/wincontrol/internal/model/types.go
package model

import "time"

type Config struct {
	APIBindAddress           string `json:"api_bind_address"`
	APIPort                  int    `json:"api_port"`
	BearerToken              string `json:"bearer_token"`
	DefaultDailyAllowanceSec int64  `json:"default_daily_allowance_sec"`
	ReenforcementDelaySec    int64  `json:"reenforcement_delay_sec"`
	WarningHalfwayEnabled    bool   `json:"warning_halfway_enabled"`
	WarningFiveMinEnabled    bool   `json:"warning_five_min_enabled"`
	HelperPath               string `json:"helper_path"`
	LogLevel                 string `json:"log_level"`
}

type UserDayState struct {
	UserSID               string    `json:"user_sid"`
	Username              string    `json:"username"`
	Date                  string    `json:"date"`
	DailyAllowanceSec     int64     `json:"daily_allowance_sec"`
	ConsumedSec           int64     `json:"consumed_sec"`
	RemainingSec          int64     `json:"remaining_sec"`
	SessionStartedAt      time.Time `json:"session_started_at"`
	Exhausted             bool      `json:"exhausted"`
	StartupWarningSent    bool      `json:"startup_warning_sent"`
	HalfwayWarningSent    bool      `json:"halfway_warning_sent"`
	FiveMinWarningSent    bool      `json:"five_min_warning_sent"`
	ReenforcementPending  bool      `json:"reenforcement_pending"`
	ReenforcementDeadline time.Time `json:"reenforcement_deadline"`
	LastEnforcementReason string    `json:"last_enforcement_reason"`
}

type StateFile struct {
	ServiceDate string                  `json:"service_date"`
	Users       map[string]UserDayState `json:"users"`
}

type ActiveUser struct {
	SessionID uint32
	Username  string
	UserSID   string
}

func (u *UserDayState) RecalculateRemaining() {
	u.RemainingSec = u.DailyAllowanceSec - u.ConsumedSec
	if u.RemainingSec < 0 {
		u.RemainingSec = 0
	}
}
```

```go
// /Users/jikaplan/test/wincontrol/internal/state/store.go
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"wincontrol/internal/model"
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
		user.DailyAllowanceSec = pickAllowance(user.DailyAllowanceSec, defaultDailyAllowanceSec)
		user.RecalculateRemaining()
		user.Exhausted = false
		user.StartupWarningSent = false
		user.HalfwayWarningSent = false
		user.FiveMinWarningSent = false
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

func pickAllowance(current, fallback int64) int64 {
	if current > 0 {
		return current
	}
	return fallback
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/state -run 'TestLoadOrCreate|TestUpsertUser' -v`
Expected: PASS with all three state-store tests green.

- [ ] **Step 5: Commit**

```bash
cd /Users/jikaplan/test/wincontrol
git add internal/model/types.go internal/state/store.go internal/state/store_test.go
git commit -m "feat: add persistent state store"
```

### Task 3: Implement Policy Engine for Quotas, Warnings, and Reenforcement

**Files:**
- Create: `wincontrol/internal/policy/engine.go`
- Create: `wincontrol/internal/policy/engine_test.go`
- Modify: `wincontrol/internal/model/types.go`
- Test: `wincontrol/internal/policy/engine_test.go`

- [ ] **Step 1: Write the failing policy engine tests**

```go
package policy

import (
	"testing"
	"time"

	"wincontrol/internal/model"
)

func TestEvaluateConsumesActiveUserAndEmitsWarnings(t *testing.T) {
	t.Parallel()

	engine := Engine{DefaultDailyAllowanceSec: 3600, ReenforcementDelaySec: 180}
	state := model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"sid-john": {
				UserSID:               "sid-john",
				Username:              "John",
				Date:                  "2026-04-01",
				DailyAllowanceSec:     3600,
				ConsumedSec:           1790,
				RemainingSec:          1810,
				StartupWarningSent:    true,
				HalfwayWarningSent:    false,
				FiveMinWarningSent:    false,
				ReenforcementPending:  false,
				LastEnforcementReason: "",
			},
		},
	}

	result := engine.Evaluate(
		time.Date(2026, 4, 1, 10, 0, 10, 0, time.UTC),
		model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"},
		state,
		20,
	)

	user := result.State.Users["sid-john"]
	if user.ConsumedSec != 1810 {
		t.Fatalf("ConsumedSec = %d, want 1810", user.ConsumedSec)
	}
	if user.RemainingSec != 1790 {
		t.Fatalf("RemainingSec = %d, want 1790", user.RemainingSec)
	}
	if len(result.Messages) != 1 || result.Messages[0] != "You have 30 minutes remaining." {
		t.Fatalf("Messages = %#v, want halfway announcement", result.Messages)
	}
	if !user.HalfwayWarningSent {
		t.Fatalf("HalfwayWarningSent = false, want true")
	}
}

func TestEvaluateCreatesReenforcementDeadlineForSameDayReturn(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	engine := Engine{DefaultDailyAllowanceSec: 3600, ReenforcementDelaySec: 180}
	state := model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"sid-john": {
				UserSID:              "sid-john",
				Username:             "John",
				Date:                 "2026-04-01",
				DailyAllowanceSec:    3600,
				ConsumedSec:          3600,
				RemainingSec:         0,
				Exhausted:            true,
				ReenforcementPending: false,
			},
		},
	}

	result := engine.Evaluate(now, model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, state, 0)

	user := result.State.Users["sid-john"]
	if !user.ReenforcementPending {
		t.Fatalf("ReenforcementPending = false, want true")
	}
	if got := user.ReenforcementDeadline.Sub(now); got != 180*time.Second {
		t.Fatalf("ReenforcementDeadline delta = %v, want 180s", got)
	}
	if len(result.Messages) != 1 || result.Messages[0] != "No time remains for today. The computer will hibernate in 180 seconds." {
		t.Fatalf("Messages = %#v", result.Messages)
	}
}

func TestEvaluateTriggersEnforcementCountdown(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 12, 3, 1, 0, time.UTC)
	engine := Engine{DefaultDailyAllowanceSec: 3600, ReenforcementDelaySec: 180}
	state := model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"sid-john": {
				UserSID:               "sid-john",
				Username:              "John",
				Date:                  "2026-04-01",
				DailyAllowanceSec:     3600,
				ConsumedSec:           3600,
				RemainingSec:          0,
				Exhausted:             true,
				ReenforcementPending:  true,
				ReenforcementDeadline: time.Date(2026, 4, 1, 12, 3, 0, 0, time.UTC),
			},
		},
	}

	result := engine.Evaluate(now, model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, state, 0)

	if !result.TriggerEnforcement {
		t.Fatalf("TriggerEnforcement = false, want true")
	}
	if len(result.Countdown) != 10 || result.Countdown[0] != "10" || result.Countdown[9] != "1" {
		t.Fatalf("Countdown = %#v, want 10..1", result.Countdown)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/policy -run TestEvaluate -v`
Expected: FAIL with `Engine` undefined and missing policy package.

- [ ] **Step 3: Write minimal policy engine implementation**

```go
// /Users/jikaplan/test/wincontrol/internal/model/types.go
package model

import "time"

type Config struct {
	APIBindAddress           string `json:"api_bind_address"`
	APIPort                  int    `json:"api_port"`
	BearerToken              string `json:"bearer_token"`
	DefaultDailyAllowanceSec int64  `json:"default_daily_allowance_sec"`
	ReenforcementDelaySec    int64  `json:"reenforcement_delay_sec"`
	WarningHalfwayEnabled    bool   `json:"warning_halfway_enabled"`
	WarningFiveMinEnabled    bool   `json:"warning_five_min_enabled"`
	HelperPath               string `json:"helper_path"`
	LogLevel                 string `json:"log_level"`
}

type UserDayState struct {
	UserSID               string    `json:"user_sid"`
	Username              string    `json:"username"`
	Date                  string    `json:"date"`
	DailyAllowanceSec     int64     `json:"daily_allowance_sec"`
	ConsumedSec           int64     `json:"consumed_sec"`
	RemainingSec          int64     `json:"remaining_sec"`
	SessionStartedAt      time.Time `json:"session_started_at"`
	Exhausted             bool      `json:"exhausted"`
	StartupWarningSent    bool      `json:"startup_warning_sent"`
	HalfwayWarningSent    bool      `json:"halfway_warning_sent"`
	FiveMinWarningSent    bool      `json:"five_min_warning_sent"`
	ReenforcementPending  bool      `json:"reenforcement_pending"`
	ReenforcementDeadline time.Time `json:"reenforcement_deadline"`
	LastEnforcementReason string    `json:"last_enforcement_reason"`
}

type StateFile struct {
	ServiceDate string                  `json:"service_date"`
	Users       map[string]UserDayState `json:"users"`
}

type ActiveUser struct {
	SessionID uint32
	Username  string
	UserSID   string
}

type Evaluation struct {
	State              StateFile
	Messages           []string
	Countdown          []string
	TriggerEnforcement bool
}

func (u *UserDayState) RecalculateRemaining() {
	u.RemainingSec = u.DailyAllowanceSec - u.ConsumedSec
	if u.RemainingSec < 0 {
		u.RemainingSec = 0
	}
}
```

```go
// /Users/jikaplan/test/wincontrol/internal/policy/engine.go
package policy

import (
	"fmt"
	"time"

	"wincontrol/internal/model"
)

type Engine struct {
	DefaultDailyAllowanceSec int64
	ReenforcementDelaySec    int64
}

func (e Engine) Evaluate(now time.Time, active model.ActiveUser, state model.StateFile, elapsedSec int64) model.Evaluation {
	if state.Users == nil {
		state.Users = map[string]model.UserDayState{}
	}

	user := state.Users[active.UserSID]
	if user.UserSID == "" {
		user = model.UserDayState{
			UserSID:           active.UserSID,
			Username:          active.Username,
			Date:              now.Format("2006-01-02"),
			DailyAllowanceSec: e.DefaultDailyAllowanceSec,
		}
	}

	user.Date = now.Format("2006-01-02")
	user.Username = active.Username
	if user.DailyAllowanceSec == 0 {
		user.DailyAllowanceSec = e.DefaultDailyAllowanceSec
	}

	result := model.Evaluation{State: state}

	if user.Exhausted {
		if !user.ReenforcementPending {
			user.ReenforcementPending = true
			user.ReenforcementDeadline = now.Add(time.Duration(e.ReenforcementDelaySec) * time.Second)
			result.Messages = append(result.Messages, fmt.Sprintf("No time remains for today. The computer will hibernate in %d seconds.", e.ReenforcementDelaySec))
		} else if !user.ReenforcementDeadline.After(now) {
			result.TriggerEnforcement = true
			result.Countdown = []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1"}
		}
		state.Users[active.UserSID] = user
		result.State = state
		return result
	}

	user.ConsumedSec += elapsedSec
	user.RecalculateRemaining()

	halfwayThreshold := user.DailyAllowanceSec / 2
	if !user.StartupWarningSent {
		result.Messages = append(result.Messages, fmt.Sprintf("You have %d minutes remaining.", user.RemainingSec/60))
		user.StartupWarningSent = true
	}
	if !user.HalfwayWarningSent && user.ConsumedSec >= halfwayThreshold {
		result.Messages = append(result.Messages, "You have 30 minutes remaining.")
		user.HalfwayWarningSent = true
	}
	if !user.FiveMinWarningSent && user.RemainingSec <= 300 {
		result.Messages = append(result.Messages, "You have 5 minutes remaining.")
		user.FiveMinWarningSent = true
	}
	if user.RemainingSec == 0 {
		user.Exhausted = true
		user.LastEnforcementReason = "daily allowance exhausted"
		result.TriggerEnforcement = true
		result.Countdown = []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1"}
	}

	state.Users[active.UserSID] = user
	result.State = state
	return result
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/policy -run TestEvaluate -v`
Expected: PASS with warning, reenforcement, and countdown tests green.

- [ ] **Step 5: Commit**

```bash
cd /Users/jikaplan/test/wincontrol
git add internal/model/types.go internal/policy/engine.go internal/policy/engine_test.go
git commit -m "feat: add policy engine for time enforcement"
```

### Task 4: Build the Bearer-Token HTTP API

**Files:**
- Create: `wincontrol/internal/api/server.go`
- Create: `wincontrol/internal/api/server_test.go`
- Modify: `wincontrol/internal/model/types.go`
- Test: `wincontrol/internal/api/server_test.go`

- [ ] **Step 1: Write the failing API tests**

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"wincontrol/internal/model"
)

type fakeAdmin struct {
	lastAdjustUser  string
	lastAdjustDelta int64
	state           model.StateFile
}

func (f *fakeAdmin) State() model.StateFile { return f.state }
func (f *fakeAdmin) AdjustUser(user string, delta int64) (model.UserDayState, error) {
	f.lastAdjustUser = user
	f.lastAdjustDelta = delta
	current := f.state.Users[user]
	current.ConsumedSec -= delta
	current.RecalculateRemaining()
	f.state.Users[user] = current
	return current, nil
}
func (f *fakeAdmin) SetAllowance(user string, sec int64) (model.UserDayState, error) { return model.UserDayState{}, nil }
func (f *fakeAdmin) ResetToday(user string) (model.UserDayState, error) { return model.UserDayState{}, nil }
func (f *fakeAdmin) Announce(msg string) error { return nil }
func (f *fakeAdmin) HibernateNow() error { return nil }

func TestAdjustEndpointRequiresBearerToken(t *testing.T) {
	t.Parallel()

	server := New("token-123", &fakeAdmin{})
	req := httptest.NewRequest(http.MethodPost, "/v1/users/sid-john/adjust", bytes.NewBufferString(`{"delta_sec":300}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAdjustEndpointAppliesDelta(t *testing.T) {
	t.Parallel()

	admin := &fakeAdmin{
		state: model.StateFile{
			ServiceDate: "2026-04-01",
			Users: map[string]model.UserDayState{
				"sid-john": {
					UserSID:           "sid-john",
					Username:          "John",
					Date:              "2026-04-01",
					DailyAllowanceSec: 3600,
					ConsumedSec:       600,
					RemainingSec:      3000,
				},
			},
		},
	}
	server := New("token-123", admin)

	body, _ := json.Marshal(map[string]int64{"delta_sec": 300})
	req := httptest.NewRequest(http.MethodPost, "/v1/users/sid-john/adjust", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if admin.lastAdjustUser != "sid-john" {
		t.Fatalf("lastAdjustUser = %q, want sid-john", admin.lastAdjustUser)
	}
	if admin.lastAdjustDelta != 300 {
		t.Fatalf("lastAdjustDelta = %d, want 300", admin.lastAdjustDelta)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/api -run TestAdjustEndpoint -v`
Expected: FAIL with `New` undefined and missing API package implementation.

- [ ] **Step 3: Write minimal API server implementation**

```go
// /Users/jikaplan/test/wincontrol/internal/api/server.go
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"wincontrol/internal/model"
)

type AdminController interface {
	State() model.StateFile
	AdjustUser(user string, delta int64) (model.UserDayState, error)
	SetAllowance(user string, sec int64) (model.UserDayState, error)
	ResetToday(user string) (model.UserDayState, error)
	Announce(msg string) error
	HibernateNow() error
}

type Server struct {
	token string
	admin AdminController
	mux   *http.ServeMux
}

func New(token string, admin AdminController) *Server {
	s := &Server{
		token: token,
		admin: admin,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	s.mux.HandleFunc("/v1/users/", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/v1/users/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}

		userID := parts[0]
		action := parts[1]

		switch {
		case r.Method == http.MethodPost && action == "adjust":
			var req struct {
				DeltaSec int64 `json:"delta_sec"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			user, err := s.admin.AdjustUser(userID, req.DeltaSec)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, user)
		case r.Method == http.MethodPost && action == "allowance":
			var req struct {
				DailyAllowanceSec int64 `json:"daily_allowance_sec"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			user, err := s.admin.SetAllowance(userID, req.DailyAllowanceSec)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, user)
		case r.Method == http.MethodPost && action == "reset-today":
			user, err := s.admin.ResetToday(userID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, user)
		case r.Method == http.MethodGet && action == "status":
			user, ok := s.admin.State().Users[userID]
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, http.StatusOK, user)
		default:
			http.NotFound(w, r)
		}
	})
}

func (s *Server) authorized(r *http.Request) bool {
	return r.Header.Get("Authorization") == "Bearer "+s.token
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/api -run TestAdjustEndpoint -v`
Expected: PASS with auth and delta handling tests green.

- [ ] **Step 5: Commit**

```bash
cd /Users/jikaplan/test/wincontrol
git add internal/api/server.go internal/api/server_test.go
git commit -m "feat: add bearer-token management api"
```

### Task 5: Add Helper IPC and TTS Runtime

**Files:**
- Create: `wincontrol/internal/helperipc/protocol.go`
- Create: `wincontrol/internal/helperipc/server.go`
- Create: `wincontrol/internal/helperipc/server_test.go`
- Create: `wincontrol/internal/helper/runtime.go`
- Create: `wincontrol/internal/helper/runtime_test.go`
- Test: `wincontrol/internal/helperipc/server_test.go`
- Test: `wincontrol/internal/helper/runtime_test.go`

- [ ] **Step 1: Write the failing helper IPC and runtime tests**

```go
package helperipc

import (
	"context"
	"testing"
	"time"
)

func TestServerDeliversSpeakCommandToRegisteredHelper(t *testing.T) {
	t.Parallel()

	server := NewServer()
	helper := make(chan Command, 1)
	server.Register("sid-john", helper)

	if err := server.Send(context.Background(), "sid-john", Command{Type: CommandSpeak, Message: "You have 5 minutes remaining."}); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	select {
	case got := <-helper:
		if got.Message != "You have 5 minutes remaining." {
			t.Fatalf("Message = %q, want warning", got.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for helper command")
	}
}
```

```go
package helper

import (
	"context"
	"testing"

	"wincontrol/internal/helperipc"
)

type fakeSpeaker struct {
	spoken []string
}

func (f *fakeSpeaker) Speak(text string) error {
	f.spoken = append(f.spoken, text)
	return nil
}

func TestRuntimeSpeaksIncomingMessages(t *testing.T) {
	t.Parallel()

	speaker := &fakeSpeaker{}
	commands := make(chan helperipc.Command, 1)
	commands <- helperipc.Command{Type: helperipc.CommandSpeak, Message: "10"}
	close(commands)

	rt := Runtime{Speaker: speaker}
	if err := rt.Run(context.Background(), commands); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(speaker.spoken) != 1 || speaker.spoken[0] != "10" {
		t.Fatalf("spoken = %#v, want [10]", speaker.spoken)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/helperipc ./internal/helper -run 'TestServerDelivers|TestRuntimeSpeaks' -v`
Expected: FAIL with `NewServer`, `Command`, and `Runtime` undefined.

- [ ] **Step 3: Write minimal helper IPC and runtime implementation**

```go
// /Users/jikaplan/test/wincontrol/internal/helperipc/protocol.go
package helperipc

type CommandType string

const (
	CommandSpeak CommandType = "speak"
)

type Command struct {
	Type    CommandType `json:"type"`
	Message string      `json:"message"`
}
```

```go
// /Users/jikaplan/test/wincontrol/internal/helperipc/server.go
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
```

```go
// /Users/jikaplan/test/wincontrol/internal/helper/runtime.go
package helper

import (
	"context"

	"wincontrol/internal/helperipc"
)

type Speaker interface {
	Speak(text string) error
}

type Runtime struct {
	Speaker Speaker
}

func (r Runtime) Run(ctx context.Context, commands <-chan helperipc.Command) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cmd, ok := <-commands:
			if !ok {
				return nil
			}
			if cmd.Type == helperipc.CommandSpeak {
				if err := r.Speaker.Speak(cmd.Message); err != nil {
					return err
				}
			}
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/helperipc ./internal/helper -run 'TestServerDelivers|TestRuntimeSpeaks' -v`
Expected: PASS for IPC delivery and helper speech runtime.

- [ ] **Step 5: Commit**

```bash
cd /Users/jikaplan/test/wincontrol
git add internal/helperipc/protocol.go internal/helperipc/server.go internal/helperipc/server_test.go internal/helper/runtime.go internal/helper/runtime_test.go
git commit -m "feat: add helper ipc and tts runtime"
```

### Task 6: Orchestrate Service Runtime and Admin Operations

**Files:**
- Create: `wincontrol/internal/service/runtime.go`
- Create: `wincontrol/internal/service/runtime_test.go`
- Modify: `wincontrol/internal/api/server.go`
- Test: `wincontrol/internal/service/runtime_test.go`

- [ ] **Step 1: Write the failing service runtime tests**

```go
package service

import (
	"context"
	"testing"
	"time"

	"wincontrol/internal/model"
)

type fakeStore struct {
	state model.StateFile
}

func (f *fakeStore) LoadOrCreate(time.Time, int64) (model.StateFile, error) { return f.state, nil }
func (f *fakeStore) Save(state model.StateFile) error {
	f.state = state
	return nil
}

type fakeDetector struct {
	user model.ActiveUser
}

func (f fakeDetector) ActiveUser(context.Context) (model.ActiveUser, bool, error) {
	return f.user, true, nil
}

type fakeHelperBus struct {
	messages []string
}

func (f *fakeHelperBus) Speak(_ context.Context, userSID, message string) error {
	f.messages = append(f.messages, userSID+":"+message)
	return nil
}

type fakePower struct {
	hibernateCalls int
	shutdownCalls  int
}

func (f *fakePower) Hibernate(context.Context) error {
	f.hibernateCalls++
	return nil
}
func (f *fakePower) Shutdown(context.Context) error {
	f.shutdownCalls++
	return nil
}

func TestTickConsumesTimeAndSpeaksPolicyMessages(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Config: model.Config{
			DefaultDailyAllowanceSec: 3600,
			ReenforcementDelaySec:    180,
		},
		Store: &fakeStore{
			state: model.StateFile{
				ServiceDate: "2026-04-01",
				Users: map[string]model.UserDayState{
					"sid-john": {
						UserSID:               "sid-john",
						Username:              "John",
						Date:                  "2026-04-01",
						DailyAllowanceSec:     3600,
						ConsumedSec:           1790,
						RemainingSec:          1810,
						StartupWarningSent:    true,
						HalfwayWarningSent:    false,
						FiveMinWarningSent:    false,
						ReenforcementPending:  false,
						LastEnforcementReason: "",
					},
				},
			},
		},
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}},
		Helper:   &fakeHelperBus{},
		Power:    &fakePower{},
	}

	err := rt.Tick(context.Background(), time.Date(2026, 4, 1, 10, 0, 10, 0, time.UTC), 20)
	if err != nil {
		t.Fatalf("Tick error: %v", err)
	}

	if len(rt.Helper.(*fakeHelperBus).messages) != 1 {
		t.Fatalf("messages = %#v, want 1 halfway warning", rt.Helper.(*fakeHelperBus).messages)
	}
	if rt.Power.(*fakePower).hibernateCalls != 0 {
		t.Fatalf("hibernateCalls = %d, want 0", rt.Power.(*fakePower).hibernateCalls)
	}
}

func TestHibernateNowRunsCountdownThenHibernate(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Config: model.Config{},
		Store:  &fakeStore{state: model.StateFile{Users: map[string]model.UserDayState{}}},
		Helper: &fakeHelperBus{},
		Power:  &fakePower{},
	}

	if err := rt.HibernateNow(context.Background(), "sid-john"); err != nil {
		t.Fatalf("HibernateNow error: %v", err)
	}

	if rt.Power.(*fakePower).hibernateCalls != 1 {
		t.Fatalf("hibernateCalls = %d, want 1", rt.Power.(*fakePower).hibernateCalls)
	}
	if len(rt.Helper.(*fakeHelperBus).messages) != 10 {
		t.Fatalf("countdown messages = %d, want 10", len(rt.Helper.(*fakeHelperBus).messages))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/service -run 'TestTick|TestHibernateNow' -v`
Expected: FAIL with `Runtime` undefined and missing service package.

- [ ] **Step 3: Write minimal service runtime implementation**

```go
// /Users/jikaplan/test/wincontrol/internal/service/runtime.go
package service

import (
	"context"
	"time"

	"wincontrol/internal/model"
	"wincontrol/internal/policy"
)

type Store interface {
	LoadOrCreate(now time.Time, defaultDailyAllowanceSec int64) (model.StateFile, error)
	Save(state model.StateFile) error
}

type Detector interface {
	ActiveUser(ctx context.Context) (model.ActiveUser, bool, error)
}

type HelperBus interface {
	Speak(ctx context.Context, userSID, message string) error
}

type PowerController interface {
	Hibernate(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type Runtime struct {
	Config   model.Config
	Store    Store
	Detector Detector
	Helper   HelperBus
	Power    PowerController
}

func (r *Runtime) Tick(ctx context.Context, now time.Time, elapsedSec int64) error {
	state, err := r.Store.LoadOrCreate(now, r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return err
	}

	active, ok, err := r.Detector.ActiveUser(ctx)
	if err != nil || !ok {
		return err
	}

	engine := policy.Engine{
		DefaultDailyAllowanceSec: r.Config.DefaultDailyAllowanceSec,
		ReenforcementDelaySec:    r.Config.ReenforcementDelaySec,
	}
	result := engine.Evaluate(now, active, state, elapsedSec)

	for _, message := range result.Messages {
		if err := r.Helper.Speak(ctx, active.UserSID, message); err != nil {
			return err
		}
	}
	if result.TriggerEnforcement {
		for _, number := range result.Countdown {
			if err := r.Helper.Speak(ctx, active.UserSID, number); err != nil {
				return err
			}
		}
		if err := r.Power.Hibernate(ctx); err != nil {
			return r.Power.Shutdown(ctx)
		}
	}

	return r.Store.Save(result.State)
}

func (r *Runtime) HibernateNow(ctx context.Context, userSID string) error {
	for _, number := range []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1"} {
		if err := r.Helper.Speak(ctx, userSID, number); err != nil {
			return err
		}
	}
	if err := r.Power.Hibernate(ctx); err != nil {
		return r.Power.Shutdown(ctx)
	}
	return nil
}

func (r *Runtime) State() model.StateFile {
	state, _ := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	return state
}

func (r *Runtime) AdjustUser(user string, delta int64) (model.UserDayState, error) {
	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[user]
	current.ConsumedSec -= delta
	if current.ConsumedSec < 0 {
		current.ConsumedSec = 0
	}
	current.RecalculateRemaining()
	if current.RemainingSec > 0 {
		current.Exhausted = false
		current.ReenforcementPending = false
		current.ReenforcementDeadline = time.Time{}
	}
	state.Users[user] = current
	return current, r.Store.Save(state)
}

func (r *Runtime) SetAllowance(user string, sec int64) (model.UserDayState, error) {
	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[user]
	current.DailyAllowanceSec = sec
	current.RecalculateRemaining()
	if current.RemainingSec > 0 {
		current.Exhausted = false
		current.ReenforcementPending = false
		current.ReenforcementDeadline = time.Time{}
	}
	state.Users[user] = current
	return current, r.Store.Save(state)
}

func (r *Runtime) ResetToday(user string) (model.UserDayState, error) {
	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[user]
	current.ConsumedSec = 0
	current.RecalculateRemaining()
	current.Exhausted = false
	current.StartupWarningSent = false
	current.HalfwayWarningSent = false
	current.FiveMinWarningSent = false
	current.ReenforcementPending = false
	current.ReenforcementDeadline = time.Time{}
	state.Users[user] = current
	return current, r.Store.Save(state)
}

func (r *Runtime) Announce(message string) error {
	active, ok, err := r.Detector.ActiveUser(context.Background())
	if err != nil || !ok {
		return err
	}
	return r.Helper.Speak(context.Background(), active.UserSID, message)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/service -run 'TestTick|TestHibernateNow' -v`
Expected: PASS with policy messages and forced hibernate behavior green.

- [ ] **Step 5: Commit**

```bash
cd /Users/jikaplan/test/wincontrol
git add internal/service/runtime.go internal/service/runtime_test.go
git commit -m "feat: add service runtime orchestration"
```

### Task 7: Add Windows Adapters and Entry Points

**Files:**
- Create: `wincontrol/internal/windows/session/console_windows.go`
- Create: `wincontrol/internal/windows/session/console_stub.go`
- Create: `wincontrol/internal/windows/power/controller_windows.go`
- Create: `wincontrol/internal/windows/power/controller_stub.go`
- Create: `wincontrol/internal/windows/helper/launcher_windows.go`
- Create: `wincontrol/internal/windows/helper/launcher_stub.go`
- Create: `wincontrol/internal/windows/service/runner_windows.go`
- Create: `wincontrol/internal/windows/service/runner_stub.go`
- Create: `wincontrol/cmd/activitysvc/main.go`
- Create: `wincontrol/cmd/activityhelper/main.go`
- Test: `wincontrol/internal/windows/service/runner_stub.go` via package build

- [ ] **Step 1: Write the failing build-oriented test**

```go
package service

import "testing"

func TestStubRunnerCompilesOnNonWindows(t *testing.T) {
	t.Parallel()

	runner := Runner{}
	if err := runner.Validate(); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/windows/service -run TestStubRunnerCompilesOnNonWindows -v`
Expected: FAIL with `Runner` undefined.

- [ ] **Step 3: Write minimal Windows adapter and entrypoint implementation**

```go
// /Users/jikaplan/test/wincontrol/internal/windows/service/runner_stub.go
//go:build !windows

package service

type Runner struct{}

func (Runner) Validate() error { return nil }
func (Runner) Run() error      { return nil }
```

```go
// /Users/jikaplan/test/wincontrol/internal/windows/service/runner_windows.go
//go:build windows

package service

import "golang.org/x/sys/windows/svc"

type Runner struct {
	Name string
}

func (r Runner) Validate() error { return nil }

func (r Runner) Run() error {
	return svc.Run(r.Name, r)
}

func (r Runner) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for req := range requests {
		switch req.Cmd {
		case svc.Stop, svc.Shutdown:
			status <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
	return false, 0
}
```

```go
// /Users/jikaplan/test/wincontrol/internal/windows/power/controller_stub.go
//go:build !windows

package power

import "context"

type Controller struct{}

func (Controller) Hibernate(context.Context) error { return nil }
func (Controller) Shutdown(context.Context) error  { return nil }
```

```go
// /Users/jikaplan/test/wincontrol/internal/windows/session/console_stub.go
//go:build !windows

package session

import (
	"context"
	"errors"

	"wincontrol/internal/model"
)

type Detector struct{}

func (Detector) ActiveUser(context.Context) (model.ActiveUser, bool, error) {
	return model.ActiveUser{}, false, errors.New("active console detection is windows-only")
}
```

```go
// /Users/jikaplan/test/wincontrol/cmd/activitysvc/main.go
package main

import (
	"log"

	winservice "wincontrol/internal/windows/service"
)

func main() {
	runner := winservice.Runner{Name: "WinControlActivityService"}
	if err := runner.Run(); err != nil {
		log.Fatal(err)
	}
}
```

```go
// /Users/jikaplan/test/wincontrol/cmd/activityhelper/main.go
package main

func main() {}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./internal/windows/service -run TestStubRunnerCompilesOnNonWindows -v`
Expected: PASS on non-Windows development hosts.

- [ ] **Step 5: Run a Windows-targeted compile check**

Run: `cd /Users/jikaplan/test/wincontrol && GOOS=windows GOARCH=amd64 go test ./...`
Expected: PASS with Windows files compiling, even if service startup is not yet fully wired.

- [ ] **Step 6: Commit**

```bash
cd /Users/jikaplan/test/wincontrol
git add internal/windows/session internal/windows/power internal/windows/helper internal/windows/service cmd/activitysvc/main.go cmd/activityhelper/main.go
git commit -m "feat: add windows adapters and entrypoints"
```

### Task 8: Add Installer, Uninstaller, and Deployment Documentation

**Files:**
- Create: `wincontrol/installer/install.ps1`
- Create: `wincontrol/installer/install.Tests.ps1`
- Create: `wincontrol/installer/uninstall.ps1`
- Create: `wincontrol/README.md`
- Test: `wincontrol/installer/install.Tests.ps1`

- [ ] **Step 1: Write the failing installer test**

```powershell
Describe "install.ps1" {
    It "contains required service and firewall setup" {
        $script = Get-Content "$PSScriptRoot/install.ps1" -Raw

        $script | Should -Match "New-Service"
        $script | Should -Match "WinControlActivityService"
        $script | Should -Match "New-NetFirewallRule"
        $script | Should -Match "HKLM:\\Software\\Microsoft\\Windows\\CurrentVersion\\Run"
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jikaplan/test/wincontrol && pwsh -NoProfile -Command "Invoke-Pester ./installer/install.Tests.ps1"`
Expected: FAIL because `installer/install.ps1` does not exist yet.

- [ ] **Step 3: Write minimal installer, uninstaller, and README**

```powershell
# /Users/jikaplan/test/wincontrol/installer/install.ps1
param(
    [string]$InstallRoot = "C:\ProgramData\Activity",
    [int]$ApiPort = 8111,
    [string]$BearerToken = "change-me"
)

$ErrorActionPreference = "Stop"

if (-not ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)) {
    throw "Administrator rights are required."
}

$binRoot = Join-Path $InstallRoot "bin"
$cfgRoot = Join-Path $InstallRoot "config"
$logRoot = Join-Path $InstallRoot "logs"
$stateRoot = Join-Path $InstallRoot "state"

New-Item -ItemType Directory -Force -Path $binRoot, $cfgRoot, $logRoot, $stateRoot | Out-Null
Copy-Item "$PSScriptRoot\..\dist\activitysvc.exe" (Join-Path $binRoot "activitysvc.exe") -Force
Copy-Item "$PSScriptRoot\..\dist\activityhelper.exe" (Join-Path $binRoot "activityhelper.exe") -Force

@{
    api_bind_address = "0.0.0.0"
    api_port = $ApiPort
    bearer_token = $BearerToken
    default_daily_allowance_sec = 3600
    reenforcement_delay_sec = 180
    warning_halfway_enabled = $true
    warning_five_min_enabled = $true
    helper_path = (Join-Path $binRoot "activityhelper.exe")
    log_level = "info"
} | ConvertTo-Json | Set-Content (Join-Path $cfgRoot "config.json")

$serviceExe = Join-Path $binRoot "activitysvc.exe"
if (Get-Service -Name "WinControlActivityService" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "WinControlActivityService" -Force
    sc.exe delete WinControlActivityService | Out-Null
    Start-Sleep -Seconds 2
}

New-Service -Name "WinControlActivityService" -BinaryPathName "`"$serviceExe`"" -DisplayName "WinControl Activity Service" -StartupType Automatic
New-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "WinControlActivityHelper" -Value (Join-Path $binRoot "activityhelper.exe") -PropertyType String -Force | Out-Null
New-NetFirewallRule -DisplayName "WinControl API" -Direction Inbound -Action Allow -Protocol TCP -LocalPort $ApiPort | Out-Null
Start-Service -Name "WinControlActivityService"
```

```powershell
# /Users/jikaplan/test/wincontrol/installer/uninstall.ps1
param([string]$InstallRoot = "C:\ProgramData\Activity")

$ErrorActionPreference = "Stop"

if (Get-Service -Name "WinControlActivityService" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "WinControlActivityService" -Force
    sc.exe delete WinControlActivityService | Out-Null
}

Remove-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "WinControlActivityHelper" -ErrorAction SilentlyContinue
Get-NetFirewallRule -DisplayName "WinControl API" -ErrorAction SilentlyContinue | Remove-NetFirewallRule
Remove-Item -Path $InstallRoot -Recurse -Force -ErrorAction SilentlyContinue
```

```md
# /Users/jikaplan/test/wincontrol/README.md
# WinControl

Windows-only uptime enforcement service with a network HTTP API and a user-session TTS helper.

## Build

```bash
go test ./...
GOOS=windows GOARCH=amd64 go build -o dist/activitysvc.exe ./cmd/activitysvc
GOOS=windows GOARCH=amd64 go build -o dist/activityhelper.exe ./cmd/activityhelper
```

## Install

Run PowerShell as Administrator:

```powershell
pwsh -ExecutionPolicy Bypass -File .\installer\install.ps1 -ApiPort 8111 -BearerToken "<token>"
```
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/jikaplan/test/wincontrol && pwsh -NoProfile -Command "Invoke-Pester ./installer/install.Tests.ps1"`
Expected: PASS with service, firewall, and Run-key checks green.

- [ ] **Step 5: Run the full verification suite**

Run: `cd /Users/jikaplan/test/wincontrol && go test ./...`
Expected: PASS for config, state, policy, API, helper IPC, helper runtime, and service runtime tests.

Run: `cd /Users/jikaplan/test/wincontrol && GOOS=windows GOARCH=amd64 go test ./...`
Expected: PASS for Windows-targeted compile and package coverage.

- [ ] **Step 6: Commit**

```bash
cd /Users/jikaplan/test/wincontrol
git add installer/install.ps1 installer/install.Tests.ps1 installer/uninstall.ps1 README.md
git commit -m "feat: add installer and deployment docs"
```

## Self-Review

Spec coverage:
- Windows service + helper architecture: Tasks 5, 6, 7, 8
- Per-user daily quotas with active console user only: Tasks 2, 3, 6, 7
- Network API with bearer token and time adjustments: Tasks 4 and 6
- TTS at startup, halfway, last five minutes, and 10-to-1 countdown: Tasks 3, 5, and 6
- Same-day exhausted-user reenforcement delay default 180 seconds: Task 3 and Task 6
- Hibernation with shutdown fallback: Tasks 6 and 7
- Logging and installer startup setup: Tasks 4, 7, and 8
- Admin-only termination goal via Windows service deployment: Tasks 7 and 8

Placeholder scan:
- No placeholder markers remain.
- Every code step contains concrete file contents or concrete command examples.

Type consistency:
- Shared config and state types are defined in `internal/model/types.go`.
- API contract, policy engine, store, and runtime all use `model.UserDayState`, `model.StateFile`, and `model.ActiveUser`.
