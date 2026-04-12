package helper

import (
	"sync"
	"time"

	"windowsuseruptimecontrol/internal/helperstatus"
)

const heartbeatTTL = 30 * time.Second

type Launcher struct {
	HelperPath     string
	HeartbeatRoot  string
	LaunchCooldown time.Duration

	mu           sync.Mutex
	lastLaunches map[string]launchAttempt
}

type launchSettings struct {
	HideWindow      bool
	NoConsoleWindow bool
}

type launchAttempt struct {
	SessionID uint32
	StartedAt time.Time
}

func shouldLaunch(now time.Time, hb helperstatus.Heartbeat, sessionID uint32) bool {
	return shouldLaunchWithCooldown(now, hb, sessionID, launchAttempt{}, 5*time.Second)
}

func shouldLaunchWithCooldown(now time.Time, hb helperstatus.Heartbeat, sessionID uint32, lastLaunch launchAttempt, cooldown time.Duration) bool {
	if hb.UserSID == "" {
		if cooldown > 0 && lastLaunch.SessionID == sessionID && now.Sub(lastLaunch.StartedAt) <= cooldown {
			return false
		}
		return true
	}
	if hb.SessionID != sessionID {
		return true
	}
	return !helperstatus.IsFresh(hb, now, heartbeatTTL)
}

func (l *Launcher) launchCooldown() time.Duration {
	if l.LaunchCooldown > 0 {
		return l.LaunchCooldown
	}
	return 5 * time.Second
}

func (l *Launcher) lastLaunch(userSID string) launchAttempt {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastLaunches == nil {
		return launchAttempt{}
	}
	return l.lastLaunches[userSID]
}

func (l *Launcher) recordLaunch(userSID string, attempt launchAttempt) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastLaunches == nil {
		l.lastLaunches = make(map[string]launchAttempt)
	}
	l.lastLaunches[userSID] = attempt
}

func defaultLaunchSettings() launchSettings {
	return launchSettings{
		HideWindow:      true,
		NoConsoleWindow: true,
	}
}
