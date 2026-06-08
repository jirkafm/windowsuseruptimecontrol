package policy

import (
	"time"

	"windowsuseruptimecontrol/internal/i18n"
	"windowsuseruptimecontrol/internal/model"
)

type Engine struct {
	Language                 string
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
			addMessage(&result, i18n.DailyNoTime(e.Language, e.ReenforcementDelaySec), i18n.DailyNoTime(i18n.English, e.ReenforcementDelaySec))
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
		addMessage(&result, i18n.DailyRemaining(e.Language, user.RemainingSec/60), i18n.DailyRemaining(i18n.English, user.RemainingSec/60))
		user.StartupWarningSent = true
	}
	if e.WarningHalfwayEnabled && !user.HalfwayWarningSent && user.ConsumedSec >= halfwayThreshold {
		addMessage(&result, i18n.DailyHalfway(e.Language), i18n.DailyHalfway(i18n.English))
		user.HalfwayWarningSent = true
	}
	if e.WarningFiveMinEnabled && !user.FiveMinWarningSent && user.RemainingSec <= 300 {
		addMessage(&result, i18n.DailyFiveMinutes(e.Language), i18n.DailyFiveMinutes(i18n.English))
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

func addMessage(result *model.Evaluation, message, fallbackMessage string) {
	result.Messages = append(result.Messages, message)
	result.FallbackMessages = append(result.FallbackMessages, fallbackMessage)
}

func countdown() []string {
	return []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1"}
}
