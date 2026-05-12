# Weekly Flex Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in weekly-flex quota mode where admins own a 7-hour default weekly allowance and the active logged-in user can redistribute that weekly time across Monday-Sunday through a loopback-only web UI.

**Architecture:** Keep daily mode unchanged and branch runtime evaluation by `Config.QuotaMode`. Add a focused `internal/weekly` package for week math, distribution validation, policy evaluation, and UI view shaping. Serve admin API as today; when weekly-flex UI is enabled, serve user pages and JSON endpoints bound to `127.0.0.1` and scoped only to the active console user.

**Tech Stack:** Go 1.25 standard library, `net/http`, embedded HTML/CSS/JS via `embed`, existing JSON state store, existing Windows session detector and helper/power enforcement path, PowerShell installer scripts.

---

## File Structure

- Modify `internal/model/types.go`: add quota mode/config fields, weekly state structs, and recalculation helpers.
- Modify `internal/config/config.go` and `internal/config/config_test.go`: parse/default weekly-flex config, including 7-hour weekly default.
- Create `internal/weekly/week.go` and `internal/weekly/week_test.go`: Monday week start, weekday index, default distribution.
- Create `internal/weekly/policy.go` and `internal/weekly/policy_test.go`: weekly state normalization, allocation validation, tick evaluation, warning/enforcement output.
- Modify `internal/service/runtime.go` and `internal/service/runtime_test.go`: branch `Tick` by mode; add active-user weekly status, distribution update, admin weekly allowance/reset methods.
- Modify `internal/state/store.go` and `internal/state/store_test.go`: preserve `weekly_users` map during load/save and initialize nil maps.
- Modify `internal/api/server.go` and `internal/api/server_test.go`: add admin weekly endpoints and local user UI routes.
- Create `internal/api/user_ui.go`, `internal/api/user_ui_test.go`, `internal/api/assets/user/index.html`, `internal/api/assets/user/app.js`, `internal/api/assets/user/styles.css`: embedded dashboard-first UI.
- Modify `internal/runtime/app.go` and `internal/runtime/app_test.go`: mode startup option parsing, user UI server bind selection, lifecycle shutdown.
- Modify `installer/install.ps1` and `installer/install.Tests.ps1`: add install parameters for quota mode, weekly allowance, and local UI port.
- Modify `README.md` and `docs/windows-validation-checklist.md`: document weekly-flex operation and validation.

## Task 1: Config And Model Fields

**Files:**
- Modify: `internal/model/types.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing config/model tests**

Add these tests to `internal/config/config_test.go`:

```go
func TestLoadConfigDefaultsWeeklyFlexFields(t *testing.T) {
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

	if cfg.QuotaMode != model.QuotaModeDaily {
		t.Fatalf("QuotaMode = %q, want %q", cfg.QuotaMode, model.QuotaModeDaily)
	}
	if cfg.DefaultWeeklyAllowanceSec != 25200 {
		t.Fatalf("DefaultWeeklyAllowanceSec = %d, want 25200", cfg.DefaultWeeklyAllowanceSec)
	}
	if cfg.UserUIEnabled {
		t.Fatalf("UserUIEnabled = true, want false for daily mode")
	}
	if cfg.UserUIPort != 0 {
		t.Fatalf("UserUIPort = %d, want 0", cfg.UserUIPort)
	}
}

func TestLoadConfigEnablesUserUIByDefaultForWeeklyFlex(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{"api_port":8088,"bearer_token":"secret-token","quota_mode":"weekly-flex"}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.QuotaMode != model.QuotaModeWeeklyFlex {
		t.Fatalf("QuotaMode = %q, want weekly-flex", cfg.QuotaMode)
	}
	if !cfg.UserUIEnabled {
		t.Fatalf("UserUIEnabled = false, want true for weekly-flex")
	}
}

func TestLoadConfigRejectsUnknownQuotaMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{"api_port":8088,"bearer_token":"secret-token","quota_mode":"monthly"}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected unknown quota mode to fail")
	}
	if !strings.Contains(err.Error(), "quota_mode") {
		t.Fatalf("error = %v, want quota_mode validation", err)
	}
}
```

Update imports to include:

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windowsuseruptimecontrol/internal/model"
)
```

- [ ] **Step 2: Run config tests to verify they fail**

Run:

```bash
go test ./internal/config
```

Expected: FAIL because `model.QuotaModeDaily`, `model.QuotaModeWeeklyFlex`, `Config.QuotaMode`, `Config.DefaultWeeklyAllowanceSec`, `Config.UserUIEnabled`, and `Config.UserUIPort` do not exist.

- [ ] **Step 3: Add model fields and constants**

In `internal/model/types.go`, add before `type Config struct`:

```go
type QuotaMode string

const (
	QuotaModeDaily      QuotaMode = "daily"
	QuotaModeWeeklyFlex QuotaMode = "weekly-flex"
)
```

Extend `Config`:

```go
type Config struct {
	APIBindAddress           string    `json:"api_bind_address"`
	APIPort                  int       `json:"api_port"`
	BearerToken              string    `json:"bearer_token"`
	QuotaMode                QuotaMode `json:"quota_mode"`
	DefaultDailyAllowanceSec int64     `json:"default_daily_allowance_sec"`
	DefaultWeeklyAllowanceSec int64    `json:"default_weekly_allowance_sec"`
	UserUIEnabled            bool      `json:"user_ui_enabled"`
	UserUIPort               int       `json:"user_ui_port"`
	ReenforcementDelaySec    int64     `json:"reenforcement_delay_sec"`
	HelperLaunchCooldownSec  int64     `json:"helper_launch_cooldown_sec"`
	WarningHalfwayEnabled    bool      `json:"warning_halfway_enabled"`
	WarningFiveMinEnabled    bool      `json:"warning_five_min_enabled"`
	HelperPath               string    `json:"helper_path"`
	LogLevel                 string    `json:"log_level"`
	LogMaxSizeMB             int       `json:"log_max_size_mb"`
	LogMaxBackups            int       `json:"log_max_backups"`
	LogMaxAgeDays            int       `json:"log_max_age_days"`
	LogCompress              bool      `json:"log_compress"`
}
```

Run `gofmt` after adding fields; align spacing automatically.

- [ ] **Step 4: Implement config defaults and validation**

In `internal/config/config.go`, update `applyDefaults`:

```go
func applyDefaults(cfg *model.Config) {
	if cfg.APIBindAddress == "" {
		cfg.APIBindAddress = "0.0.0.0"
	}
	if cfg.QuotaMode == "" {
		cfg.QuotaMode = model.QuotaModeDaily
	}
	if cfg.DefaultDailyAllowanceSec == 0 {
		cfg.DefaultDailyAllowanceSec = 3600
	}
	if cfg.DefaultWeeklyAllowanceSec == 0 {
		cfg.DefaultWeeklyAllowanceSec = 25200
	}
	if cfg.QuotaMode == model.QuotaModeWeeklyFlex {
		cfg.UserUIEnabled = true
	}
	if cfg.ReenforcementDelaySec == 0 {
		cfg.ReenforcementDelaySec = 180
	}
	if cfg.HelperLaunchCooldownSec == 0 {
		cfg.HelperLaunchCooldownSec = 5
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.LogMaxSizeMB == 0 {
		cfg.LogMaxSizeMB = 10
	}
	if cfg.LogMaxBackups == 0 {
		cfg.LogMaxBackups = 10
	}
	if cfg.LogMaxAgeDays == 0 {
		cfg.LogMaxAgeDays = 365
	}
}
```

