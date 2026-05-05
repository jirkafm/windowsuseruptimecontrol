package config

import (
	"os"
	"path/filepath"
	"strings"
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
	if cfg.HelperLaunchCooldownSec != 5 {
		t.Fatalf("HelperLaunchCooldownSec = %d, want 5", cfg.HelperLaunchCooldownSec)
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

func TestLoadConfigHonorsHelperLaunchCooldownOverride(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{"api_port":8088,"bearer_token":"secret-token","helper_launch_cooldown_sec":9}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.HelperLaunchCooldownSec != 9 {
		t.Fatalf("HelperLaunchCooldownSec = %d, want 9", cfg.HelperLaunchCooldownSec)
	}
}

func TestLoadConfigHonorsDisabledWarnings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{
		"api_port":8088,
		"bearer_token":"secret-token",
		"warning_halfway_enabled":false,
		"warning_five_min_enabled":false
	}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.WarningHalfwayEnabled {
		t.Fatalf("WarningHalfwayEnabled = true, want false")
	}
	if cfg.WarningFiveMinEnabled {
		t.Fatalf("WarningFiveMinEnabled = true, want false")
	}
}

func TestLoadConfigCustomConsumedWarningsNormalizesValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{
		"api_port":8088,
		"bearer_token":"secret-token",
		"custom_consumed_warning_percentages":[66,25,25,50,1,99]
	}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var warnings []string
	restore := SetWarningLogger(func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	defer restore()

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []int{1, 25, 66, 99}
	if len(cfg.CustomConsumedWarningPercentages) != len(want) {
		t.Fatalf("CustomConsumedWarningPercentages = %#v, want %#v", cfg.CustomConsumedWarningPercentages, want)
	}
	for idx := range want {
		if cfg.CustomConsumedWarningPercentages[idx] != want[idx] {
			t.Fatalf("CustomConsumedWarningPercentages = %#v, want %#v", cfg.CustomConsumedWarningPercentages, want)
		}
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "50") {
		t.Fatalf("warnings = %#v, want ignored 50 warning", warnings)
	}
}

func TestLoadConfigRejectsTooManyCustomConsumedWarnings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{
		"api_port":8088,
		"bearer_token":"secret-token",
		"custom_consumed_warning_percentages":[1,2,3,4,5,6,7,8,9,10,11]
	}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "maximum of 10") {
		t.Fatalf("Load error = %v, want maximum of 10 error", err)
	}
}

func TestLoadConfigRejectsOutOfRangeCustomConsumedWarning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{
		"api_port":8088,
		"bearer_token":"secret-token",
		"custom_consumed_warning_percentages":[25,100]
	}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "1 to 99") {
		t.Fatalf("Load error = %v, want range error", err)
	}
}
