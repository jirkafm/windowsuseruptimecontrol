//go:build !windows

package helper

type WindowsSpeaker struct {
	Culture string
	LogPath string
}

func (*WindowsSpeaker) Speak(string, ...string) error { return nil }
