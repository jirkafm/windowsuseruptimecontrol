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

func shouldLaunch(now time.Time, hb helperstatus.Heartbeat, sessionID uint32) bool {
	if hb.UserSID == "" {
		return true
	}
	if hb.SessionID != sessionID {
		return true
	}
	return !helperstatus.IsFresh(hb, now, heartbeatTTL)
}
