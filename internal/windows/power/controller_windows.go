//go:build windows

package power

import (
	"context"
	"os/exec"
)

type Controller struct{}

func (Controller) Hibernate(context.Context) error {
	return exec.Command("shutdown", "/h").Run()
}

func (Controller) Shutdown(context.Context) error {
	return exec.Command("shutdown", "/s", "/t", "0").Run()
}
