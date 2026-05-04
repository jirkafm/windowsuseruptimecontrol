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
	if cfg.HelperLaunchCooldownSec == 0 {
		cfg.HelperLaunchCooldownSec = 5
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
}
