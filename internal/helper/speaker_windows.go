//go:build windows

package helper

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

type WindowsSpeaker struct{}

func (WindowsSpeaker) Speak(text string) error {
	return newSpeechCommand(text).Run()
}

func newSpeechCommand(text string) *exec.Cmd {
	escaped := strings.ReplaceAll(text, "'", "''")
	cmd := fmt.Sprintf("Add-Type -AssemblyName System.Speech; (New-Object System.Speech.Synthesis.SpeechSynthesizer).Speak('%s')", escaped)
	command := exec.Command("powershell", "-Command", cmd)
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
	return command
}
