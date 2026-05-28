package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"windowsuseruptimecontrol/internal/model"
)

func TestLoadOrCreateResetsUsersOnNewDay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	raw := []byte(`{
		"service_date":"2026-03-31",
		"users":{
			"S-1-5-21-john":{
				"user_sid":"S-1-5-21-john",
				"username":"John",
				"date":"2026-03-31",
				"daily_allowance_sec":3600,
				"consumed_sec":3550,
				"remaining_sec":50,
				"exhausted":true
			}
		}
	}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	store := NewJSONStore(path)
	got, err := store.LoadOrCreate(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC), 3600)
	if err != nil {
		t.Fatalf("LoadOrCreate error: %v", err)
	}

	user := got.Users["S-1-5-21-john"]
	if user.Date != "2026-04-01" {
		t.Fatalf("Date = %q, want 2026-04-01", user.Date)
	}
	if user.ConsumedSec != 0 {
		t.Fatalf("ConsumedSec = %d, want 0", user.ConsumedSec)
	}
	if user.RemainingSec != 3600 {
		t.Fatalf("RemainingSec = %d, want 3600", user.RemainingSec)
	}
	if user.Exhausted {
		t.Fatalf("Exhausted = true, want false")
	}
}

func TestLoadOrCreateRenamesCorruptState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}

	store := NewJSONStore(path)
	got, err := store.LoadOrCreate(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC), 3600)
	if err != nil {
		t.Fatalf("LoadOrCreate error: %v", err)
	}
	if got.ServiceDate != "2026-04-01" {
		t.Fatalf("ServiceDate = %q, want 2026-04-01", got.ServiceDate)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "state.json.corrupt-*"))
	if err != nil {
		t.Fatalf("Glob error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("corrupt backups = %d, want 1", len(matches))
	}
}

func TestLoadOrCreateInitializesWeeklyUsersMap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	raw := []byte(`{"service_date":"2026-05-12","users":{}}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	store := NewJSONStore(path)
	got, err := store.LoadOrCreate(time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC), 3600)
	if err != nil {
		t.Fatalf("LoadOrCreate error: %v", err)
	}
	if got.WeeklyUsers == nil {
		t.Fatal("WeeklyUsers = nil, want initialized map")
	}
}

func TestUpsertUserPersistsCalculatedRemainingTime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	store := NewJSONStore(path)
	state := model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"S-1-5-21-mary": {
				UserSID:           "S-1-5-21-mary",
				Username:          "Mary",
				Date:              "2026-04-01",
				DailyAllowanceSec: 5400,
				ConsumedSec:       1200,
				RemainingSec:      4200,
			},
		},
	}

	if err := store.Save(state); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	reloaded, err := store.LoadOrCreate(time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC), 3600)
	if err != nil {
		t.Fatalf("LoadOrCreate error: %v", err)
	}

	if reloaded.Users["S-1-5-21-mary"].RemainingSec != 4200 {
		t.Fatalf("RemainingSec = %d, want 4200", reloaded.Users["S-1-5-21-mary"].RemainingSec)
	}
}