In `Load`, after bearer token validation:

```go
	switch cfg.QuotaMode {
	case model.QuotaModeDaily, model.QuotaModeWeeklyFlex:
	default:
		return model.Config{}, fmt.Errorf("quota_mode must be %q or %q", model.QuotaModeDaily, model.QuotaModeWeeklyFlex)
	}
	if cfg.DefaultWeeklyAllowanceSec <= 0 {
		return model.Config{}, fmt.Errorf("default_weekly_allowance_sec must be positive")
	}
	if cfg.UserUIPort < 0 || cfg.UserUIPort > 65535 {
		return model.Config{}, fmt.Errorf("user_ui_port must be between 0 and 65535")
	}
```

- [ ] **Step 5: Run config tests to verify they pass**

Run:

```bash
go test ./internal/config
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add internal/model/types.go internal/config/config.go internal/config/config_test.go
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "feat: add weekly flex config"
```

## Task 2: Weekly State Types And Week Helpers

**Files:**
- Modify: `internal/model/types.go`
- Create: `internal/weekly/week.go`
- Create: `internal/weekly/week_test.go`
- Modify: `internal/state/store.go`
- Modify: `internal/state/store_test.go`

- [ ] **Step 1: Write failing week helper tests**

Create `internal/weekly/week_test.go`:

```go
package weekly

import (
	"testing"
	"time"
)

func TestWeekStartReturnsMonday(t *testing.T) {
	t.Parallel()

	got := WeekStart(time.Date(2026, 5, 17, 18, 0, 0, 0, time.UTC))
	want := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("WeekStart = %s, want %s", got, want)
	}
}

func TestWeekdayIndexUsesMondayAsZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		date time.Time
		want int
	}{
		{"monday", time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC), 0},
		{"sunday", time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC), 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WeekdayIndex(tc.date); got != tc.want {
				t.Fatalf("WeekdayIndex = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDefaultDistributionSumsToAllowanceAndUsesFifteenMinuteSlots(t *testing.T) {
	t.Parallel()

	got := DefaultDistribution(25200)
	var sum int64
	for idx, value := range got {
		if value%900 != 0 {
			t.Fatalf("allocation[%d] = %d, want 15-minute increment", idx, value)
		}
		if value > 12600 {
			t.Fatalf("allocation[%d] = %d, want <= 50%% weekly allowance", idx, value)
		}
		sum += value
	}
	if sum != 25200 {
		t.Fatalf("sum = %d, want 25200", sum)
	}
}
```

- [ ] **Step 2: Run weekly tests to verify they fail**

Run:

```bash
go test ./internal/weekly
```

Expected: FAIL because the package and functions do not exist.

- [ ] **Step 3: Add weekly state structs**

In `internal/model/types.go`, add:

```go
type WeeklyUserState struct {
	UserSID               string    `json:"user_sid"`
	Username              string    `json:"username"`
	WeekStart             string    `json:"week_start"`
	WeeklyAllowanceSec    int64     `json:"weekly_allowance_sec"`
	AllocationsSec        [7]int64  `json:"allocations_sec"`
	ConsumedSec           [7]int64  `json:"consumed_sec"`
	RemainingSec          int64     `json:"remaining_sec"`
	Exhausted             bool      `json:"exhausted"`
	DayExhausted          bool      `json:"day_exhausted"`
	StartupWarningSent    bool      `json:"startup_warning_sent"`
	HalfwayWarningSent    bool      `json:"halfway_warning_sent"`
	FiveMinWarningSent    bool      `json:"five_min_warning_sent"`
	ReenforcementPending  bool      `json:"reenforcement_pending"`
	ReenforcementDeadline time.Time `json:"reenforcement_deadline"`
	LastEnforcementReason string    `json:"last_enforcement_reason"`
}

func (u *WeeklyUserState) WeeklyConsumedSec() int64 {
	var total int64
	for _, value := range u.ConsumedSec {
		total += value
	}
	return total
}

func (u *WeeklyUserState) RecalculateWeeklyRemaining() {
	u.RemainingSec = u.WeeklyAllowanceSec - u.WeeklyConsumedSec()
	if u.RemainingSec < 0 {
		u.RemainingSec = 0
	}
}
```

Extend `StateFile`:

```go
type StateFile struct {
	ServiceDate string                     `json:"service_date"`
	Users       map[string]UserDayState    `json:"users"`
	WeeklyUsers map[string]WeeklyUserState `json:"weekly_users,omitempty"`
}
```

- [ ] **Step 4: Add week helper implementation**

Create `internal/weekly/week.go`:

```go
package weekly

import "time"

const SlotSec int64 = 900

func WeekStart(now time.Time) time.Time {
	y, m, d := now.Date()
	loc := now.Location()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, loc)
	offset := (int(midnight.Weekday()) + 6) % 7
	return midnight.AddDate(0, 0, -offset)
}

func WeekdayIndex(now time.Time) int {
	return (int(now.Weekday()) + 6) % 7
}

func DefaultDistribution(allowanceSec int64) [7]int64 {
	var result [7]int64
	if allowanceSec <= 0 {
		return result
	}
	slots := allowanceSec / SlotSec
	for i := int64(0); i < slots; i++ {
		result[int(i%7)] += SlotSec
	}
	return result
}
```

- [ ] **Step 5: Preserve weekly map in state store**

Add this test to `internal/state/store_test.go`:

```go
func TestLoadOrCreateInitializesWeeklyUsersMap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	raw := []byte(`{"service_date":"2026-05-12","users":{}}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	store := NewJSONStore(path)
	got, err := store.LoadOrCreate(time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC), 3600)
	if err != nil {
		t.Fatalf("LoadOrCreate error: %v", err)
	}
	if got.WeeklyUsers == nil {
		t.Fatal("WeeklyUsers = nil, want initialized map")
	}
}
```

In `internal/state/store.go`, after `state.Users` initialization:

```go
	if state.WeeklyUsers == nil {
		state.WeeklyUsers = map[string]model.WeeklyUserState{}
	}
```

For new or corrupt state returns, include `WeeklyUsers: map[string]model.WeeklyUserState{}`.

- [ ] **Step 6: Run helper and state tests**

Run:

```bash
go test ./internal/weekly ./internal/state
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add internal/model/types.go internal/weekly/week.go internal/weekly/week_test.go internal/state/store.go internal/state/store_test.go
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "feat: add weekly quota state helpers"
```

## Task 3: Weekly Policy Evaluation And Distribution Validation

**Files:**
- Create: `internal/weekly/policy.go`
- Create: `internal/weekly/policy_test.go`
- Modify: `internal/model/types.go`

- [ ] **Step 1: Write failing weekly policy tests**

Create `internal/weekly/policy_test.go`:

```go
package weekly

import (
	"strings"
	"testing"
	"time"

	"windowsuseruptimecontrol/internal/model"
)

