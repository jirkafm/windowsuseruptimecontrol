package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"windowsuseruptimecontrol/internal/model"
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
	err      error
}

func (f *fakeHelperBus) Speak(_ context.Context, userSID, message string) error {
	if f.err != nil {
		return f.err
	}
	f.messages = append(f.messages, userSID+":"+message)
	return nil
}

type fakePower struct {
	hibernateCalls int
	shutdownCalls  int
	onHibernate    func()
}

func (f *fakePower) Hibernate(context.Context) error {
	f.hibernateCalls++
	if f.onHibernate != nil {
		f.onHibernate()
	}
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
			WarningHalfwayEnabled:    true,
			WarningFiveMinEnabled:    true,
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

func TestTickSavesExhaustedStateBeforeHibernate(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		state: model.StateFile{
			ServiceDate: "2026-04-01",
			Users: map[string]model.UserDayState{
				"sid-john": {
					UserSID:            "sid-john",
					Username:           "John",
					Date:               "2026-04-01",
					DailyAllowanceSec:  3600,
					ConsumedSec:        3599,
					RemainingSec:       1,
					StartupWarningSent: true,
					HalfwayWarningSent: true,
					FiveMinWarningSent: true,
				},
			},
		},
	}
	power := &fakePower{
		onHibernate: func() {
			if !store.state.Users["sid-john"].Exhausted {
				t.Fatal("expected exhausted state to be saved before hibernate")
			}
		},
	}
	rt := Runtime{
		Config: model.Config{
			DefaultDailyAllowanceSec: 3600,
			ReenforcementDelaySec:    180,
		},
		Store:    store,
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, ok: true},
		Helper:   &fakeHelperBus{},
		Power:    power,
	}

	if err := rt.Tick(context.Background(), time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), 1); err != nil {
		t.Fatalf("Tick error: %v", err)
	}
}

func TestTickPersistsPolicyStateWhenHelperNotificationFails(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		state: model.StateFile{
			ServiceDate: "2026-04-01",
			Users: map[string]model.UserDayState{
				"sid-john": {
					UserSID:            "sid-john",
					Username:           "John",
					Date:               "2026-04-01",
					DailyAllowanceSec:  3600,
					ConsumedSec:        3599,
					RemainingSec:       1,
					StartupWarningSent: true,
					HalfwayWarningSent: true,
					FiveMinWarningSent: true,
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
		Helper:   &fakeHelperBus{err: errors.New("helper unavailable")},
		Power:    &fakePower{},
	}

	err := rt.Tick(context.Background(), time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), 1)
	if err == nil {
		t.Fatal("expected helper error")
	}
	if !store.state.Users["sid-john"].Exhausted {
		t.Fatal("expected exhausted state to be saved despite helper error")
	}
}

func TestTickStillHibernatesWhenCountdownNotificationFails(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		state: model.StateFile{
			ServiceDate: "2026-04-01",
			Users: map[string]model.UserDayState{
				"sid-john": {
					UserSID:            "sid-john",
					Username:           "John",
					Date:               "2026-04-01",
					DailyAllowanceSec:  3600,
					ConsumedSec:        3599,
					RemainingSec:       1,
					StartupWarningSent: true,
					HalfwayWarningSent: true,
					FiveMinWarningSent: true,
				},
			},
		},
	}
	power := &fakePower{}
	rt := Runtime{
		Config: model.Config{
			DefaultDailyAllowanceSec: 3600,
			ReenforcementDelaySec:    180,
		},
		Store:    store,
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, ok: true},
		Helper:   &fakeHelperBus{err: errors.New("helper unavailable")},
		Power:    power,
	}

	err := rt.Tick(context.Background(), time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), 1)
	if err == nil {
		t.Fatal("expected helper error to be reported")
	}
	if power.hibernateCalls != 1 {
		t.Fatalf("hibernateCalls = %d, want 1", power.hibernateCalls)
	}
}

type blockingStore struct {
	mu             sync.Mutex
	state          model.StateFile
	saveStarted    chan struct{}
	releaseSave    chan struct{}
	startOnce      sync.Once
	loadDuringSave bool
	saving         bool
}

func newBlockingStore(state model.StateFile) *blockingStore {
	return &blockingStore{
		state:       state,
		saveStarted: make(chan struct{}),
		releaseSave: make(chan struct{}),
	}
}

func (b *blockingStore) LoadOrCreate(time.Time, int64) (model.StateFile, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.saving {
		b.loadDuringSave = true
	}
	return b.state, nil
}

func (b *blockingStore) Save(state model.StateFile) error {
	b.mu.Lock()
	b.state = state
	b.saving = true
	b.mu.Unlock()

	b.startOnce.Do(func() { close(b.saveStarted) })
	<-b.releaseSave

	b.mu.Lock()
	b.saving = false
	b.mu.Unlock()
	return nil
}

func TestRuntimeSerializesTickAndUserMutation(t *testing.T) {
	t.Parallel()

	store := newBlockingStore(model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"sid-john": {
				UserSID:           "sid-john",
				Username:          "John",
				Date:              "2026-04-01",
				DailyAllowanceSec: 3600,
				ConsumedSec:       600,
				RemainingSec:      3000,
			},
		},
	})
	rt := Runtime{
		Config:   model.Config{DefaultDailyAllowanceSec: 3600, ReenforcementDelaySec: 180},
		Store:    store,
		Detector: fakeDetector{user: model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, ok: true},
		Helper:   &fakeHelperBus{},
		Power:    &fakePower{},
	}

	tickDone := make(chan error, 1)
	go func() {
		tickDone <- rt.Tick(context.Background(), time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), 1)
	}()

	<-store.saveStarted
	adjustDone := make(chan error, 1)
	go func() {
		_, err := rt.AdjustUser("john", 300)
		adjustDone <- err
	}()

	select {
	case err := <-adjustDone:
		if err != nil {
			t.Fatalf("AdjustUser error: %v", err)
		}
		t.Fatal("AdjustUser completed while Tick save was still in progress")
	case <-time.After(50 * time.Millisecond):
	}

	close(store.releaseSave)
	if err := <-tickDone; err != nil {
		t.Fatalf("Tick error: %v", err)
	}
	if err := <-adjustDone; err != nil {
		t.Fatalf("AdjustUser error: %v", err)
	}
	if store.loadDuringSave {
		t.Fatal("store was loaded by a second mutation while Tick was saving")
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

func TestConfigViewIncludesWarningToggles(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Config: model.Config{
			WarningHalfwayEnabled: true,
			WarningFiveMinEnabled: false,
		},
	}

	view := rt.ConfigView()
	if view["warning_halfway_enabled"] != true {
		t.Fatalf("warning_halfway_enabled = %#v, want true", view["warning_halfway_enabled"])
	}
	if view["warning_five_min_enabled"] != false {
		t.Fatalf("warning_five_min_enabled = %#v, want false", view["warning_five_min_enabled"])
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
