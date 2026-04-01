package helper

import (
	"context"
	"testing"

	"wincontrol/internal/helperipc"
)

type fakeSpeaker struct {
	spoken []string
}

func (f *fakeSpeaker) Speak(text string) error {
	f.spoken = append(f.spoken, text)
	return nil
}

func TestRuntimeSpeaksIncomingMessages(t *testing.T) {
	t.Parallel()

	speaker := &fakeSpeaker{}
	commands := make(chan helperipc.Command, 1)
	commands <- helperipc.Command{Type: helperipc.CommandSpeak, Message: "10"}
	close(commands)

	rt := Runtime{Speaker: speaker}
	if err := rt.Run(context.Background(), commands); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(speaker.spoken) != 1 || speaker.spoken[0] != "10" {
		t.Fatalf("spoken = %#v, want [10]", speaker.spoken)
	}
}
