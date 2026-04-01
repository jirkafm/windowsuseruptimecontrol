//go:build !windows

package helper

type WindowsSpeaker struct{}

func (WindowsSpeaker) Speak(string) error { return nil }
