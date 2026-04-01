package helperstatus

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHeartbeatRoundTrip(t *testing.T) {
	t.Parallel()

	store := New(filepath.Join(t.TempDir(), "heartbeats"))
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	if err := store.Record("sid-john", Heartbeat{
		UserSID:   "sid-john",
		SessionID: 3,
		PID:       1234,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Record error: %v", err)
	}

	got, err := store.Load("sid-john")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.UserSID != "sid-john" || got.SessionID != 3 || got.PID != 1234 {
		t.Fatalf("heartbeat = %#v", got)
	}
	if !got.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, now)
	}
}

func TestIsFresh(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 12, 0, 30, 0, time.UTC)
	if !IsFresh(Heartbeat{UpdatedAt: now.Add(-10 * time.Second)}, now, 30*time.Second) {
		t.Fatal("expected heartbeat to be fresh")
	}
	if IsFresh(Heartbeat{UpdatedAt: now.Add(-31 * time.Second)}, now, 30*time.Second) {
		t.Fatal("expected heartbeat to be stale")
	}
}
