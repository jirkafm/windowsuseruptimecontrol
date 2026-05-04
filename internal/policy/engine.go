package policy

import (
	"fmt"
	"time"

	"windowsuseruptimecontrol/internal/model"
)

type Engine struct {
	DefaultDailyAllowanceSec int64
	ReenforcementDelaySec    int64
	WarningHalfwayEnabled    bool
	WarningFiveMinEnabled    bool
}

func (e Engine) Evaluate(now time.Time, active model.ActiveUser, state model.StateFile, elapsedSec int64) model.Evaluation {
	if state.Users == nil {
		state.Users = map[string]model.UserDayState{}
	}

	user := state.Users[active.UserSID]
	if user.UserSID == "" {
		user = model.UserDayState{
			UserSID:           active.UserSID,
			Username:          active.Username,
			Date:              now.Format("2006-01-02"),
			DailyAllowanceSec: e.DefaultDailyAllowanceSec,
		}
	}

	user.Date = now.Format("2006-01-02")
	user.Username = active.Username
	if user.DailyAllowanceSec == 0 {
		user.DailyAllowanceSec = e.DefaultDailyAllowanceSec
	}

	result := model.Evaluation{State: state}

	if user.Exhausted {
		if !user.ReenforcementPending {
			user.ReenforcementPending = true
			user.ReenforcementDeadline = now.Add(time.Duration(e.ReenforcementDelaySec) * time.Second)
			result.Messages = append(result.Messages, fmt.Sprintf("No time remains for today. The computer will hibernate in %d seconds.", e.ReenforcementDelaySec))
		} else if !user.ReenforcementDeadline.After(now) {
			result.TriggerEnforcement = true
			result.Countdown = countdown()
		}
		state.Users[active.UserSID] = user
		result.State = state
		return result
	}

	user.ConsumedSec += elapsedSec
	user.RecalculateRemaining()

	halfwayThreshold := user.DailyAllowanceSec / 2
	if !user.StartupWarningSent {
		result.Messages = append(result.Messages, fmt.Sprintf("You have %d minutes remaining.", user.RemainingSec/60))
		user.StartupWarningSent = true
	}
	if e.WarningHalfwayEnabled && !user.HalfwayWarningSent && user.ConsumedSec >= halfwayThreshold {
		result.Messages = append(result.Messages, "You have 30 minutes remaining.")
		user.HalfwayWarningSent = true
	}
	if e.WarningFiveMinEnabled && !user.FiveMinWarningSent && user.RemainingSec <= 300 {
		result.Messages = append(result.Messages, "You have 5 minutes remaining.")
		user.FiveMinWarningSent = true
	}
	if user.RemainingSec == 0 {
		user.Exhausted = true
		user.LastEnforcementReason = "daily allowance exhausted"
		result.TriggerEnforcement = true
		result.Countdown = countdown()
	}

	state.Users[active.UserSID] = user
	result.State = state
	return result
}

func countdown() []string {
	return []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1"}
}
