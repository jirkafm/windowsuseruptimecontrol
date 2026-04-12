package runtime

import "testing"

func TestInstallRootUsesRenamedLocalDirectory(t *testing.T) {
	t.Setenv("WINDOWS_USER_UPTIME_CONTROL_ROOT", "")
	t.Setenv("WINCONTROL_ROOT", "")

	if got := installRoot(); got != ".windowsuseruptimecontrol" {
		t.Fatalf("installRoot() = %q, want %q", got, ".windowsuseruptimecontrol")
	}
}
