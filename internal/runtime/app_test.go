package runtime

import (
	"strings"
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

func TestUserUIAddrBindsLoopbackOnConfiguredPort(t *testing.T) {
	t.Parallel()

	got := userUIAddr(model.Config{APIBindAddress: "0.0.0.0", APIPort: 8111, UserUIPort: 8122})
	if got != "127.0.0.1:8122" {
		t.Fatalf("userUIAddr = %q, want 127.0.0.1:8122", got)
	}
}

func TestUserUIAddrReusesAPIPortOnlyForLoopbackAPI(t *testing.T) {
	t.Parallel()

	got := userUIAddr(model.Config{APIBindAddress: "127.0.0.1", APIPort: 8111})
	if got != "127.0.0.1:8111" {
		t.Fatalf("userUIAddr = %q, want 127.0.0.1:8111", got)
	}
}

func TestUserUIAddrChoosesDefaultWhenAPIIsWildcardAndPortUnset(t *testing.T) {
	t.Parallel()

	got := userUIAddr(model.Config{APIBindAddress: "0.0.0.0", APIPort: 8111})
	if got != "127.0.0.1:8112" {
		t.Fatalf("userUIAddr = %q, want 127.0.0.1:8112", got)
	}
}

func TestApplyServiceStartupArgsEnablesWeeklyFlexMode(t *testing.T) {
	t.Parallel()

	cfg := model.Config{QuotaMode: model.QuotaModeDaily, UserUIEnabled: false}
	got, err := applyServiceStartupArgs(cfg, []string{"activitysvc.exe", "--quota-mode", "weekly-flex"})
	if err != nil {
		t.Fatalf("applyServiceStartupArgs error: %v", err)
	}
	if got.QuotaMode != model.QuotaModeWeeklyFlex {
		t.Fatalf("QuotaMode = %q, want weekly-flex", got.QuotaMode)
	}
	if !got.UserUIEnabled {
		t.Fatal("UserUIEnabled = false, want true when weekly-flex is enabled by startup args")
	}
}

func TestApplyServiceStartupArgsRejectsUnknownQuotaMode(t *testing.T) {
	t.Parallel()

	_, err := applyServiceStartupArgs(model.Config{}, []string{"activitysvc.exe", "--quota-mode", "monthly"})
	if err == nil {
		t.Fatal("expected invalid quota mode to fail")
	}
	if !strings.Contains(err.Error(), "quota-mode") {
		t.Fatalf("error = %v, want quota-mode validation", err)
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
