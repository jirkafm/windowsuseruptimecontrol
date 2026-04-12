package helper

import (
	"context"

	"windowsuseruptimecontrol/internal/helperipc"
)

type Speaker interface {
	Speak(text string) error
}

type Runtime struct {
	Speaker Speaker
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
