package helperipc

import (
	"context"
	"testing"
	"time"
)

func TestServerDeliversSpeakCommandToRegisteredHelper(t *testing.T) {
	t.Parallel()

	server := NewServer()
	helper, unregister := server.Register("sid-john", 5)
	defer unregister()

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

func TestServerReplacingHelperClosesPreviousConnection(t *testing.T) {
	t.Parallel()

	server := NewServer()
	first, firstUnregister := server.Register("sid-john", 5)
	defer firstUnregister()

	second, secondUnregister := server.Register("sid-john", 5)
	defer secondUnregister()

	select {
	case _, ok := <-first:
		if ok {
			t.Fatal("expected first helper channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first helper channel to close")
	}

	if err := server.Send(context.Background(), "sid-john", Command{Type: CommandSpeak, Message: "new helper"}); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	select {
	case got := <-second:
		if got.Message != "new helper" {
			t.Fatalf("Message = %q, want new helper", got.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for second helper command")
	}
}

func TestServerConnectedTracksActiveRegistration(t *testing.T) {
	t.Parallel()

	server := NewServer()
	if server.Connected("sid-john") {
		t.Fatal("expected helper to start disconnected")
	}

	_, unregister := server.Register("sid-john", 5)
	if !server.Connected("sid-john") {
		t.Fatal("expected helper to be connected after registration")
	}

	unregister()
	if server.Connected("sid-john") {
		t.Fatal("expected helper to be disconnected after unregister")
	}
}
