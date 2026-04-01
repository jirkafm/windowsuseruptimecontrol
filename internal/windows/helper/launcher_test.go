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