func TestEvaluateCreatesWeeklyStateWithSevenHourDefault(t *testing.T) {
	t.Parallel()

	engine := Engine{DefaultWeeklyAllowanceSec: 25200, ReenforcementDelaySec: 180}
	result := engine.Evaluate(
		time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
		model.ActiveUser{Username: "John", UserSID: "sid-john"},
		model.StateFile{Users: map[string]model.UserDayState{}, WeeklyUsers: map[string]model.WeeklyUserState{}},
		1,
	)

	user := result.State.WeeklyUsers["sid-john"]
	if user.WeeklyAllowanceSec != 25200 {
		t.Fatalf("WeeklyAllowanceSec = %d, want 25200", user.WeeklyAllowanceSec)
	}
	if user.WeekStart != "2026-05-11" {
		t.Fatalf("WeekStart = %q, want 2026-05-11", user.WeekStart)
	}
	if user.WeeklyConsumedSec() != 1 {
		t.Fatalf("WeeklyConsumedSec = %d, want 1", user.WeeklyConsumedSec())
	}
}

func TestValidateDistributionRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	current := model.WeeklyUserState{
		WeeklyAllowanceSec: 25200,
		ConsumedSec:        [7]int64{1800, 0, 0, 0, 0, 0, 0},
	}

	cases := []struct {
		name string
		dist [7]int64
		want string
	}{
		{"wrong sum", [7]int64{900, 900, 900, 900, 900, 900, 900}, "sum"},
		{"not increment", [7]int64{1801, 3600, 3600, 3600, 3600, 3600, 5399}, "15-minute"},
		{"over day cap", [7]int64{13500, 1800, 1800, 1800, 1800, 1800, 2700}, "50%"},
		{"below consumed", [7]int64{900, 3600, 3600, 3600, 3600, 3600, 6300}, "already consumed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDistribution(current, tc.dist)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestEvaluateExhaustsCurrentDayBeforeWeek(t *testing.T) {
	t.Parallel()

	engine := Engine{DefaultWeeklyAllowanceSec: 25200, ReenforcementDelaySec: 180}
	state := model.StateFile{
		Users: map[string]model.UserDayState{},
		WeeklyUsers: map[string]model.WeeklyUserState{
			"sid-john": {
				UserSID:            "sid-john",
				Username:           "John",
				WeekStart:          "2026-05-11",
				WeeklyAllowanceSec: 25200,
				AllocationsSec:     [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600},
				ConsumedSec:        [7]int64{0, 3599, 0, 0, 0, 0, 0},
				StartupWarningSent: true,
			},
		},
	}

	result := engine.Evaluate(time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC), model.ActiveUser{Username: "John", UserSID: "sid-john"}, state, 1)

	user := result.State.WeeklyUsers["sid-john"]
	if !user.DayExhausted {
		t.Fatal("DayExhausted = false, want true")
	}
	if user.Exhausted {
		t.Fatal("Exhausted = true, want false because weekly time remains")
	}
	if !result.TriggerEnforcement {
		t.Fatal("TriggerEnforcement = false, want true for day exhaustion")
	}
}

func TestEvaluateExhaustsWholeWeek(t *testing.T) {
	t.Parallel()

	engine := Engine{DefaultWeeklyAllowanceSec: 25200, ReenforcementDelaySec: 180}
	state := model.StateFile{
		Users: map[string]model.UserDayState{},
		WeeklyUsers: map[string]model.WeeklyUserState{
			"sid-john": {
				UserSID:            "sid-john",
				Username:           "John",
				WeekStart:          "2026-05-11",
				WeeklyAllowanceSec: 25200,
				AllocationsSec:     [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600},
				ConsumedSec:        [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3599},
				StartupWarningSent: true,
			},
		},
	}

	result := engine.Evaluate(time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC), model.ActiveUser{Username: "John", UserSID: "sid-john"}, state, 1)

	user := result.State.WeeklyUsers["sid-john"]
	if !user.Exhausted {
		t.Fatal("Exhausted = false, want true")
	}
	if user.LastEnforcementReason != "weekly allowance exhausted" {
		t.Fatalf("LastEnforcementReason = %q, want weekly allowance exhausted", user.LastEnforcementReason)
	}
}
```

- [ ] **Step 2: Run weekly policy tests to verify they fail**

Run:

```bash
go test ./internal/weekly
```

Expected: FAIL because `Engine`, `ValidateDistribution`, and weekly evaluation behavior do not exist.

- [ ] **Step 3: Add weekly policy implementation**

Create `internal/weekly/policy.go`:

```go
package weekly

import (
	"fmt"
	"time"

	"windowsuseruptimecontrol/internal/model"
)

type Engine struct {
	DefaultWeeklyAllowanceSec int64
	ReenforcementDelaySec    int64
	WarningHalfwayEnabled    bool
	WarningFiveMinEnabled    bool
}

func (e Engine) Evaluate(now time.Time, active model.ActiveUser, state model.StateFile, elapsedSec int64) model.Evaluation {
	if state.WeeklyUsers == nil {
		state.WeeklyUsers = map[string]model.WeeklyUserState{}
	}

	user := NormalizeUser(now, active, state.WeeklyUsers[active.UserSID], e.DefaultWeeklyAllowanceSec)
	result := model.Evaluation{State: state}

	if user.Exhausted || user.DayExhausted {
		if !user.ReenforcementPending {
			user.ReenforcementPending = true
			user.ReenforcementDeadline = now.Add(time.Duration(e.ReenforcementDelaySec) * time.Second)
			result.Messages = append(result.Messages, fmt.Sprintf("No time remains. The computer will hibernate in %d seconds.", e.ReenforcementDelaySec))
		} else if !user.ReenforcementDeadline.After(now) {
			result.TriggerEnforcement = true
			result.Countdown = countdown()
		}
		state.WeeklyUsers[active.UserSID] = user
		result.State = state
		return result
	}

	day := WeekdayIndex(now)
	user.ConsumedSec[day] += elapsedSec
	user.RecalculateWeeklyRemaining()

	if !user.StartupWarningSent {
		result.Messages = append(result.Messages, fmt.Sprintf("You have %d minutes remaining this week.", user.RemainingSec/60))
		user.StartupWarningSent = true
	}
	if e.WarningHalfwayEnabled && !user.HalfwayWarningSent && user.WeeklyConsumedSec() >= user.WeeklyAllowanceSec/2 {
		result.Messages = append(result.Messages, "You have used half of your weekly time.")
		user.HalfwayWarningSent = true
	}
	if e.WarningFiveMinEnabled && !user.FiveMinWarningSent && user.RemainingSec <= 300 {
		result.Messages = append(result.Messages, "You have 5 minutes remaining this week.")
		user.FiveMinWarningSent = true
	}

	if user.WeeklyConsumedSec() >= user.WeeklyAllowanceSec {
		user.Exhausted = true
		user.LastEnforcementReason = "weekly allowance exhausted"
		result.TriggerEnforcement = true
		result.Countdown = countdown()
	} else if user.ConsumedSec[day] >= user.AllocationsSec[day] {
		user.DayExhausted = true
		user.LastEnforcementReason = "daily allocation exhausted"
		result.TriggerEnforcement = true
		result.Countdown = countdown()
	}

	state.WeeklyUsers[active.UserSID] = user
	result.State = state
	return result
}

