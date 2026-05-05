package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"windowsuseruptimecontrol/internal/helperipc"
	"windowsuseruptimecontrol/internal/model"
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

func (f *fakeAdmin) LookupUser(user string) (model.UserDayState, error) {
	got, ok := f.state.Users[user]
	if !ok {
		return model.UserDayState{}, errors.New("not found")
	}
	return got, nil
}

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

func TestInfoEndpointRequiresBearerToken(t *testing.T) {
	t.Parallel()

	server := New("token-123", &fakeAdmin{}, fakeLogger{})
	req := httptest.NewRequest(http.MethodGet, "/v1/info", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestInfoEndpointListsKeyEndpointsAndExamples(t *testing.T) {
	t.Parallel()

	server := New("token-123", &fakeAdmin{}, fakeLogger{})
	req := httptest.NewRequest(http.MethodGet, "/v1/info", nil)
	req.Header.Set("Authorization", "Bearer token-123")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`"service":"windowsuseruptimecontrol"`,
		`"path":"/v1/info"`,
		`"path":"/v1/config"`,
		`"path":"/v1/announce"`,
		`"path":"/v1/users/{userId}/adjust"`,
		`Authorization: Bearer token-123`,
		`"delta_sec":300`,
		`"message":"WindowsUserUptimeControl test announcement"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %q in %s", want, body)
		}
	}
}

func TestHelperStreamRequiresHelperToken(t *testing.T) {
	t.Parallel()

	helpers := helperipc.NewServer()
	server := NewWithHelper("token-123", &fakeAdmin{}, fakeLogger{}, "helper-token", helpers)
	req := httptest.NewRequest(http.MethodGet, "/internal/helper/stream?user_sid=sid-john&session_id=5", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHelperStreamDeliversCommandsToAuthenticatedHelper(t *testing.T) {
	t.Parallel()

	helpers := helperipc.NewServer()
	server := NewWithHelper("token-123", &fakeAdmin{}, fakeLogger{}, "helper-token", helpers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/internal/helper/stream?user_sid=sid-john&session_id=5", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer helper-token")
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		server.ServeHTTP(rec, req)
		close(done)
	}()

	waitFor(t, func() bool { return helpers.Connected("sid-john") }, "helper connection")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if err := helpers.Send(context.Background(), "sid-john", helperipc.Command{Type: helperipc.CommandSpeak, Message: "hello"}); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	waitFor(t, func() bool { return strings.Contains(rec.Body.String(), `"message":"hello"`) }, "streamed command")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for helper stream to close")
	}
}

func TestHelperStreamAnnouncesEstablishedConnection(t *testing.T) {
	t.Parallel()

	helpers := helperipc.NewServer()
	server := NewWithHelper("token-123", &fakeAdmin{}, fakeLogger{}, "helper-token", helpers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/internal/helper/stream?user_sid=sid-john&session_id=5", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer helper-token")
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		server.ServeHTTP(rec, req)
		close(done)
	}()

	waitFor(t, func() bool {
		return strings.Contains(rec.Body.String(), `"message":"Connection with uptime control service established."`)
	}, "connection announcement")

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for helper stream to close")
	}
}

func waitFor(t *testing.T, ok func() bool, label string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}
