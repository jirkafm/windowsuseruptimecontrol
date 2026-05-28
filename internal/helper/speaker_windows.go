//go:build windows

package helper

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"windowsuseruptimecontrol/internal/i18n"

	"golang.org/x/sys/windows"
)

type WindowsSpeaker struct {
	Culture string
}

func (s WindowsSpeaker) Speak(text string) error {
	return newSpeechCommand(text, s.Culture).Run()
}

func newSpeechCommand(text, culture string) *exec.Cmd {
	escaped := strings.ReplaceAll(text, "'", "''")
	culture = strings.ReplaceAll(i18n.SpeechCulture(culture), "'", "''")
	cmd := fmt.Sprintf(
		"Add-Type -AssemblyName System.Speech; Add-Type -AssemblyName System.Globalization; $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer; $synth.SelectVoiceByHints([System.Speech.Synthesis.VoiceGender]::NotSet, [System.Speech.Synthesis.VoiceAge]::NotSet, 0, (New-Object System.Globalization.CultureInfo('%s'))); $synth.Speak('%s')",
		culture,
		escaped,
	)
	command := exec.Command("powershell", "-Command", cmd)
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
	return command
}