func NormalizeUser(now time.Time, active model.ActiveUser, user model.WeeklyUserState, defaultWeeklyAllowanceSec int64) model.WeeklyUserState {
	weekStart := WeekStart(now).Format("2006-01-02")
	if defaultWeeklyAllowanceSec <= 0 {
		defaultWeeklyAllowanceSec = 25200
	}
	if user.UserSID == "" {
		user = model.WeeklyUserState{
			UserSID:            active.UserSID,
			Username:           active.Username,
			WeekStart:          weekStart,
			WeeklyAllowanceSec: defaultWeeklyAllowanceSec,
			AllocationsSec:     DefaultDistribution(defaultWeeklyAllowanceSec),
		}
	}
	user.UserSID = active.UserSID
	user.Username = active.Username
	if user.WeeklyAllowanceSec == 0 {
		user.WeeklyAllowanceSec = defaultWeeklyAllowanceSec
	}
	if user.WeekStart != weekStart {
		user.WeekStart = weekStart
		user.ConsumedSec = [7]int64{}
		user.Exhausted = false
		user.DayExhausted = false
		user.StartupWarningSent = false
		user.HalfwayWarningSent = false
		user.FiveMinWarningSent = false
		user.ReenforcementPending = false
		user.ReenforcementDeadline = time.Time{}
		user.LastEnforcementReason = ""
	}
	if distributionSum(user.AllocationsSec) != user.WeeklyAllowanceSec {
		user.AllocationsSec = DefaultDistribution(user.WeeklyAllowanceSec)
	}
	user.RecalculateWeeklyRemaining()
	return user
}

func ValidateDistribution(current model.WeeklyUserState, dist [7]int64) error {
	if distributionSum(dist) != current.WeeklyAllowanceSec {
		return fmt.Errorf("distribution sum must equal weekly allowance")
	}
	capSec := current.WeeklyAllowanceSec / 2
	for idx, value := range dist {
		if value < 0 {
			return fmt.Errorf("day %d allocation cannot be negative", idx)
		}
		if value%SlotSec != 0 {
			return fmt.Errorf("day %d allocation must use 15-minute increments", idx)
		}
		if value > capSec {
			return fmt.Errorf("day %d allocation cannot exceed 50%% of weekly allowance", idx)
		}
		if value < current.ConsumedSec[idx] {
			return fmt.Errorf("day %d allocation cannot be lower than already consumed time", idx)
		}
	}
	return nil
}

func ApplyDistribution(now time.Time, current model.WeeklyUserState, dist [7]int64) (model.WeeklyUserState, error) {
	if err := ValidateDistribution(current, dist); err != nil {
		return model.WeeklyUserState{}, err
	}
	current.AllocationsSec = dist
	day := WeekdayIndex(now)
	if current.WeeklyConsumedSec() < current.WeeklyAllowanceSec && current.ConsumedSec[day] < current.AllocationsSec[day] {
		current.DayExhausted = false
		current.ReenforcementPending = false
		current.ReenforcementDeadline = time.Time{}
	}
	current.RecalculateWeeklyRemaining()
	return current, nil
}

func distributionSum(values [7]int64) int64 {
	var sum int64
	for _, value := range values {
		sum += value
	}
	return sum
}

func countdown() []string {
	return []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1"}
}
```

- [ ] **Step 4: Run weekly policy tests**

Run:

```bash
go test ./internal/weekly
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add internal/weekly/policy.go internal/weekly/policy_test.go internal/model/types.go
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "feat: evaluate weekly flex quotas"
```

## Task 4: Runtime Weekly Mode Integration

**Files:**
- Modify: `internal/service/runtime.go`
- Modify: `internal/service/runtime_test.go`

- [ ] **Step 1: Write failing runtime tests**

Add to `internal/service/runtime_test.go`:

```go
func TestTickUsesWeeklyFlexMode(t *testing.T) {
	t.Parallel()

	store := &fakeStore{state: model.StateFile{
		ServiceDate:  "2026-05-12",
		Users:        map[string]model.UserDayState{},
		WeeklyUsers:  map[string]model.WeeklyUserState{},
	}}
	rt := Runtime{
		Config: model.Config{
			QuotaMode:                 model.QuotaModeWeeklyFlex,
			DefaultWeeklyAllowanceSec: 25200,
			ReenforcementDelaySec:     180,
		},
		Store:    store,
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, ok: true},
		Helper:   &fakeHelperBus{},
		Power:    &fakePower{},
	}

	if err := rt.Tick(context.Background(), time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC), 1); err != nil {
		t.Fatalf("Tick error: %v", err)
	}

	if _, ok := store.state.WeeklyUsers["sid-john"]; !ok {
		t.Fatal("weekly state missing for active user")
	}
	if _, ok := store.state.Users["sid-john"]; ok {
		t.Fatal("daily state should not be created in weekly-flex mode")
	}
}

func TestUpdateActiveWeeklyDistributionOnlyAffectsActiveUser(t *testing.T) {
	t.Parallel()

	store := &fakeStore{state: model.StateFile{
		ServiceDate: "2026-05-12",
		Users:       map[string]model.UserDayState{},
		WeeklyUsers: map[string]model.WeeklyUserState{
			"sid-john": {
				UserSID:            "sid-john",
				Username:           "John",
				WeekStart:          "2026-05-11",
				WeeklyAllowanceSec: 25200,
				AllocationsSec:     [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600},
			},
			"sid-jane": {
				UserSID:            "sid-jane",
				Username:           "Jane",
				WeekStart:          "2026-05-11",
				WeeklyAllowanceSec: 25200,
				AllocationsSec:     [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600},
			},
		},
	}}
	rt := Runtime{
		Config:   model.Config{QuotaMode: model.QuotaModeWeeklyFlex, DefaultWeeklyAllowanceSec: 25200},
		Store:    store,
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, ok: true},
	}

	dist := [7]int64{4500, 4500, 3600, 3600, 3600, 2700, 2700}
	updated, err := rt.UpdateActiveWeeklyDistribution(context.Background(), time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC), dist)
	if err != nil {
		t.Fatalf("UpdateActiveWeeklyDistribution error: %v", err)
	}

	if updated.UserSID != "sid-john" {
		t.Fatalf("UserSID = %q, want sid-john", updated.UserSID)
	}
	if store.state.WeeklyUsers["sid-jane"].AllocationsSec[0] != 3600 {
		t.Fatal("inactive user's distribution changed")
	}
}
```

- [ ] **Step 2: Run runtime tests to verify they fail**

Run:

```bash
go test ./internal/service
```

Expected: FAIL because weekly runtime methods and mode branch do not exist.

- [ ] **Step 3: Add service interfaces and weekly methods**

In `internal/service/runtime.go`, import:

```go
	"windowsuseruptimecontrol/internal/weekly"
```

In `Tick`, replace the direct daily engine section with:

```go
	if r.Config.QuotaMode == model.QuotaModeWeeklyFlex {
		return r.tickWeekly(ctx, now, active, state, elapsedSec)
	}
	return r.tickDaily(ctx, now, active, state, elapsedSec)
