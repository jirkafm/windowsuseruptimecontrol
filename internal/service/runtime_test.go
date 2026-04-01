package service

import (
	"context"
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
