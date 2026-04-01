package helperipc

import (
	"context"
	"testing"
	"time"
)

func TestServerDeliversSpeakCommandToRegisteredHelper(t *testing.T) {
	t.Parallel()

	server := NewServer()
	helper := make(chan Command, 1)
	server.Register("sid-john", helper)

	if err := server.Send(context.Background(), "sid-john", Command{Type: CommandSpeak, Message: "You have 5 minutes remaining."}); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	select {
	case got := <-helper:
		if got.Message != "You have 5 minutes remaining." {
			t.Fatalf("Message = %q, want warning", got.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for helper command")
	}
}