```

Extract the current daily policy body into:

```go
func (r *Runtime) tickDaily(ctx context.Context, now time.Time, active model.ActiveUser, state model.StateFile, elapsedSec int64) error {
	// Move the current policy.Engine evaluation, save, logging, speaking, and enforcement body here unchanged.
}
```

Add weekly evaluator:

```go
func (r *Runtime) tickWeekly(ctx context.Context, now time.Time, active model.ActiveUser, state model.StateFile, elapsedSec int64) error {
	engine := weekly.Engine{
		DefaultWeeklyAllowanceSec: r.Config.DefaultWeeklyAllowanceSec,
		ReenforcementDelaySec:     r.Config.ReenforcementDelaySec,
		WarningHalfwayEnabled:     r.Config.WarningHalfwayEnabled,
		WarningFiveMinEnabled:     r.Config.WarningFiveMinEnabled,
	}
	beforeUser := state.WeeklyUsers[active.UserSID]
	result := engine.Evaluate(now, active, state, elapsedSec)
	afterUser := result.State.WeeklyUsers[active.UserSID]

	if err := r.Store.Save(result.State); err != nil {
		return err
	}

	if r.activeTrackingSID != active.UserSID {
		r.logf("weekly flex control started for user username=%s sid=%s session=%d allowance_sec=%d remaining_sec=%d", active.Username, active.UserSID, active.SessionID, afterUser.WeeklyAllowanceSec, afterUser.RemainingSec)
		r.activeTrackingSID = active.UserSID
	}
	if !beforeUser.Exhausted && afterUser.Exhausted {
		r.logf("weekly time depleted username=%s sid=%s consumed_sec=%d allowance_sec=%d reason=%q", afterUser.Username, afterUser.UserSID, afterUser.WeeklyConsumedSec(), afterUser.WeeklyAllowanceSec, afterUser.LastEnforcementReason)
	}

	return r.deliverEvaluation(ctx, active.UserSID, afterUser.Username, afterUser.RemainingSec, result)
}
```

Extract common message/enforcement delivery from daily code:

```go
func (r *Runtime) deliverEvaluation(ctx context.Context, userSID, username string, remainingSec int64, result model.Evaluation) error {
	var notifyErr error
	speak := func(message string) {
		if err := r.Helper.Speak(ctx, userSID, message); err != nil {
			notifyErr = errors.Join(notifyErr, err)
		}
	}
	for _, message := range result.Messages {
		r.logf("quota message issued username=%s sid=%s remaining_sec=%d message=%q", username, userSID, remainingSec, message)
		speak(message)
	}
	if result.TriggerEnforcement {
		for _, number := range result.Countdown {
			speak(number)
		}
		r.logf("enforcement hibernate requested username=%s sid=%s", username, userSID)
		if err := r.Power.Hibernate(ctx); err != nil {
			r.logf("hibernate failed; attempting shutdown username=%s sid=%s error=%v", username, userSID, err)
			if shutdownErr := r.Power.Shutdown(ctx); shutdownErr != nil {
				r.logf("shutdown fallback failed username=%s sid=%s error=%v", username, userSID, shutdownErr)
				return errors.Join(notifyErr, err, shutdownErr)
			}
			r.logf("shutdown fallback completed username=%s sid=%s", username, userSID)
		} else {
			r.logf("hibernate completed username=%s sid=%s", username, userSID)
		}
	}
	return notifyErr
}
```

Add active-user weekly mutation:

```go
func (r *Runtime) ActiveWeeklyStatus(ctx context.Context, now time.Time) (model.WeeklyUserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.Store.LoadOrCreate(now, r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	active, ok, err := r.Detector.ActiveUser(ctx)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	if !ok {
		return model.WeeklyUserState{}, fmt.Errorf("no active console user")
	}
	current := weekly.NormalizeUser(now, active, state.WeeklyUsers[active.UserSID], r.Config.DefaultWeeklyAllowanceSec)
	state.WeeklyUsers[active.UserSID] = current
	if err := r.Store.Save(state); err != nil {
		return model.WeeklyUserState{}, err
	}
	return current, nil
}

func (r *Runtime) UpdateActiveWeeklyDistribution(ctx context.Context, now time.Time, dist [7]int64) (model.WeeklyUserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.Store.LoadOrCreate(now, r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	active, ok, err := r.Detector.ActiveUser(ctx)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	if !ok {
		return model.WeeklyUserState{}, fmt.Errorf("no active console user")
	}
	current := weekly.NormalizeUser(now, active, state.WeeklyUsers[active.UserSID], r.Config.DefaultWeeklyAllowanceSec)
	updated, err := weekly.ApplyDistribution(now, current, dist)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	state.WeeklyUsers[active.UserSID] = updated
	return updated, r.Store.Save(state)
}
```

- [ ] **Step 4: Run runtime tests**

Run:

```bash
go test ./internal/service
```

Expected: PASS.

- [ ] **Step 5: Run policy and service tests together**

Run:

```bash
go test ./internal/policy ./internal/weekly ./internal/service
```

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add internal/service/runtime.go internal/service/runtime_test.go
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "feat: wire weekly flex runtime mode"
```

## Task 5: Local User UI API Routes

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/server_test.go`

- [ ] **Step 1: Write failing API tests**

Extend `fakeAdmin` in `internal/api/server_test.go`:

```go
func (f *fakeAdmin) ActiveWeeklyStatus(context.Context, time.Time) (model.WeeklyUserState, error) {
	return f.state.WeeklyUsers["sid-john"], nil
}

func (f *fakeAdmin) UpdateActiveWeeklyDistribution(_ context.Context, _ time.Time, dist [7]int64) (model.WeeklyUserState, error) {
	current := f.state.WeeklyUsers["sid-john"]
	current.AllocationsSec = dist
	f.state.WeeklyUsers["sid-john"] = current
	return current, nil
}
```

Add tests:

```go
func TestUserWeeklyStatusDoesNotRequireBearerTokenAndReturnsActiveUserOnly(t *testing.T) {
	t.Parallel()

	admin := &fakeAdmin{state: model.StateFile{WeeklyUsers: map[string]model.WeeklyUserState{
		"sid-john": {UserSID: "sid-john", Username: "John", WeekStart: "2026-05-11", WeeklyAllowanceSec: 25200},
		"sid-jane": {UserSID: "sid-jane", Username: "Jane", WeekStart: "2026-05-11", WeeklyAllowanceSec: 25200},
	}}}
	server := New("token-123", admin, fakeLogger{})
	req := httptest.NewRequest(http.MethodGet, "/user/api/status", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"user_sid":"sid-john"`) {
		t.Fatalf("response missing active user: %s", body)
	}
	if strings.Contains(body, "sid-jane") {
		t.Fatalf("response exposed inactive user: %s", body)
	}
}

func TestUserWeeklyDistributionRejectsBadJSON(t *testing.T) {
	t.Parallel()

	server := New("token-123", &fakeAdmin{}, fakeLogger{})
	req := httptest.NewRequest(http.MethodPost, "/user/api/distribution", strings.NewReader(`{"allocations_sec":[1,2]}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 2: Run API tests to verify they fail**

Run:

```bash
go test ./internal/api
```

Expected: FAIL because user weekly routes and admin interface methods do not exist.

- [ ] **Step 3: Extend API controller and routes**

In `internal/api/server.go`, extend `AdminController`:

```go
	ActiveWeeklyStatus(ctx context.Context, now time.Time) (model.WeeklyUserState, error)
	UpdateActiveWeeklyDistribution(ctx context.Context, now time.Time, dist [7]int64) (model.WeeklyUserState, error)
```

Add `time` import.

In `routes`, before `/v1` handlers:

```go
	s.mux.HandleFunc("/user/api/status", s.handleUserWeeklyStatus)
	s.mux.HandleFunc("/user/api/distribution", s.handleUserWeeklyDistribution)
```

Add handlers:

```go
func (s *Server) handleUserWeeklyStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.logRequest(r, http.StatusMethodNotAllowed)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, err := s.admin.ActiveWeeklyStatus(r.Context(), time.Now())
	if err != nil {
		s.logRequest(r, http.StatusServiceUnavailable)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	s.logRequest(r, http.StatusOK)
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleUserWeeklyDistribution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.logRequest(r, http.StatusMethodNotAllowed)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AllocationsSec []int64 `json:"allocations_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.AllocationsSec) != 7 {
		s.logRequest(r, http.StatusBadRequest)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	var dist [7]int64
	copy(dist[:], req.AllocationsSec)
	user, err := s.admin.UpdateActiveWeeklyDistribution(r.Context(), time.Now(), dist)
	if err != nil {
		s.logRequest(r, http.StatusBadRequest)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.logRequest(r, http.StatusOK)
	writeJSON(w, http.StatusOK, user)
}
```

- [ ] **Step 4: Run API tests**

Run:

```bash
go test ./internal/api
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add internal/api/server.go internal/api/server_test.go
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "feat: add active user weekly API"
```

## Task 6: Embedded Dashboard UI

**Files:**
- Create: `internal/api/user_ui.go`
- Create: `internal/api/user_ui_test.go`
- Create: `internal/api/assets/user/index.html`
- Create: `internal/api/assets/user/app.js`
- Create: `internal/api/assets/user/styles.css`
- Modify: `internal/api/server.go`

- [ ] **Step 1: Write failing UI route test**

Create `internal/api/user_ui_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserUIServesDashboardShell(t *testing.T) {
	t.Parallel()

	server := New("token-123", &fakeAdmin{}, fakeLogger{})
	req := httptest.NewRequest(http.MethodGet, "/user/", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Weekly time", "weekly-chart", "allocation-form"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q in %s", want, body)
		}
	}
}
```

- [ ] **Step 2: Run UI test to verify it fails**

Run:

```bash
go test ./internal/api
```

Expected: FAIL because `/user/` is not registered.

- [ ] **Step 3: Add embedded assets and route**

Create `internal/api/user_ui.go`:

```go
package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/user/*
var userUIAssets embed.FS

func userUIHandler() http.Handler {
	sub, err := fs.Sub(userUIAssets, "assets/user")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
```

In `routes`, add:

```go
	s.mux.Handle("/user/", http.StripPrefix("/user/", userUIHandler()))
```

Create `internal/api/assets/user/index.html`:

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Weekly time</title>
  <link rel="stylesheet" href="/user/styles.css">
</head>
<body>
  <main class="shell">
    <header class="topbar">
      <div>
        <h1>Weekly time</h1>
        <p id="week-range">Loading current week...</p>
      </div>
      <strong id="status-pill" class="pill">Loading</strong>
    </header>
    <section class="stats" aria-label="Weekly statistics">
      <article><span>Used</span><strong id="weekly-used">--</strong></article>
      <article><span>Remaining</span><strong id="weekly-remaining">--</strong></article>
      <article><span>Today used</span><strong id="today-used">--</strong></article>
      <article><span>Today left</span><strong id="today-remaining">--</strong></article>
    </section>
    <section class="panel">
      <h2>Consumption</h2>
      <div id="weekly-chart" class="chart" aria-label="Allocated and consumed time by day"></div>
    </section>
    <section class="panel">
      <h2>Distribution</h2>
      <form id="allocation-form"></form>
      <div class="actions">
        <p id="form-status" role="status"></p>
        <button id="save-button" type="button" disabled>Save distribution</button>
      </div>
    </section>
  </main>
  <script src="/user/app.js"></script>
</body>
</html>
```

Create `styles.css` and `app.js` with accessible native controls. Keep colors restrained and multi-hue: blue for allocation, teal for consumed, amber for warnings, neutral backgrounds.

- [ ] **Step 4: Run API tests**

Run:

```bash
go test ./internal/api
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add internal/api/user_ui.go internal/api/user_ui_test.go internal/api/assets/user internal/api/server.go
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "feat: add weekly flex user dashboard"
```

## Task 7: Runtime User UI Server Binding

**Files:**
- Modify: `internal/runtime/app.go`
- Modify: `internal/runtime/app_test.go`

- [ ] **Step 1: Write failing bind selection tests**

Add to `internal/runtime/app_test.go`:

```go
func TestUserUIAddrBindsLoopbackOnConfiguredPort(t *testing.T) {
	t.Parallel()

	got := userUIAddr(model.Config{APIBindAddress: "0.0.0.0", APIPort: 8111, UserUIPort: 8122})
	if got != "127.0.0.1:8122" {
		t.Fatalf("userUIAddr = %q, want 127.0.0.1:8122", got)
	}
}

func TestUserUIAddrReusesAPIPortOnlyForLoopbackAPI(t *testing.T) {
	t.Parallel()

	got := userUIAddr(model.Config{APIBindAddress: "127.0.0.1", APIPort: 8111})
	if got != "127.0.0.1:8111" {
		t.Fatalf("userUIAddr = %q, want 127.0.0.1:8111", got)
	}
}

func TestUserUIAddrChoosesDefaultWhenAPIIsWildcardAndPortUnset(t *testing.T) {
	t.Parallel()

	got := userUIAddr(model.Config{APIBindAddress: "0.0.0.0", APIPort: 8111})
	if got != "127.0.0.1:8112" {
		t.Fatalf("userUIAddr = %q, want 127.0.0.1:8112", got)
	}
}

func TestApplyServiceStartupArgsEnablesWeeklyFlexMode(t *testing.T) {
	t.Parallel()

	cfg := model.Config{QuotaMode: model.QuotaModeDaily, UserUIEnabled: false}
	got, err := applyServiceStartupArgs(cfg, []string{"activitysvc.exe", "--quota-mode", "weekly-flex"})
	if err != nil {
		t.Fatalf("applyServiceStartupArgs error: %v", err)
	}
	if got.QuotaMode != model.QuotaModeWeeklyFlex {
		t.Fatalf("QuotaMode = %q, want weekly-flex", got.QuotaMode)
	}
	if !got.UserUIEnabled {
		t.Fatal("UserUIEnabled = false, want true when weekly-flex is enabled by startup args")
	}
}

func TestApplyServiceStartupArgsRejectsUnknownQuotaMode(t *testing.T) {
	t.Parallel()

	_, err := applyServiceStartupArgs(model.Config{}, []string{"activitysvc.exe", "--quota-mode", "monthly"})
	if err == nil {
		t.Fatal("expected invalid quota mode to fail")
	}
	if !strings.Contains(err.Error(), "quota-mode") {
		t.Fatalf("error = %v, want quota-mode validation", err)
	}
}
```

- [ ] **Step 2: Run runtime tests to verify they fail**

Run:

```bash
go test ./internal/runtime
```

Expected: FAIL because `userUIAddr` does not exist.

- [ ] **Step 3: Add bind selection and server startup**

In `internal/runtime/app.go`, add:

```go
func userUIAddr(cfg model.Config) string {
	port := cfg.UserUIPort
	if port == 0 && isLoopbackBind(cfg.APIBindAddress) {
		port = cfg.APIPort
	}
	if port == 0 {
		port = cfg.APIPort + 1
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func isLoopbackBind(host string) bool {
	return host == "127.0.0.1" || host == "localhost" || host == "::1" || host == "[::1]"
}

func applyServiceStartupArgs(cfg model.Config, args []string) (model.Config, error) {
	for idx := 0; idx < len(args)-1; idx++ {
		switch args[idx] {
		case "--quota-mode":
			mode := model.QuotaMode(args[idx+1])
			switch mode {
			case model.QuotaModeDaily:
				cfg.QuotaMode = model.QuotaModeDaily
			case model.QuotaModeWeeklyFlex:
				cfg.QuotaMode = model.QuotaModeWeeklyFlex
				cfg.UserUIEnabled = true
			default:
				return model.Config{}, fmt.Errorf("quota-mode must be %q or %q", model.QuotaModeDaily, model.QuotaModeWeeklyFlex)
			}
		}
	}
	return cfg, nil
}
```

Before constructing runtime dependencies in `ServiceMain`, apply startup args after loading config:

```go
	cfg, err = applyServiceStartupArgs(cfg, os.Args)
	if err != nil {
		return err
	}
```

After the admin `httpServer` is constructed, start a separate user UI server only when the API itself is not loopback:

```go
	var userUIServer *http.Server
	if cfg.UserUIEnabled && cfg.QuotaMode == model.QuotaModeWeeklyFlex && !isLoopbackBind(cfg.APIBindAddress) {
		userUIServer = &http.Server{
			Addr:    userUIAddr(cfg),
			Handler: server,
		}
		logger.Servicef("user ui starting on %s", userUIServer.Addr)
		go func() {
			if err := userUIServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Servicef("user ui server error: %v", err)
			}
		}()
	}
```

On shutdown:

```go
			if userUIServer != nil {
				_ = userUIServer.Shutdown(shutdownCtx)
			}
```

If API bind is already loopback, the existing server handles `/user/` on the same loopback port.

- [ ] **Step 4: Run runtime tests**

Run:

```bash
go test ./internal/runtime
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add internal/runtime/app.go internal/runtime/app_test.go
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "feat: bind weekly user ui to loopback"
```

## Task 8: Admin Weekly Operations And Info Output

**Files:**
- Modify: `internal/service/runtime.go`
- Modify: `internal/service/runtime_test.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/server_test.go`

- [ ] **Step 1: Write failing admin weekly tests**

Add this service test to `internal/service/runtime_test.go`:

```go
func TestSetWeeklyAllowanceRejectsAllowanceBelowConsumed(t *testing.T) {
	t.Parallel()

	store := &fakeStore{state: model.StateFile{
		ServiceDate: "2026-05-12",
		Users:       map[string]model.UserDayState{},
		WeeklyUsers: map[string]model.WeeklyUserState{
			"sid-john": {
				UserSID:            "sid-john",
				Username:           "MYPC\\john",
				WeekStart:          "2026-05-11",
				WeeklyAllowanceSec: 25200,
				AllocationsSec:     [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600},
				ConsumedSec:        [7]int64{3600, 3600, 0, 0, 0, 0, 0},
			},
		},
	}}
	rt := Runtime{Config: model.Config{DefaultWeeklyAllowanceSec: 25200}, Store: store}

	_, err := rt.SetWeeklyAllowance("john", 3600)
	if err == nil {
		t.Fatal("expected weekly allowance below consumed time to fail")
	}
	if !strings.Contains(err.Error(), "already consumed") {
		t.Fatalf("error = %v, want already consumed", err)
	}
}

func TestResetWeekClearsWeeklyConsumption(t *testing.T) {
	t.Parallel()

	store := &fakeStore{state: model.StateFile{
		ServiceDate: "2026-05-12",
		Users:       map[string]model.UserDayState{},
		WeeklyUsers: map[string]model.WeeklyUserState{
			"sid-john": {
				UserSID:               "sid-john",
				Username:              "MYPC\\john",
				WeekStart:             "2026-05-11",
				WeeklyAllowanceSec:    25200,
				AllocationsSec:        [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600},
				ConsumedSec:           [7]int64{3600, 900, 0, 0, 0, 0, 0},
				RemainingSec:          20700,
				Exhausted:             true,
				DayExhausted:          true,
				ReenforcementPending:  true,
				ReenforcementDeadline: time.Date(2026, 5, 12, 12, 3, 0, 0, time.UTC),
			},
		},
	}}
	rt := Runtime{Config: model.Config{DefaultWeeklyAllowanceSec: 25200}, Store: store}

	user, err := rt.ResetWeek("john", time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ResetWeek error: %v", err)
	}
	if user.WeeklyConsumedSec() != 0 {
		t.Fatalf("WeeklyConsumedSec = %d, want 0", user.WeeklyConsumedSec())
	}
	if user.Exhausted || user.DayExhausted || user.ReenforcementPending {
		t.Fatalf("weekly flags not cleared: %#v", user)
	}
	if user.RemainingSec != 25200 {
		t.Fatalf("RemainingSec = %d, want 25200", user.RemainingSec)
	}
}
```

Extend `fakeAdmin` in `internal/api/server_test.go`:

```go
func (f *fakeAdmin) LookupWeeklyUser(user string, now time.Time) (model.WeeklyUserState, error) {
	got, ok := f.state.WeeklyUsers[user]
	if !ok {
		return model.WeeklyUserState{}, errors.New("not found")
	}
	return got, nil
}

func (f *fakeAdmin) SetWeeklyAllowance(user string, sec int64) (model.WeeklyUserState, error) {
	current := f.state.WeeklyUsers[user]
	current.WeeklyAllowanceSec = sec
	current.AllocationsSec = [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600}
	current.RecalculateWeeklyRemaining()
	f.state.WeeklyUsers[user] = current
	return current, nil
}

func (f *fakeAdmin) ResetWeek(user string, now time.Time) (model.WeeklyUserState, error) {
	current := f.state.WeeklyUsers[user]
	current.ConsumedSec = [7]int64{}
	current.Exhausted = false
	current.DayExhausted = false
	current.RecalculateWeeklyRemaining()
	f.state.WeeklyUsers[user] = current
	return current, nil
}
```

Add these API tests:

```go
func TestWeeklyStatusEndpointReturnsUserWeeklyState(t *testing.T) {
	t.Parallel()

	admin := &fakeAdmin{state: model.StateFile{WeeklyUsers: map[string]model.WeeklyUserState{
		"sid-john": {UserSID: "sid-john", Username: "John", WeekStart: "2026-05-11", WeeklyAllowanceSec: 25200},
	}}}
	server := New("token-123", admin, fakeLogger{})
	req := httptest.NewRequest(http.MethodGet, "/v1/users/sid-john/weekly-status", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"weekly_allowance_sec":25200`) {
		t.Fatalf("body = %s, want weekly allowance", rec.Body.String())
	}
}

func TestWeeklyAllowanceEndpointUpdatesUserWeeklyAllowance(t *testing.T) {
	t.Parallel()

	admin := &fakeAdmin{state: model.StateFile{WeeklyUsers: map[string]model.WeeklyUserState{
		"sid-john": {UserSID: "sid-john", Username: "John", WeekStart: "2026-05-11", WeeklyAllowanceSec: 25200},
	}}}
	server := New("token-123", admin, fakeLogger{})
	req := httptest.NewRequest(http.MethodPost, "/v1/users/sid-john/weekly-allowance", strings.NewReader(`{"weekly_allowance_sec":28800}`))
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if admin.state.WeeklyUsers["sid-john"].WeeklyAllowanceSec != 28800 {
		t.Fatalf("WeeklyAllowanceSec = %d, want 28800", admin.state.WeeklyUsers["sid-john"].WeeklyAllowanceSec)
	}
}

func TestResetWeekEndpointClearsWeeklyConsumption(t *testing.T) {
	t.Parallel()

	admin := &fakeAdmin{state: model.StateFile{WeeklyUsers: map[string]model.WeeklyUserState{
		"sid-john": {UserSID: "sid-john", Username: "John", WeekStart: "2026-05-11", WeeklyAllowanceSec: 25200, ConsumedSec: [7]int64{900, 0, 0, 0, 0, 0, 0}},
	}}}
	server := New("token-123", admin, fakeLogger{})
	req := httptest.NewRequest(http.MethodPost, "/v1/users/sid-john/reset-week", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if admin.state.WeeklyUsers["sid-john"].WeeklyConsumedSec() != 0 {
		t.Fatalf("WeeklyConsumedSec = %d, want 0", admin.state.WeeklyUsers["sid-john"].WeeklyConsumedSec())
	}
}
```

Expected initial failure: routes and service methods do not exist.

- [ ] **Step 2: Implement runtime methods**

Add to `internal/service/runtime.go`:

```go
func (r *Runtime) LookupWeeklyUser(user string, now time.Time) (model.WeeklyUserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.Store.LoadOrCreate(now, r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	key, err := resolveWeeklyUserKey(state, user)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	current := state.WeeklyUsers[key]
	current.RecalculateWeeklyRemaining()
	return current, nil
}
```

Implement `SetWeeklyAllowance` by changing `WeeklyAllowanceSec`, resetting `AllocationsSec` with `weekly.DefaultDistribution(sec)`, rejecting values below already consumed weekly total, recalculating remaining, saving state, and returning the user.

Implement `ResetWeek` by zeroing `ConsumedSec`, clearing weekly flags/deadline/reason, recalculating remaining, saving state, and returning the user.

- [ ] **Step 3: Implement API routes**

Extend `/v1/users/{userId}/...` switch with:

```go
case r.Method == http.MethodGet && action == "weekly-status":
case r.Method == http.MethodPost && action == "weekly-allowance":
case r.Method == http.MethodPost && action == "reset-week":
```

Use JSON request field `weekly_allowance_sec`.

Update `infoEndpoints()` with the three weekly endpoints.

- [ ] **Step 4: Run service and API tests**

Run:

```bash
go test ./internal/service ./internal/api
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add internal/service/runtime.go internal/service/runtime_test.go internal/api/server.go internal/api/server_test.go
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "feat: add weekly flex admin controls"
```

## Task 9: Installer And Documentation

**Files:**
- Modify: `installer/install.ps1`
- Modify: `installer/install.Tests.ps1`
- Modify: `README.md`
- Modify: `docs/windows-validation-checklist.md`

- [ ] **Step 1: Write failing installer tests**

Add expectations to `installer/install.Tests.ps1`:

```powershell
$script | Should -Match '\[ValidateSet\("daily","weekly-flex"\)\]'
$script | Should -Match '\$QuotaMode = "daily"'
$script | Should -Match 'default_weekly_allowance_sec = \$DefaultWeeklyAllowanceSec'
$script | Should -Match 'user_ui_port = \$UserUiPort'
```

- [ ] **Step 2: Update installer**

In `installer/install.ps1`, extend `param`:

```powershell
    [ValidateSet("daily","weekly-flex")]
    [string]$QuotaMode = "daily",
    [int]$DefaultWeeklyAllowanceSec = 25200,
    [int]$UserUiPort = 0,
    [bool]$UserUiEnabled = $false
```

In config JSON hashtable add:

```powershell
    quota_mode = $QuotaMode
    default_weekly_allowance_sec = $DefaultWeeklyAllowanceSec
    user_ui_enabled = ($UserUiEnabled -or $QuotaMode -eq "weekly-flex")
    user_ui_port = $UserUiPort
```

- [ ] **Step 3: Update docs**

In `README.md`, add a weekly-flex section:

```markdown
## Weekly Flex Mode

Daily mode remains the default. To enable weekly-flex mode, install with `-QuotaMode weekly-flex`. The default weekly allowance is 7 hours (`25200` seconds). Administrators control the total allowance. The logged-in user can open the local user UI on `127.0.0.1` and redistribute that allowance across Monday-Sunday in 15-minute increments. No day can receive more than 50% of the weekly allowance.
```

In `docs/windows-validation-checklist.md`, add manual checks for weekly-flex install, loopback UI, current-user-only data, 15-minute slider increments, 50% day cap, and next-Monday reset.

- [ ] **Step 4: Run installer and Go tests**

Run:

```bash
pwsh -NoProfile -File installer/install.Tests.ps1
go test ./...
```

Expected: PASS. If PowerShell is unavailable on the current machine, record that only Go tests were run and run PowerShell validation on Windows.

- [ ] **Step 5: Commit**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git add installer/install.ps1 installer/install.Tests.ps1 README.md docs/windows-validation-checklist.md
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git commit -m "docs: document weekly flex mode"
```

## Task 10: Full Verification

**Files:**
- No code changes expected unless verification exposes a defect.

- [ ] **Step 1: Run full Go test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run PowerShell installer tests**

Run:

```bash
pwsh -NoProfile -File installer/install.Tests.ps1
```

Expected: PASS.

- [ ] **Step 3: Build Windows binaries**

Run:

```bash
GOOS=windows GOARCH=amd64 go build -o dist/activitysvc.exe ./cmd/activitysvc
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui" -o dist/activityhelper.exe ./cmd/activityhelper
```

Expected: both commands exit 0 and produce binaries under `dist/`.

- [ ] **Step 4: Inspect final diff**

Run:

```bash
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git status --short
GIT_CONFIG_GLOBAL=/Users/jikaplan/.jirkafm/my-env/.gitconfig git log --oneline -10
```

Expected: only intentional untracked local artifacts remain, such as `.gocache/` if still present. Recent commits should match the task commits above.
