package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"windowsuseruptimecontrol/internal/model"
	"windowsuseruptimecontrol/internal/policy"
)

type Store interface {
	LoadOrCreate(now time.Time, defaultDailyAllowanceSec int64) (model.StateFile, error)
	Save(state model.StateFile) error
}

type Detector interface {
	ActiveUser(ctx context.Context) (model.ActiveUser, bool, error)
}

type HelperBus interface {
	Speak(ctx context.Context, userSID, message string) error
}

type PowerController interface {
	Hibernate(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type Runtime struct {
	mu       sync.Mutex
	Config   model.Config
	Store    Store
	Detector Detector
	Helper   HelperBus
	Power    PowerController

	lastTick                    time.Time
	restartReenforcementPending bool
}

func (r *Runtime) Tick(ctx context.Context, now time.Time, elapsedSec int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.shouldRestartReenforcement(now) {
		r.restartReenforcementPending = true
	}
	r.lastTick = now

	state, err := r.Store.LoadOrCreate(now, r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return err
	}

	active, ok, err := r.Detector.ActiveUser(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return r.Store.Save(state)
	}
	if r.restartReenforcementPending {
		state = restartReenforcementDelay(state, active.UserSID)
		r.restartReenforcementPending = false
	}

	engine := policy.Engine{
		DefaultDailyAllowanceSec: r.Config.DefaultDailyAllowanceSec,
		ReenforcementDelaySec:    r.Config.ReenforcementDelaySec,
		WarningHalfwayEnabled:    r.Config.WarningHalfwayEnabled,
		WarningFiveMinEnabled:    r.Config.WarningFiveMinEnabled,
	}
	result := engine.Evaluate(now, active, state, elapsedSec)

	if err := r.Store.Save(result.State); err != nil {
		return err
	}

	var notifyErr error
	speak := func(message string) {
		if err := r.Helper.Speak(ctx, active.UserSID, message); err != nil {
			notifyErr = errors.Join(notifyErr, err)
		}
	}

	for _, message := range result.Messages {
		speak(message)
	}
	if result.TriggerEnforcement {
		for _, number := range result.Countdown {
			speak(number)
		}
		if err := r.Power.Hibernate(ctx); err != nil {
			if shutdownErr := r.Power.Shutdown(ctx); shutdownErr != nil {
				return errors.Join(notifyErr, err, shutdownErr)
			}
		}
	}
	return notifyErr
}

func (r *Runtime) shouldRestartReenforcement(now time.Time) bool {
	if r.lastTick.IsZero() {
		return true
	}
	return now.Sub(r.lastTick) > 30*time.Second
}

func restartReenforcementDelay(state model.StateFile, userSID string) model.StateFile {
	user := state.Users[userSID]
	if !user.Exhausted {
		return state
	}

	user.ReenforcementPending = false
	user.ReenforcementDeadline = time.Time{}
	state.Users[userSID] = user
	return state
}

func (r *Runtime) HibernateNow() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	active, ok, err := r.Detector.ActiveUser(context.Background())
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	for _, number := range []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1"} {
		if err := r.Helper.Speak(context.Background(), active.UserSID, number); err != nil {
			return err
		}
	}
	if err := r.Power.Hibernate(context.Background()); err != nil {
		return r.Power.Shutdown(context.Background())
	}
	return nil
}

func (r *Runtime) ConfigView() map[string]any {
	return map[string]any{
		"api_bind_address":            r.Config.APIBindAddress,
		"api_port":                    r.Config.APIPort,
		"default_daily_allowance_sec": r.Config.DefaultDailyAllowanceSec,
		"helper_launch_cooldown_sec":  r.Config.HelperLaunchCooldownSec,
		"reenforcement_delay_sec":     r.Config.ReenforcementDelaySec,
		"warning_halfway_enabled":     r.Config.WarningHalfwayEnabled,
		"warning_five_min_enabled":    r.Config.WarningFiveMinEnabled,
		"log_level":                   r.Config.LogLevel,
	}
}

func (r *Runtime) State() model.StateFile {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, _ := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	return state
}

func (r *Runtime) LookupUser(user string) (model.UserDayState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}

	key, err := resolveUserKey(state, user)
	if err != nil {
		return model.UserDayState{}, err
	}
	return state.Users[key], nil
}

func (r *Runtime) AdjustUser(user string, delta int64) (model.UserDayState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	key, err := resolveUserKey(state, user)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[key]
	current.ConsumedSec -= delta
	if current.ConsumedSec < 0 {
		current.ConsumedSec = 0
	}
	current.RecalculateRemaining()
	if current.RemainingSec > 0 {
		current.Exhausted = false
		current.ReenforcementPending = false
		current.ReenforcementDeadline = time.Time{}
	}
	state.Users[key] = current
	return current, r.Store.Save(state)
}

func (r *Runtime) SetAllowance(user string, sec int64) (model.UserDayState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	key, err := resolveUserKey(state, user)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[key]
	current.DailyAllowanceSec = sec
	current.RecalculateRemaining()
	if current.RemainingSec > 0 {
		current.Exhausted = false
		current.ReenforcementPending = false
		current.ReenforcementDeadline = time.Time{}
	}
	state.Users[key] = current
	return current, r.Store.Save(state)
}

func (r *Runtime) ResetToday(user string) (model.UserDayState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	key, err := resolveUserKey(state, user)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[key]
	current.ConsumedSec = 0
	current.RecalculateRemaining()
	current.Exhausted = false
	current.StartupWarningSent = false
	current.HalfwayWarningSent = false
	current.FiveMinWarningSent = false
	current.ReenforcementPending = false
	current.ReenforcementDeadline = time.Time{}
	state.Users[key] = current
	return current, r.Store.Save(state)
}

func (r *Runtime) Announce(message string) error {
	active, ok, err := r.Detector.ActiveUser(context.Background())
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return r.Helper.Speak(context.Background(), active.UserSID, message)
}

func resolveUserKey(state model.StateFile, input string) (string, error) {
	if _, ok := state.Users[input]; ok {
		return input, nil
	}

	needle := strings.TrimSpace(input)
	if needle == "" {
		return "", fmt.Errorf("user is required")
	}

	exactMatches := make([]string, 0, 1)
	simpleMatches := make([]string, 0, 1)
	for key, user := range state.Users {
		username := strings.TrimSpace(user.Username)
		if strings.EqualFold(username, needle) {
			exactMatches = append(exactMatches, key)
			continue
		}
		if strings.EqualFold(simpleUsername(username), needle) {
			simpleMatches = append(simpleMatches, key)
		}
	}

	switch {
	case len(exactMatches) == 1:
		return exactMatches[0], nil
	case len(exactMatches) > 1:
		return "", fmt.Errorf("ambiguous user %q", input)
	case len(simpleMatches) == 1:
		return simpleMatches[0], nil
	case len(simpleMatches) > 1:
		return "", fmt.Errorf("ambiguous user %q", input)
	default:
		return "", fmt.Errorf("user %q not found", input)
	}
}

func simpleUsername(username string) string {
	if idx := strings.LastIndex(username, `\`); idx >= 0 && idx < len(username)-1 {
		return username[idx+1:]
	}
	return username
}
