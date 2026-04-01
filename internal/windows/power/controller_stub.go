//go:build !windows

package power

import "context"

type Controller struct{}

func (Controller) Hibernate(context.Context) error { return nil }

func (Controller) Shutdown(context.Context) error { return nil }
