package helper

import (
	"strings"
	"testing"
	"time"
)

func TestShouldLaunchWhenNoActiveConnectionIsTracked(t *testing.T) {
	t.Parallel()

	if !shouldLaunch(time.Now(), 5) {
		t.Fatal("expected launch when no active connection is tracked")
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

	if shouldLaunchWithCooldown(now, 5, lastLaunch, cooldown) {
		t.Fatal("expected no relaunch while initial helper startup window is still active")
	}
}

func TestShouldLaunchAgainWhenStartupGraceExpiresWithoutConnection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	cooldown := 5 * time.Second
	lastLaunch := launchAttempt{
		SessionID: 5,
		StartedAt: now.Add(-cooldown - time.Second),
	}

	if !shouldLaunchWithCooldown(now, 5, lastLaunch, cooldown) {
		t.Fatal("expected relaunch after startup grace expires without connection")
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

func TestBuildCommandLineIncludesHelperConnectionArguments(t *testing.T) {
	t.Parallel()

	got := buildCommandLine(`C:\Program Files\Activity\activityhelper.exe`, 5, "http://127.0.0.1:8111/internal/helper/stream", "token-123")
	for _, want := range []string{
		`"C:\Program Files\Activity\activityhelper.exe"`,
		`--session-id 5`,
		`--helper-url "http://127.0.0.1:8111/internal/helper/stream"`,
		`--helper-token "token-123"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("command line %q missing %q", got, want)
		}
	}
}
