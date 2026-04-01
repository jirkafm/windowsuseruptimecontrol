package service

import (
	"context"
	"time"

	"wincontrol/internal/model"
	"wincontrol/internal/policy"
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
	Config   model.Config
	Store    Store
	Detector Detector
	Helper   HelperBus
	Power    PowerController
}

func (r *Runtime) Tick(ctx context.Context, now time.Time, elapsedSec int64) error {
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

	engine := policy.Engine{
		DefaultDailyAllowanceSec: r.Config.DefaultDailyAllowanceSec,
		ReenforcementDelaySec:    r.Config.ReenforcementDelaySec,
	}
	result := engine.Evaluate(now, active, state, elapsedSec)

	for _, message := range result.Messages {
		if err := r.Helper.Speak(ctx, active.UserSID, message); err != nil {
			return err
		}
	}
	if result.TriggerEnforcement {
		for _, number := range result.Countdown {
			if err := r.Helper.Speak(ctx, active.UserSID, number); err != nil {
				return err
			}
		}
		if err := r.Power.Hibernate(ctx); err != nil {
			if shutdownErr := r.Power.Shutdown(ctx); shutdownErr != nil {
				return shutdownErr
			}
		}
	}

	return r.Store.Save(result.State)
}

func (r *Runtime) HibernateNow() error {
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
		"reenforcement_delay_sec":     r.Config.ReenforcementDelaySec,
		"log_level":                   r.Config.LogLevel,
	}
}

func (r *Runtime) State() model.StateFile {
	state, _ := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	return state
}

func (r *Runtime) AdjustUser(user string, delta int64) (model.UserDayState, error) {
	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[user]
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
	state.Users[user] = current
	return current, r.Store.Save(state)
}

func (r *Runtime) SetAllowance(user string, sec int64) (model.UserDayState, error) {
	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[user]
	current.DailyAllowanceSec = sec
	current.RecalculateRemaining()
	if current.RemainingSec > 0 {
		current.Exhausted = false
		current.ReenforcementPending = false
		current.ReenforcementDeadline = time.Time{}
	}
	state.Users[user] = current
	return current, r.Store.Save(state)
}

func (r *Runtime) ResetToday(user string) (model.UserDayState, error) {
	state, err := r.Store.LoadOrCreate(time.Now(), r.Config.DefaultDailyAllowanceSec)
	if err != nil {
		return model.UserDayState{}, err
	}
	current := state.Users[user]
	current.ConsumedSec = 0
	current.RecalculateRemaining()
	current.Exhausted = false
	current.StartupWarningSent = false
	current.HalfwayWarningSent = false
	current.FiveMinWarningSent = false
	current.ReenforcementPending = false
	current.ReenforcementDeadline = time.Time{}
	state.Users[user] = current
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
