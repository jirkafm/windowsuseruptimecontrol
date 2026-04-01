package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"wincontrol/internal/model"
)

type fakeAdmin struct {
	lastAdjustUser  string
	lastAdjustDelta int64
	state           model.StateFile
}

type fakeLogger struct{}

func (fakeLogger) APIf(string, ...any) {}

func (fakeLogger) Recent(int) ([]string, error) { return []string{"line1"}, nil }

func (f *fakeAdmin) State() model.StateFile { return f.state }

func (f *fakeAdmin) ConfigView() map[string]any { return map[string]any{"api_port": 8080} }

func (f *fakeAdmin) AdjustUser(user string, delta int64) (model.UserDayState, error) {
	f.lastAdjustUser = user
	f.lastAdjustDelta = delta
	current := f.state.Users[user]
	current.ConsumedSec -= delta
	current.RecalculateRemaining()
	f.state.Users[user] = current
	return current, nil
}

func (f *fakeAdmin) SetAllowance(user string, sec int64) (model.UserDayState, error) {
	current := f.state.Users[user]
	current.DailyAllowanceSec = sec
	current.RecalculateRemaining()
	f.state.Users[user] = current
	return current, nil
}

func (f *fakeAdmin) ResetToday(user string) (model.UserDayState, error) {
	current := f.state.Users[user]
	current.ConsumedSec = 0
	current.RecalculateRemaining()
	f.state.Users[user] = current
	return current, nil
}

func (f *fakeAdmin) Announce(msg string) error { return nil }

func (f *fakeAdmin) HibernateNow() error { return nil }

func TestAdjustEndpointRequiresBearerToken(t *testing.T) {
	t.Parallel()

	server := New("token-123", &fakeAdmin{}, fakeLogger{})
	req := httptest.NewRequest(http.MethodPost, "/v1/users/sid-john/adjust", bytes.NewBufferString(`{"delta_sec":300}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAdjustEndpointAppliesDelta(t *testing.T) {
	t.Parallel()

	admin := &fakeAdmin{
		state: model.StateFile{
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
		},
	}
	server := New("token-123", admin, fakeLogger{})

	body, _ := json.Marshal(map[string]int64{"delta_sec": 300})
	req := httptest.NewRequest(http.MethodPost, "/v1/users/sid-john/adjust", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if admin.lastAdjustUser != "sid-john" {
		t.Fatalf("lastAdjustUser = %q, want sid-john", admin.lastAdjustUser)
	}
	if admin.lastAdjustDelta != 300 {
		t.Fatalf("lastAdjustDelta = %d, want 300", admin.lastAdjustDelta)
	}
}
