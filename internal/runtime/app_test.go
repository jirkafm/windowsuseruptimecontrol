package runtime

import (
	"testing"

	"windowsuseruptimecontrol/internal/model"
)

func TestInstallRootUsesRenamedLocalDirectory(t *testing.T) {
	t.Setenv("WINDOWS_USER_UPTIME_CONTROL_ROOT", "")
	t.Setenv("WINCONTROL_ROOT", "")

	if got := installRoot(); got != ".windowsuseruptimecontrol" {
		t.Fatalf("installRoot() = %q, want %q", got, ".windowsuseruptimecontrol")
	}
}

func TestHelperStreamURLUsesLoopbackForWildcardBind(t *testing.T) {
	t.Parallel()

	got := helperStreamURL(model.Config{APIBindAddress: "0.0.0.0", APIPort: 8111})
	want := "http://127.0.0.1:8111/internal/helper/stream"
	if got != want {
		t.Fatalf("helperStreamURL() = %q, want %q", got, want)
	}
}

func TestHelperConnectionArgsRequireURLAndToken(t *testing.T) {
	t.Parallel()

	_, _, _, err := helperConnectionArgs([]string{"activityhelper.exe", "--session-id", "5"})
	if err == nil {
		t.Fatal("expected missing helper connection args to fail")
	}
}

func TestHelperConnectionArgsParsesURLTokenAndSession(t *testing.T) {
	t.Parallel()

	streamURL, token, sessionID, err := helperConnectionArgs([]string{
		"activityhelper.exe",
		"--helper-url", "http://127.0.0.1:8111/internal/helper/stream",
		"--helper-token", "token-123",
		"--session-id", "5",
	})
	if err != nil {
		t.Fatalf("helperConnectionArgs error: %v", err)
	}
	if streamURL != "http://127.0.0.1:8111/internal/helper/stream" || token != "token-123" || sessionID != 5 {
		t.Fatalf("args = url %q token %q session %d", streamURL, token, sessionID)
	}
}

func TestNewHelperTokenReturnsToken(t *testing.T) {
	t.Parallel()

	token, err := newHelperToken()
	if err != nil {
		t.Fatalf("newHelperToken error: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64", len(token))
	}
}
