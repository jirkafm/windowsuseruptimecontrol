package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserUIServesDashboardShell(t *testing.T) {
	t.Parallel()

	server := New("token-123", &fakeAdmin{}, fakeLogger{})
	req := httptest.NewRequest(http.MethodGet, "/user/", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Weekly time", "weekly-chart", "allocation-form"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q in %s", want, body)
		}
	}
}
