package helperfs

import (
	"context"
	"testing"

	"wincontrol/internal/helperipc"
)

func TestBusRoundTrip(t *testing.T) {
	t.Parallel()

	bus := New(t.TempDir())
	if err := bus.Speak(context.Background(), "sid-john", "You have 5 minutes remaining."); err != nil {
		t.Fatalf("Speak error: %v", err)
	}

	commands, err := bus.Poll("sid-john")
	if err != nil {
		t.Fatalf("Poll error: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(commands))
	}
	if commands[0] != (helperipc.Command{Type: helperipc.CommandSpeak, Message: "You have 5 minutes remaining."}) {
		t.Fatalf("command = %#v", commands[0])
	}
}
