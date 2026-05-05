package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"

	"windowsuseruptimecontrol/internal/model"
)

var warningLogger = struct {
	sync.Mutex
	fn func(format string, args ...any)
}{}

func SetWarningLogger(fn func(format string, args ...any)) func() {
	warningLogger.Lock()
	previous := warningLogger.fn
	warningLogger.fn = fn
	warningLogger.Unlock()

	return func() {
		warningLogger.Lock()
		warningLogger.fn = previous
		warningLogger.Unlock()
	}
}

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
	if err := normalizeCustomConsumedWarnings(&cfg); err != nil {
		return model.Config{}, err
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

func normalizeCustomConsumedWarnings(cfg *model.Config) error {
	values := cfg.CustomConsumedWarningPercentages
	if len(values) > 10 {
		return fmt.Errorf("custom_consumed_warning_percentages supports a maximum of 10 items")
	}

	seen := make(map[int]bool, len(values))
	normalized := make([]int, 0, len(values))
	for _, value := range values {
		if value < 1 || value > 99 {
			return fmt.Errorf("custom_consumed_warning_percentages values must be from 1 to 99")
		}
		if value == 50 {
			warnf("custom consumed warning percentage 50 is ignored since it is implemented by default")
			continue
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	sort.Ints(normalized)
	cfg.CustomConsumedWarningPercentages = normalized
	return nil
}

func warnf(format string, args ...any) {
	warningLogger.Lock()
	defer warningLogger.Unlock()
	if warningLogger.fn != nil {
		warningLogger.fn(format, args...)
	}
}
