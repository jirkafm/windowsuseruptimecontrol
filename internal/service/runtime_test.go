package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"wincontrol/internal/model"
)

type fakeStore struct {
	state model.StateFile
}

func (f *fakeStore) LoadOrCreate(time.Time, int64) (model.StateFile, error) { return f.state, nil }

func (f *fakeStore) Save(state model.StateFile) error {
	f.state = state
	return nil
}

type fakeDetector struct {
	user model.ActiveUser
	ok   bool
}

func (f fakeDetector) ActiveUser(context.Context) (model.ActiveUser, bool, error) {
	return f.user, f.ok, nil
}

type fakeHelperBus struct {
	messages []string
}

func (f *fakeHelperBus) Speak(_ context.Context, userSID, message string) error {
	f.messages = append(f.messages, userSID+":"+message)
	return nil
}

type fakePower struct {
	hibernateCalls int
	shutdownCalls  int
}

func (f *fakePower) Hibernate(context.Context) error {
	f.hibernateCalls++
	return nil
}

func (f *fakePower) Shutdown(context.Context) error {
	f.shutdownCalls++
	return nil
}

func TestTickConsumesTimeAndSpeaksPolicyMessages(t *testing.T) {
	t.Parallel()

	helper := &fakeHelperBus{}
	power := &fakePower{}
	rt := Runtime{
		Config: model.Config{
			DefaultDailyAllowanceSec: 3600,
			ReenforcementDelaySec:    180,
		},
		Store: &fakeStore{
			state: model.StateFile{
				ServiceDate: "2026-04-01",
				Users: map[string]model.UserDayState{
					"sid-john": {
						UserSID:              "sid-john",
						Username:             "John",
						Date:                 "2026-04-01",
						DailyAllowanceSec:    3600,
						ConsumedSec:          1790,
						RemainingSec:         1810,
						StartupWarningSent:   true,
						HalfwayWarningSent:   false,
						FiveMinWarningSent:   false,
						ReenforcementPending: false,
					},
				},
			},
		},
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, ok: true},
		Helper:   helper,
		Power:    power,
	}

	err := rt.Tick(context.Background(), time.Date(2026, 4, 1, 10, 0, 10, 0, time.UTC), 20)
	if err != nil {
		t.Fatalf("Tick error: %v", err)
	}

	if len(helper.messages) != 1 {
		t.Fatalf("messages = %#v, want 1 halfway warning", helper.messages)
	}
	if power.hibernateCalls != 0 {
		t.Fatalf("hibernateCalls = %d, want 0", power.hibernateCalls)
	}
}

