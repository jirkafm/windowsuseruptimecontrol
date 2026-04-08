package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"path/filepath"
	"time"

	"wincontrol/internal/api"
	"wincontrol/internal/config"
	"wincontrol/internal/helper"
	"wincontrol/internal/helperfs"
	"wincontrol/internal/helperstatus"
	"wincontrol/internal/logging"
	"wincontrol/internal/service"
	"wincontrol/internal/state"
	helperlauncher "wincontrol/internal/windows/helper"
	"wincontrol/internal/windows/power"
	"wincontrol/internal/windows/session"
)

func ServiceMain(ctx context.Context) error {
	baseDir := installRoot()
	cfg, err := config.Load(filepath.Join(baseDir, "config", "config.json"))
	if err != nil {
		return err
	}

	logger := logging.New(
		filepath.Join(baseDir, "logs", "service.log"),
		filepath.Join(baseDir, "logs", "api.log"),
	)
	store := state.NewJSONStore(filepath.Join(baseDir, "state", "state.json"))
	bus := helperfs.New(filepath.Join(baseDir, "state", "spool"))
	detector := session.Detector{}
	powerCtl := power.Controller{}
	runtime := &service.Runtime{
		Config:   cfg,
		Store:    store,
		Detector: detector,
		Helper:   bus,
		Power:    powerCtl,
	}

	server := api.New(cfg.BearerToken, runtime, logger)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.APIBindAddress, cfg.APIPort),
		Handler: server,
	}

	logger.Servicef("service starting on %s", httpServer.Addr)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Servicef("api server error: %v", err)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	launcher := helperlauncher.Launcher{
		HelperPath:     cfg.HelperPath,
		HeartbeatRoot:  filepath.Join(baseDir, "state", "heartbeats"),
		LaunchCooldown: time.Duration(cfg.HelperLaunchCooldownSec) * time.Second,
	}

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(shutdownCtx)
			logger.Servicef("service stopping")
			return nil
		case now := <-ticker.C:
			active, ok, err := detector.ActiveUser(ctx)
			if err != nil {
				logger.Servicef("active user detection failed: %v", err)
				continue
			}
			if ok {
				_ = launcher.EnsureRunning(ctx, active.SessionID, active.UserSID)
			}
			if err := runtime.Tick(ctx, now, 1); err != nil {
				logger.Servicef("tick failed: %v", err)
			}
		}
	}
}

func HelperMain(ctx context.Context) error {
	current, err := user.Current()
	if err != nil {
		return err
	}

	baseDir := installRoot()
	bus := helperfs.New(filepath.Join(baseDir, "state", "spool"))
	heartbeats := helperstatus.New(filepath.Join(baseDir, "state", "heartbeats"))
	speaker := helper.WindowsSpeaker{}
	logger := logging.New(
		filepath.Join(baseDir, "logs", "helper-service.log"),
		filepath.Join(baseDir, "logs", "helper-api.log"),
	)
	sessionID := helperSessionID()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := heartbeats.Record(current.Uid, helperstatus.Heartbeat{
				UserSID:   current.Uid,
				SessionID: sessionID,
				PID:       os.Getpid(),
				UpdatedAt: time.Now(),
			}); err != nil {
				logger.Servicef("heartbeat write failed: %v", err)
			}
			commands, err := bus.Poll(current.Uid)
			if err != nil {
				logger.Servicef("helper poll failed: %v", err)
				continue
			}
			for _, cmd := range commands {
				if cmd.Type == "speak" {
					if err := speaker.Speak(cmd.Message); err != nil {
						logger.Servicef("speak failed: %v", err)
					}
				}
			}
		}
	}
}

func installRoot() string {
	if root := os.Getenv("WINCONTROL_ROOT"); root != "" {
		return root
	}
	if isWindowsLikePath() {
		return `C:\ProgramData\Activity`
	}
	return ".wincontrol"
}

func isWindowsLikePath() bool {
	return os.PathSeparator == '\\'
}

func helperSessionID() uint32 {
	for idx := 0; idx < len(os.Args)-1; idx++ {
		if os.Args[idx] != "--session-id" {
			continue
		}
		value, err := strconv.ParseUint(os.Args[idx+1], 10, 32)
		if err != nil {
			return 0
		}
		return uint32(value)
	}
	return 0
}
