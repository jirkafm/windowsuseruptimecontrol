package model

import "time"

type QuotaMode string

const (
	QuotaModeDaily      QuotaMode = "daily"
	QuotaModeWeeklyFlex QuotaMode = "weekly-flex"
)

type Config struct {
	APIBindAddress            string    `json:"api_bind_address"`
	APIPort                   int       `json:"api_port"`
	BearerToken               string    `json:"bearer_token"`
	QuotaMode                 QuotaMode `json:"quota_mode"`
	DefaultDailyAllowanceSec  int64     `json:"default_daily_allowance_sec"`
	DefaultWeeklyAllowanceSec int64     `json:"default_weekly_allowance_sec"`
	UserUIEnabled             bool      `json:"user_ui_enabled"`
	UserUIPort                int       `json:"user_ui_port"`
	ReenforcementDelaySec     int64     `json:"reenforcement_delay_sec"`
	HelperLaunchCooldownSec   int64     `json:"helper_launch_cooldown_sec"`
	WarningHalfwayEnabled     bool      `json:"warning_halfway_enabled"`
	WarningFiveMinEnabled     bool      `json:"warning_five_min_enabled"`
	HelperPath                string    `json:"helper_path"`
	LogLevel                  string    `json:"log_level"`
	LogMaxSizeMB              int       `json:"log_max_size_mb"`
	LogMaxBackups             int       `json:"log_max_backups"`
	LogMaxAgeDays             int       `json:"log_max_age_days"`
	LogCompress               bool      `json:"log_compress"`
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

type StateFile struct {
	ServiceDate string                     `json:"service_date"`
	Users       map[string]UserDayState    `json:"users"`
	WeeklyUsers map[string]WeeklyUserState `json:"weekly_users,omitempty"`
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
