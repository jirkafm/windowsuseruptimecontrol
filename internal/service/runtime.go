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
	"windowsuseruptimecontrol/internal/weekly"
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

type Logger interface {
	Servicef(format string, args ...any)
}

type Runtime struct {
	mu       sync.Mutex
	Config   model.Config
	Store    Store
	Detector Detector
	Helper   HelperBus
	Power    PowerController
	Log      Logger

	lastTick                    time.Time
	restartReenforcementPending bool
	activeTrackingSID           string
	noActiveLogged              bool
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
		if !r.noActiveLogged {
			r.logf("uptime control paused: no active console user")
			r.noActiveLogged = true
		}
		r.activeTrackingSID = ""
		return r.Store.Save(state)
	}
	r.noActiveLogged = false
	if r.restartReenforcementPending {
		state = restartReenforcementDelay(state, active.UserSID)
		r.restartReenforcementPending = false
	}

	if r.Config.QuotaMode == model.QuotaModeWeeklyFlex {
		return r.tickWeekly(ctx, now, active, state, elapsedSec)
	}
	return r.tickDaily(ctx, now, active, state, elapsedSec)
}

func (r *Runtime) tickDaily(ctx context.Context, now time.Time, active model.ActiveUser, state model.StateFile, elapsedSec int64) error {
	engine := policy.Engine{
		DefaultDailyAllowanceSec: r.Config.DefaultDailyAllowanceSec,
		ReenforcementDelaySec:    r.Config.ReenforcementDelaySec,
		WarningHalfwayEnabled:    r.Config.WarningHalfwayEnabled,
		WarningFiveMinEnabled:    r.Config.WarningFiveMinEnabled,
	}
	beforeUser, hadBeforeUser := state.Users[active.UserSID]
	result := engine.Evaluate(now, active, state, elapsedSec)
	afterUser := result.State.Users[active.UserSID]

	if err := r.Store.Save(result.State); err != nil {
		return err
	}

	if r.activeTrackingSID != active.UserSID {
		r.logf(
			"uptime control started for user username=%s sid=%s session=%d allowance_sec=%d remaining_sec=%d",
			active.Username,
			active.UserSID,
			active.SessionID,
			afterUser.DailyAllowanceSec,
			afterUser.RemainingSec,
		)
		r.activeTrackingSID = active.UserSID
	}
	if (!hadBeforeUser || !beforeUser.Exhausted) && afterUser.Exhausted {
		r.logf(
			"user time depleted username=%s sid=%s consumed_sec=%d allowance_sec=%d reason=%q",
			afterUser.Username,
			afterUser.UserSID,
			afterUser.ConsumedSec,
			afterUser.DailyAllowanceSec,
			afterUser.LastEnforcementReason,
		)
	}
	if hadBeforeUser && beforeUser.Exhausted && !beforeUser.ReenforcementPending && afterUser.ReenforcementPending {
		r.logf(
			"reenforcement delay started username=%s sid=%s deadline=%s delay_sec=%d",
			afterUser.Username,
			afterUser.UserSID,
			afterUser.ReenforcementDeadline.Format(time.RFC3339),
			r.Config.ReenforcementDelaySec,
		)
	}
	if hadBeforeUser && beforeUser.Exhausted && beforeUser.ReenforcementPending && !beforeUser.ReenforcementDeadline.After(now) && result.TriggerEnforcement {
		r.logf(
			"reenforcement delay expired username=%s sid=%s triggering_enforcement=true",
			afterUser.Username,
			afterUser.UserSID,
		)
	}

	return r.deliverEvaluation(ctx, afterUser.UserSID, afterUser.Username, afterUser.RemainingSec, result)
}

func (r *Runtime) tickWeekly(ctx context.Context, now time.Time, active model.ActiveUser, state model.StateFile, elapsedSec int64) error {
	engine := weekly.Engine{
		DefaultWeeklyAllowanceSec: r.Config.DefaultWeeklyAllowanceSec,
		ReenforcementDelaySec:     r.Config.ReenforcementDelaySec,
		WarningHalfwayEnabled:     r.Config.WarningHalfwayEnabled,
		WarningFiveMinEnabled:     r.Config.WarningFiveMinEnabled,
	}
	beforeUser := state.WeeklyUsers[active.UserSID]
	result := engine.Evaluate(now, active, state, elapsedSec)
	afterUser := result.State.WeeklyUsers[active.UserSID]

	if err := r.Store.Save(result.State); err != nil {
		return err
	}

	if r.activeTrackingSID != active.UserSID {
		r.logf(
			"weekly flex control started for user username=%s sid=%s session=%d allowance_sec=%d remaining_sec=%d",
			active.Username,
			active.UserSID,
			active.SessionID,
			afterUser.WeeklyAllowanceSec,
			afterUser.RemainingSec,
		)
		r.activeTrackingSID = active.UserSID
	}
	if !beforeUser.Exhausted && afterUser.Exhausted {
		r.logf(
			"weekly time depleted username=%s sid=%s consumed_sec=%d allowance_sec=%d reason=%q",
			afterUser.Username,
			afterUser.UserSID,
			afterUser.WeeklyConsumedSec(),
			afterUser.WeeklyAllowanceSec,
			afterUser.LastEnforcementReason,
		)
	}

	return r.deliverEvaluation(ctx, afterUser.UserSID, afterUser.Username, afterUser.RemainingSec, result)
}

