//go:build !windows

package helper

type WindowsSpeaker struct {
	Culture string
}

func (WindowsSpeaker) Speak(string) error { return nil }
