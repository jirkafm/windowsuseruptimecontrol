//go:build windows

package helper

import (
	"fmt"
	"os/exec"
	"strings"
)

type WindowsSpeaker struct{}

func (WindowsSpeaker) Speak(text string) error {
	escaped := strings.ReplaceAll(text, "'", "''")
	cmd := fmt.Sprintf("Add-Type -AssemblyName System.Speech; (New-Object System.Speech.Synthesis.SpeechSynthesizer).Speak('%s')", escaped)
	return exec.Command("powershell", "-Command", cmd).Run()
}
