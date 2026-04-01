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
