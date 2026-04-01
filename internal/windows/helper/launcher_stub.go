//go:build !windows

package helper

import "context"

func (Launcher) EnsureRunning(context.Context, uint32, string) error { return nil }