func TestTickRearmsExpiredReenforcementDelayOnFirstTick(t *testing.T) {
	t.Parallel()

	helper := &fakeHelperBus{}
	power := &fakePower{}
	store := &fakeStore{
		state: model.StateFile{
			ServiceDate: "2026-04-01",
			Users: map[string]model.UserDayState{
				"sid-john": {
					UserSID:               "sid-john",
					Username:              "John",
					Date:                  "2026-04-01",
					DailyAllowanceSec:     3600,
					ConsumedSec:           3600,
					RemainingSec:          0,
					Exhausted:             true,
					ReenforcementPending:  true,
					ReenforcementDeadline: time.Date(2026, 4, 1, 7, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	rt := Runtime{
		Config: model.Config{
			DefaultDailyAllowanceSec: 3600,
			ReenforcementDelaySec:    180,
		},
		Store:    store,
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, ok: true},
		Helper:   helper,
		Power:    power,
	}

	now := time.Date(2026, 4, 1, 8, 0, 0, 0, time.UTC)
	if err := rt.Tick(context.Background(), now, 1); err != nil {
		t.Fatalf("Tick error: %v", err)
	}

	if power.hibernateCalls != 0 {
		t.Fatalf("hibernateCalls = %d, want 0", power.hibernateCalls)
	}
	if len(helper.messages) != 1 {
		t.Fatalf("messages = %#v, want grace-period announcement", helper.messages)
	}

	user := store.state.Users["sid-john"]
	if !user.ReenforcementPending {
		t.Fatalf("ReenforcementPending = false, want true")
	}
	if got := user.ReenforcementDeadline.Sub(now); got != 180*time.Second {
		t.Fatalf("ReenforcementDeadline delta = %v, want 180s", got)
	}
}

func TestHibernateNowRunsCountdownThenHibernate(t *testing.T) {
	t.Parallel()

	helper := &fakeHelperBus{}
	power := &fakePower{}
	rt := Runtime{
		Config:   model.Config{},
		Store:    &fakeStore{state: model.StateFile{Users: map[string]model.UserDayState{}}},
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, ok: true},
		Helper:   helper,
		Power:    power,
	}

	if err := rt.HibernateNow(); err != nil {
		t.Fatalf("HibernateNow error: %v", err)
	}

	if power.hibernateCalls != 1 {
		t.Fatalf("hibernateCalls = %d, want 1", power.hibernateCalls)
	}
	if len(helper.messages) != 10 {
		t.Fatalf("countdown messages = %d, want 10", len(helper.messages))
	}
}

func TestAdjustUserResolvesSimpleUsernameFromQualifiedUsername(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		state: model.StateFile{
			ServiceDate: "2026-04-02",
			Users: map[string]model.UserDayState{
				"S-1-5-21-john": {
					UserSID:           "S-1-5-21-john",
					Username:          "MYPC\\john",
					Date:              "2026-04-02",
					DailyAllowanceSec: 3600,
					ConsumedSec:       600,
					RemainingSec:      3000,
				},
			},
		},
	}
	rt := Runtime{
		Config: model.Config{DefaultDailyAllowanceSec: 3600},
		Store:  store,
	}

	user, err := rt.AdjustUser("john", 300)
	if err != nil {
		t.Fatalf("AdjustUser error: %v", err)
	}
	if user.UserSID != "S-1-5-21-john" {
		t.Fatalf("UserSID = %q, want S-1-5-21-john", user.UserSID)
	}
	if store.state.Users["S-1-5-21-john"].ConsumedSec != 300 {
		t.Fatalf("ConsumedSec = %d, want 300", store.state.Users["S-1-5-21-john"].ConsumedSec)
	}
}

func TestLookupUserResolvesExactUsername(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Config: model.Config{DefaultDailyAllowanceSec: 3600},
		Store: &fakeStore{
			state: model.StateFile{
				ServiceDate: "2026-04-02",
				Users: map[string]model.UserDayState{
					"S-1-5-21-jane": {
						UserSID:           "S-1-5-21-jane",
						Username:          "jane",
						Date:              "2026-04-02",
						DailyAllowanceSec: 3600,
						ConsumedSec:       120,
						RemainingSec:      3480,
					},
				},
			},
		},
	}

	user, err := rt.LookupUser("jane")
	if err != nil {
		t.Fatalf("LookupUser error: %v", err)
	}
	if user.UserSID != "S-1-5-21-jane" {
		t.Fatalf("UserSID = %q, want S-1-5-21-jane", user.UserSID)
	}
}

func TestLookupUserReturnsErrorForAmbiguousSimpleUsername(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Config: model.Config{DefaultDailyAllowanceSec: 3600},
		Store: &fakeStore{
			state: model.StateFile{
				ServiceDate: "2026-04-02",
				Users: map[string]model.UserDayState{
					"S-1-5-21-john-a": {
						UserSID:           "S-1-5-21-john-a",
						Username:          "MYPC\\john",
						Date:              "2026-04-02",
						DailyAllowanceSec: 3600,
					},
					"S-1-5-21-john-b": {
						UserSID:           "S-1-5-21-john-b",
						Username:          "WORK\\john",
						Date:              "2026-04-02",
						DailyAllowanceSec: 3600,
					},
				},
			},
		},
	}

	_, err := rt.LookupUser("john")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %v, want ambiguity message", err)
	}
}
