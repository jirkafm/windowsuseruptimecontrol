package config

import (
	"encoding/json"
	"fmt"
	"os"

	"windowsuseruptimecontrol/internal/model"
)

func Load(path string) (model.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := model.Config{
		WarningHalfwayEnabled: true,
		WarningFiveMinEnabled: true,
		LogCompress:           true,
	}
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

	return cfg, nil
}

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
