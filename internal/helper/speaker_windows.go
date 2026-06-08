//go:build windows

package helper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"windowsuseruptimecontrol/internal/i18n"

	"golang.org/x/sys/windows"
)

const fallbackNoticeEnglish = "The desired language configuration is not available on this Windows machine. Falling back to English."

type WindowsSpeaker struct {
	Culture string
	LogPath string

	voiceAvailable func(culture string) (bool, error)
	speak          func(text, culture string) error

	mu                sync.Mutex
	voiceAvailability map[string]bool
	fallbackAnnounced map[string]bool
}

func (s *WindowsSpeaker) Speak(text string, fallbackText ...string) error {
	desiredCulture := i18n.SpeechCulture(s.Culture)
	textToSpeak := text
	cultureToUse := desiredCulture

	if desiredCulture != i18n.SpeechCulture(i18n.English) {
		available, err := s.isVoiceAvailable(desiredCulture)
		if err != nil {
			s.logf("speech voice availability check failed desired_culture=%s fallback_culture=en-US error=%v", desiredCulture, err)
		}
		if err != nil || !available {
			if fallback := firstFallbackText(fallbackText); fallback != "" {
				textToSpeak = fallback
			}
			cultureToUse = i18n.SpeechCulture(i18n.English)
			if s.markFallback(desiredCulture) {
				s.logf("configured speech culture unavailable desired_culture=%s fallback_culture=%s", desiredCulture, cultureToUse)
				if err := s.runSpeech(fallbackNoticeEnglish, cultureToUse); err != nil {
					return err
				}
			}
		}
	}

	return s.runSpeech(textToSpeak, cultureToUse)
}

func (s *WindowsSpeaker) isVoiceAvailable(culture string) (bool, error) {
	s.mu.Lock()
	if s.voiceAvailability != nil {
		if available, ok := s.voiceAvailability[culture]; ok {
			s.mu.Unlock()
			return available, nil
		}
	}
	s.mu.Unlock()

	check := installedVoiceAvailable
	if s.voiceAvailable != nil {
		check = s.voiceAvailable
	}
	available, err := check(culture)
	if err != nil {
		return false, err
	}

	s.mu.Lock()
	if s.voiceAvailability == nil {
		s.voiceAvailability = map[string]bool{}
	}
	s.voiceAvailability[culture] = available
	s.mu.Unlock()

	return available, nil
}

func (s *WindowsSpeaker) markFallback(culture string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fallbackAnnounced == nil {
		s.fallbackAnnounced = map[string]bool{}
	}
	if s.fallbackAnnounced[culture] {
		return false
	}
	s.fallbackAnnounced[culture] = true
	return true
}

func (s *WindowsSpeaker) runSpeech(text, culture string) error {
	if s.speak != nil {
		return s.speak(text, culture)
	}
	return newSpeechCommand(text, culture).Run()
}

func (s *WindowsSpeaker) logf(format string, args ...any) {
	if s.LogPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.LogPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(s.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
	_, _ = file.WriteString(line)
}

func firstFallbackText(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func newSpeechCommand(text, culture string) *exec.Cmd {
	escaped := strings.ReplaceAll(text, "'", "''")
	culture = strings.ReplaceAll(i18n.SpeechCulture(culture), "'", "''")
	cmd := fmt.Sprintf(
		"Add-Type -AssemblyName System.Speech; $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer; $cultureName = '%s'; $voice = $synth.GetInstalledVoices() | Where-Object { $_.Enabled -and $_.VoiceInfo.Culture.Name -eq $cultureName } | Select-Object -First 1; if ($voice) { $synth.SelectVoice($voice.VoiceInfo.Name) }; $synth.Speak('%s')",
		culture,
		escaped,
	)
	return hiddenPowerShellCommand(cmd)
}

func installedVoiceAvailable(culture string) (bool, error) {
	culture = strings.ReplaceAll(i18n.SpeechCulture(culture), "'", "''")
	cmd := fmt.Sprintf(
		"Add-Type -AssemblyName System.Speech; $synth = New-Object System.Speech.Synthesis.SpeechSynthesizer; $cultureName = '%s'; $voice = $synth.GetInstalledVoices() | Where-Object { $_.Enabled -and $_.VoiceInfo.Culture.Name -eq $cultureName } | Select-Object -First 1; if ($voice) { 'true' } else { 'false' }",
		culture,
	)
	output, err := hiddenPowerShellCommand(cmd).Output()
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(string(output)), "true"), nil
}

func hiddenPowerShellCommand(cmd string) *exec.Cmd {
	command := exec.Command("powershell", "-Command", cmd)
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
	return command
}
