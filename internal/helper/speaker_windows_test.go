//go:build windows

package helper

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestNewSpeechCommandHidesPowerShellWindow(t *testing.T) {
	t.Parallel()

	cmd := newSpeechCommand("10")
	if cmd.Path != "powershell" && cmd.Args[0] != "powershell" {
		t.Fatalf("command = %#v, want powershell", cmd.Args)
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("expected process attributes that hide the PowerShell window")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("expected HideWindow to be enabled")
	}
	if cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("expected CREATE_NO_WINDOW creation flag")
	}
}
