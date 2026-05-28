package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"windowsuseruptimecontrol/internal/api"
	"windowsuseruptimecontrol/internal/config"
	"windowsuseruptimecontrol/internal/helper"
	"windowsuseruptimecontrol/internal/helperipc"
	"windowsuseruptimecontrol/internal/logging"
	"windowsuseruptimecontrol/internal/model"
	"windowsuseruptimecontrol/internal/service"
	"windowsuseruptimecontrol/internal/state"
	helperlauncher "windowsuseruptimecontrol/internal/windows/helper"
	"windowsuseruptimecontrol/internal/windows/power"
	"windowsuseruptimecontrol/internal/windows/session"
)

func ServiceMain(ctx context.Context) error {
	baseDir := installRoot()
	cfg, err := config.Load(filepath.Join(baseDir, "config", "config.json"))
	if err != nil {
		return err
	}
	cfg, err = applyServiceStartupArgs(cfg, os.Args)
	if err != nil {
		return err
	}

	logger := logging.NewWithRotation(
		filepath.Join(baseDir, "logs", "service.log"),
		filepath.Join(baseDir, "logs", "api.log"),
		logging.RotationConfig{
			MaxSizeMB:  cfg.LogMaxSizeMB,
			MaxBackups: cfg.LogMaxBackups,
			MaxAgeDays: cfg.LogMaxAgeDays,
			Compress:   cfg.LogCompress,
		},
	)
	store := state.NewJSONStore(filepath.Join(baseDir, "state", "state.json"))
	helpers := helperipc.NewServer()
	detector := session.Detector{}
	powerCtl := power.Controller{}
	runtime := &service.Runtime{
		Config:   cfg,
		Store:    store,
		Detector: detector,
		Helper:   helpers,
		Power:    powerCtl,
		Log:      logger,
	}

	helperToken, err := newHelperToken()
	if err != nil {
		return err
	}
	server := api.NewWithHelper(cfg.BearerToken, runtime, logger, helperToken, helpers)
	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.APIBindAddress, cfg.APIPort),
		Handler: server,
	}
	var userUIServer *http.Server
	if cfg.UserUIEnabled && cfg.QuotaMode == model.QuotaModeWeeklyFlex && !isLoopbackBind(cfg.APIBindAddress) {
		userUIServer = &http.Server{
			Addr:    userUIAddr(cfg),
			Handler: server,
		}
	}

	logger.Servicef("service starting on %s", httpServer.Addr)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Servicef("api server error: %v", err)
		}
	}()
	if userUIServer != nil {
		logger.Servicef("user ui starting on %s", userUIServer.Addr)
		go func() {
			if err := userUIServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Servicef("user ui server error: %v", err)
			}
		}()
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	launcher := helperlauncher.Launcher{
		HelperPath:     cfg.HelperPath,
		HelperURL:      helperStreamURL(cfg),
		HelperToken:    helperToken,
		LaunchCooldown: time.Duration(cfg.HelperLaunchCooldownSec) * time.Second,
	}

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(shutdownCtx)
			if userUIServer != nil {
				_ = userUIServer.Shutdown(shutdownCtx)
			}
			logger.Servicef("service stopping")
			return nil
		case now := <-ticker.C:
			active, ok, err := detector.ActiveUser(ctx)
			if err != nil {
				logger.Servicef("active user detection failed: %v", err)
				continue
			}
			if ok && !helpers.Connected(active.UserSID) {
				logger.Servicef("helper launch requested username=%s sid=%s session=%d", active.Username, active.UserSID, active.SessionID)
				if err := launcher.EnsureRunning(ctx, active.SessionID, active.UserSID); err != nil {
					logger.Servicef("helper launch failed username=%s sid=%s session=%d error=%v", active.Username, active.UserSID, active.SessionID, err)
				}
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

	streamURL, token, sessionID, err := helperConnectionArgs(os.Args)
	if err != nil {
		return err
	}

	rt := helper.Runtime{Speaker: helper.WindowsSpeaker{}}
	return rt.RunHTTPStream(ctx, streamURL, token, current.Uid, sessionID)
}

func installRoot() string {
	if root := os.Getenv("WINDOWS_USER_UPTIME_CONTROL_ROOT"); root != "" {
		return root
	}
	if root := os.Getenv("WINCONTROL_ROOT"); root != "" {
		return root
	}
	if isWindowsLikePath() {
		return `C:\ProgramData\Activity`
	}
	return ".windowsuseruptimecontrol"
}

func isWindowsLikePath() bool {
	return os.PathSeparator == '\\'
}

func helperSessionID() uint32 {
	_, _, sessionID, err := helperConnectionArgs(os.Args)
	if err != nil {
		return 0
	}
	return sessionID
}

func helperConnectionArgs(args []string) (string, string, uint32, error) {
	var streamURL string
	var token string
	var sessionID uint32
	for idx := 0; idx < len(args)-1; idx++ {
		switch args[idx] {
		case "--helper-url":
			streamURL = args[idx+1]
		case "--helper-token":
			token = args[idx+1]
		case "--session-id":
			value, err := strconv.ParseUint(args[idx+1], 10, 32)
			if err != nil {
				return "", "", 0, err
			}
			sessionID = uint32(value)
		}
	}
	if strings.TrimSpace(streamURL) == "" {
		return "", "", 0, fmt.Errorf("helper-url is required")
	}
	if strings.TrimSpace(token) == "" {
		return "", "", 0, fmt.Errorf("helper-token is required")
	}
	return streamURL, token, sessionID, nil
}

func helperStreamURL(cfg model.Config) string {
	host := cfg.APIBindAddress
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(cfg.APIPort)) + "/internal/helper/stream"
}

func userUIAddr(cfg model.Config) string {
	port := cfg.UserUIPort
	if port == 0 && isLoopbackBind(cfg.APIBindAddress) {
		port = cfg.APIPort
	}
	if port == 0 {
		port = cfg.APIPort + 1
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func isLoopbackBind(host string) bool {
	return host == "127.0.0.1" || host == "localhost" || host == "::1" || host == "[::1]"
}

func applyServiceStartupArgs(cfg model.Config, args []string) (model.Config, error) {
	for idx := 0; idx < len(args)-1; idx++ {
		switch args[idx] {
		case "--quota-mode":
			mode := model.QuotaMode(args[idx+1])
			switch mode {
			case model.QuotaModeDaily:
				cfg.QuotaMode = model.QuotaModeDaily
			case model.QuotaModeWeeklyFlex:
				cfg.QuotaMode = model.QuotaModeWeeklyFlex
				cfg.UserUIEnabled = true
			default:
				return model.Config{}, fmt.Errorf("quota-mode must be %q or %q", model.QuotaModeDaily, model.QuotaModeWeeklyFlex)
			}
		}
	}
	return cfg, nil
}

func newHelperToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
