package helper

import (
	"testing"
	"time"

	"wincontrol/internal/helperstatus"
)

func TestShouldLaunchWhenHeartbeatMissing(t *testing.T) {
	t.Parallel()

	if !shouldLaunch(time.Now(), helperstatus.Heartbeat{}, 5) {
		t.Fatal("expected launch when heartbeat is missing")
	}
}

func TestShouldNotLaunchAgainWhenHelperWasJustStartedForSameSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	cooldown := 5 * time.Second
	lastLaunch := launchAttempt{
		SessionID: 5,
		StartedAt: now.Add(-2 * time.Second),
	}

	if shouldLaunchWithCooldown(now, helperstatus.Heartbeat{}, 5, lastLaunch, cooldown) {
		t.Fatal("expected no relaunch while initial helper startup window is still active")
	}
}

func TestShouldLaunchAgainWhenStartupGraceExpiresWithoutHeartbeat(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	cooldown := 5 * time.Second
	lastLaunch := launchAttempt{
		SessionID: 5,
		StartedAt: now.Add(-cooldown - time.Second),
	}

	if !shouldLaunchWithCooldown(now, helperstatus.Heartbeat{}, 5, lastLaunch, cooldown) {
		t.Fatal("expected relaunch after startup grace expires without heartbeat")
	}
}

func TestShouldNotLaunchWhenHeartbeatFreshForSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	hb := helperstatus.Heartbeat{
		UserSID:   "sid-john",
		SessionID: 5,
		UpdatedAt: now.Add(-10 * time.Second),
	}

	if shouldLaunch(now, hb, 5) {
		t.Fatal("expected no launch for fresh heartbeat in same session")
	}
}

func TestShouldLaunchWhenHeartbeatStale(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	hb := helperstatus.Heartbeat{
		UserSID:   "sid-john",
		SessionID: 5,
		UpdatedAt: now.Add(-31 * time.Second),
	}

	if !shouldLaunch(now, hb, 5) {
		t.Fatal("expected launch for stale heartbeat")
	}
}

func TestDefaultLaunchSettingsHideHelperWindow(t *testing.T) {
	t.Parallel()

	settings := defaultLaunchSettings()
	if !settings.HideWindow {
		t.Fatal("expected helper window to be hidden")
	}
	if !settings.NoConsoleWindow {
		t.Fatal("expected helper launch to suppress the console window")
	}
}
