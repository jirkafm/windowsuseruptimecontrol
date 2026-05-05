package helper

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Launcher struct {
	HelperPath     string
	HelperURL      string
	HelperToken    string
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

func shouldLaunch(now time.Time, sessionID uint32) bool {
	return shouldLaunchWithCooldown(now, sessionID, launchAttempt{}, 5*time.Second)
}

func shouldLaunchWithCooldown(now time.Time, sessionID uint32, lastLaunch launchAttempt, cooldown time.Duration) bool {
	if cooldown > 0 && lastLaunch.SessionID == sessionID && !lastLaunch.StartedAt.IsZero() && now.Sub(lastLaunch.StartedAt) <= cooldown {
		return false
	}
	return true
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

func buildCommandLine(helperPath string, sessionID uint32, helperURL, helperToken string) string {
	return fmt.Sprintf(
		"%s --session-id %d --helper-url %s --helper-token %s",
		quoteArg(helperPath),
		sessionID,
		quoteArg(helperURL),
		quoteArg(helperToken),
	)
}

func quoteArg(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}
