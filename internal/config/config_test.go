package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windowsuseruptimecontrol/internal/model"
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
	if cfg.LogMaxSizeMB != 10 {
		t.Fatalf("LogMaxSizeMB = %d, want 10", cfg.LogMaxSizeMB)
	}
	if cfg.LogMaxBackups != 10 {
		t.Fatalf("LogMaxBackups = %d, want 10", cfg.LogMaxBackups)
	}
	if cfg.LogMaxAgeDays != 365 {
		t.Fatalf("LogMaxAgeDays = %d, want 365", cfg.LogMaxAgeDays)
	}
	if !cfg.LogCompress {
		t.Fatalf("LogCompress = false, want true")
	}
	if cfg.BearerToken != "secret-token" {
		t.Fatalf("BearerToken = %q, want secret-token", cfg.BearerToken)
	}
}

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

func TestLoadConfigHonorsLogRotationOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := []byte(`{
		"api_port":8088,
		"bearer_token":"secret-token",
		"log_max_size_mb":25,
		"log_max_backups":3,
		"log_max_age_days":90,
		"log_compress":false
	}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.LogMaxSizeMB != 25 {
		t.Fatalf("LogMaxSizeMB = %d, want 25", cfg.LogMaxSizeMB)
	}
	if cfg.LogMaxBackups != 3 {
		t.Fatalf("LogMaxBackups = %d, want 3", cfg.LogMaxBackups)
	}
	if cfg.LogMaxAgeDays != 90 {
		t.Fatalf("LogMaxAgeDays = %d, want 90", cfg.LogMaxAgeDays)
	}
	if cfg.LogCompress {
		t.Fatalf("LogCompress = true, want false")
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
