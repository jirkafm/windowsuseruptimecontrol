//go:build !windows

package session

import (
	"context"
	"errors"

	"windowsuseruptimecontrol/internal/model"
)

type Detector struct{}

func (Detector) ActiveUser(context.Context) (model.ActiveUser, bool, error) {
	return model.ActiveUser{}, false, errors.New("active console detection is windows-only")
}
