package helper

import (
	"time"

	"wincontrol/internal/helperstatus"
)

const heartbeatTTL = 30 * time.Second

type Launcher struct {
	HelperPath    string
	HeartbeatRoot string
}

type launchSettings struct {
	HideWindow      bool
	NoConsoleWindow bool
}

func shouldLaunch(now time.Time, hb helperstatus.Heartbeat, sessionID uint32) bool {
	if hb.UserSID == "" {
		return true
	}
	if hb.SessionID != sessionID {
		return true
	}
	return !helperstatus.IsFresh(hb, now, heartbeatTTL)
}

func defaultLaunchSettings() launchSettings {
	return launchSettings{
		HideWindow:      true,
		NoConsoleWindow: true,
	}
}
