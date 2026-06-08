//go:build windows

package helper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNewSpeechCommandHidesPowerShellWindow(t *testing.T) {
	t.Parallel()

	cmd := newSpeechCommand("10", "cs-CZ")
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

func TestNewSpeechCommandSelectsRequestedCulture(t *testing.T) {
	t.Parallel()

	cmd := newSpeechCommand("Zbývá ti 5 minut.", "cs-CZ")
	got := cmd.Args[len(cmd.Args)-1]
	if !strings.Contains(got, "GetInstalledVoices") {
		t.Fatalf("command = %q, want installed voice lookup", got)
	}
	if !strings.Contains(got, "cs-CZ") {
		t.Fatalf("command = %q, want cs-CZ culture", got)
	}
}

func TestWindowsSpeakerFallsBackToEnglishWhenRequestedCultureUnavailable(t *testing.T) {
	t.Parallel()

	var spoken []string
	var cultures []string
	logPath := filepath.Join(t.TempDir(), "helper.log")
	speaker := &WindowsSpeaker{
		Culture: "cs-CZ",
		LogPath: logPath,
		voiceAvailable: func(culture string) (bool, error) {
			return false, nil
		},
		speak: func(text, culture string) error {
			spoken = append(spoken, text)
			cultures = append(cultures, culture)
			return nil
		},
	}

	if err := speaker.Speak("Zbývá ti 5 minut.", "You have 5 minutes remaining."); err != nil {
		t.Fatalf("Speak error: %v", err)
	}
	if err := speaker.Speak("Zbývá ti 30 minut.", "You have 30 minutes remaining."); err != nil {
		t.Fatalf("second Speak error: %v", err)
	}

	if len(spoken) != 3 {
		t.Fatalf("spoken = %#v, want fallback notice once plus two messages", spoken)
	}
	if spoken[0] != fallbackNoticeEnglish {
		t.Fatalf("first spoken = %q, want fallback notice", spoken[0])
	}
	if spoken[1] != "You have 5 minutes remaining." || spoken[2] != "You have 30 minutes remaining." {
		t.Fatalf("spoken = %#v, want English fallback messages", spoken)
	}
	for _, culture := range cultures {
		if culture != "en-US" {
			t.Fatalf("cultures = %#v, want en-US fallback", cultures)
		}
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read helper log: %v", err)
	}
	if got := strings.Count(string(data), "configured speech culture unavailable"); got != 1 {
		t.Fatalf("fallback log count = %d in %q, want 1", got, string(data))
	}
}
