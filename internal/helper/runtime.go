package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"windowsuseruptimecontrol/internal/helperipc"
)

type Speaker interface {
	Speak(text string) error
}

type Runtime struct {
	Speaker    Speaker
	HTTPClient *http.Client
}

func (r Runtime) Run(ctx context.Context, commands <-chan helperipc.Command) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cmd, ok := <-commands:
			if !ok {
				return nil
			}
			if cmd.Type == helperipc.CommandSpeak {
				if err := r.Speaker.Speak(cmd.Message); err != nil {
					return err
				}
			}
		}
	}
}

func (r Runtime) RunHTTPStream(ctx context.Context, streamURL, token, userSID string, sessionID uint32) error {
	parsed, err := url.Parse(streamURL)
	if err != nil {
		return err
	}
	query := parsed.Query()
	query.Set("user_sid", userSID)
	query.Set("session_id", strconv.FormatUint(uint64(sessionID), 10))
	parsed.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := r.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("helper stream rejected: status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var cmd helperipc.Command
		if err := decoder.Decode(&cmd); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if cmd.Type == helperipc.CommandSpeak {
			if err := r.Speaker.Speak(cmd.Message); err != nil {
				return err
			}
		}
	}
}
