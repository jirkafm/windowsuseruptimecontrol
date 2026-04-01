//go:build !windows

package service

import "context"

type Runner struct {
	Name    string
	RunFunc func(context.Context) error
}

func (Runner) Validate() error { return nil }

func (r Runner) Run() error {
	if r.RunFunc == nil {
		return nil
	}
	return r.RunFunc(context.Background())
}