func (r *Runtime) deliverEvaluation(ctx context.Context, userSID, username string, remainingSec int64, result model.Evaluation) error {
	var notifyErr error
	speak := func(message string) {
		if err := r.Helper.Speak(ctx, userSID, message); err != nil {
			notifyErr = errors.Join(notifyErr, err)
		}
	}

	for _, message := range result.Messages {
		r.logf(
			"quota message issued username=%s sid=%s remaining_sec=%d message=%q",
			username,
			userSID,
			remainingSec,
			message,
		)
		speak(message)
	}
	if result.TriggerEnforcement {
		for _, number := range result.Countdown {
			speak(number)
		}
		r.logf("enforcement hibernate requested username=%s sid=%s", username, userSID)
		if err := r.Power.Hibernate(ctx); err != nil {
			r.logf("hibernate failed; attempting shutdown username=%s sid=%s error=%v", username, userSID, err)
			if shutdownErr := r.Power.Shutdown(ctx); shutdownErr != nil {
				r.logf("shutdown fallback failed username=%s sid=%s error=%v", username, userSID, shutdownErr)
				return errors.Join(notifyErr, err, shutdownErr)
			}
			r.logf("shutdown fallback completed username=%s sid=%s", username, userSID)
		} else {
			r.logf("hibernate completed username=%s sid=%s", username, userSID)
		}
	}
	return notifyErr
}

func (r *Runtime) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log.Servicef(format, args...)
	}
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

func (r *Runtime) ActiveWeeklyStatus(ctx context.Context, now time.Time) (model.WeeklyUserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.Store.LoadOrCreate(now, r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	active, ok, err := r.Detector.ActiveUser(ctx)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	if !ok {
		return model.WeeklyUserState{}, fmt.Errorf("no active console user")
	}
	current := weekly.NormalizeUser(now, active, state.WeeklyUsers[active.UserSID], r.Config.DefaultWeeklyAllowanceSec)
	state.WeeklyUsers[active.UserSID] = current
	if err := r.Store.Save(state); err != nil {
		return model.WeeklyUserState{}, err
	}
	return current, nil
}

func (r *Runtime) UpdateActiveWeeklyDistribution(ctx context.Context, now time.Time, dist [7]int64) (model.WeeklyUserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, err := r.Store.LoadOrCreate(now, r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	active, ok, err := r.Detector.ActiveUser(ctx)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	if !ok {
		return model.WeeklyUserState{}, fmt.Errorf("no active console user")
	}
	current := weekly.NormalizeUser(now, active, state.WeeklyUsers[active.UserSID], r.Config.DefaultWeeklyAllowanceSec)
	updated, err := weekly.ApplyDistribution(now, current, dist)
	if err != nil {
		return model.WeeklyUserState{}, err
	}
	state.WeeklyUsers[active.UserSID] = updated
	return updated, r.Store.Save(state)
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
		"log_max_size_mb":             r.Config.LogMaxSizeMB,
		"log_max_backups":             r.Config.LogMaxBackups,
		"log_max_age_days":            r.Config.LogMaxAgeDays,
		"log_compress":                r.Config.LogCompress,
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
	if err := r.Store.Save(state); err != nil {
		return model.UserDayState{}, err
	}
	r.announceAllowanceChanged(context.Background(), key, current)
	return current, nil
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

func (r *Runtime) announceAllowanceChanged(ctx context.Context, userKey string, user model.UserDayState) {
	if r.Detector == nil || r.Helper == nil {
		return
	}
	active, ok, err := r.Detector.ActiveUser(ctx)
	if err != nil || !ok {
		return
	}
	if !strings.EqualFold(active.UserSID, user.UserSID) && !strings.EqualFold(active.UserSID, userKey) {
		return
	}
	_ = r.Helper.Speak(ctx, active.UserSID, fmt.Sprintf(
		"Your remaining time has changed. You have %d minutes remaining.",
		user.RemainingSec/60,
	))
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
