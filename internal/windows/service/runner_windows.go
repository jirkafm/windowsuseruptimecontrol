//go:build windows

package service

import (
	"context"
	"sync"

	"golang.org/x/sys/windows/svc"
)

type Runner struct {
	Name    string
	RunFunc func(context.Context) error
}

func (r Runner) Validate() error { return nil }

func (r Runner) Run() error {
	return svc.Run(r.Name, r)
}

func (r Runner) Execute(args []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	if r.RunFunc != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.RunFunc(ctx)
		}()
	}

	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for req := range requests {
		switch req.Cmd {
		case svc.Stop, svc.Shutdown:
			cancel()
			wg.Wait()
			status <- svc.Status{State: svc.StopPending}
			return false, 0
		default:
			status <- req.CurrentStatus
		}
	}
	cancel()
	wg.Wait()
	return false, 0
}
