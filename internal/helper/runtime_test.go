package helper

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"windowsuseruptimecontrol/internal/helperipc"
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

func TestRuntimeSpeaksHTTPStreamMessagesAndExitsWhenClosed(t *testing.T) {
	t.Parallel()

	speaker := &fakeSpeaker{}
	reader, writer := io.Pipe()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer helper-token" {
			t.Fatalf("Authorization = %q, want helper token", req.Header.Get("Authorization"))
		}
		query := req.URL.Query()
		if query.Get("user_sid") != "sid-john" || query.Get("session_id") != "5" {
			t.Fatalf("query = %q, want user_sid and session_id", req.URL.RawQuery)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       reader,
			Header:     make(http.Header),
		}, nil
	})}

	go func() {
		_ = json.NewEncoder(writer).Encode(helperipc.Command{Type: helperipc.CommandSpeak, Message: "hello"})
		_ = writer.Close()
	}()

	rt := Runtime{Speaker: speaker, HTTPClient: client}
	if err := rt.RunHTTPStream(context.Background(), "http://activitysvc/internal/helper/stream", "helper-token", "sid-john", 5); err != nil {
		t.Fatalf("RunHTTPStream error: %v", err)
	}

	if len(speaker.spoken) != 1 || speaker.spoken[0] != "hello" {
		t.Fatalf("spoken = %#v, want [hello]", speaker.spoken)
	}
}

func TestRuntimeHTTPStreamReturnsErrorForRejectedConnection(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader("unauthorized")),
			Header:     make(http.Header),
		}, nil
	})}

	rt := Runtime{Speaker: &fakeSpeaker{}, HTTPClient: client}
	if err := rt.RunHTTPStream(context.Background(), "http://activitysvc/internal/helper/stream", "bad-token", "sid-john", 5); err == nil {
		t.Fatal("expected RunHTTPStream error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
