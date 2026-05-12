package weekly

import (
	"fmt"
	"time"

	"windowsuseruptimecontrol/internal/model"
)

type Engine struct {
	DefaultWeeklyAllowanceSec int64
	ReenforcementDelaySec     int64
	WarningHalfwayEnabled     bool
	WarningFiveMinEnabled     bool
}

func (e Engine) Evaluate(now time.Time, active model.ActiveUser, state model.StateFile, elapsedSec int64) model.Evaluation {
	if state.WeeklyUsers == nil {
		state.WeeklyUsers = map[string]model.WeeklyUserState{}
	}

	user := NormalizeUser(now, active, state.WeeklyUsers[active.UserSID], e.DefaultWeeklyAllowanceSec)
	result := model.Evaluation{State: state}

	if user.Exhausted || user.DayExhausted {
		if !user.ReenforcementPending {
			user.ReenforcementPending = true
			user.ReenforcementDeadline = now.Add(time.Duration(e.ReenforcementDelaySec) * time.Second)
			result.Messages = append(result.Messages, fmt.Sprintf("No time remains. The computer will hibernate in %d seconds.", e.ReenforcementDelaySec))
		} else if !user.ReenforcementDeadline.After(now) {
			result.TriggerEnforcement = true
			result.Countdown = countdown()
		}
		state.WeeklyUsers[active.UserSID] = user
		result.State = state
		return result
	}

	day := WeekdayIndex(now)
	user.ConsumedSec[day] += elapsedSec
	user.RecalculateWeeklyRemaining()

	if !user.StartupWarningSent {
		result.Messages = append(result.Messages, fmt.Sprintf("You have %d minutes remaining this week.", user.RemainingSec/60))
		user.StartupWarningSent = true
	}
	if e.WarningHalfwayEnabled && !user.HalfwayWarningSent && user.WeeklyConsumedSec() >= user.WeeklyAllowanceSec/2 {
		result.Messages = append(result.Messages, "You have used half of your weekly time.")
		user.HalfwayWarningSent = true
	}
	if e.WarningFiveMinEnabled && !user.FiveMinWarningSent && user.RemainingSec <= 300 {
		result.Messages = append(result.Messages, "You have 5 minutes remaining this week.")
		user.FiveMinWarningSent = true
	}

	if user.WeeklyConsumedSec() >= user.WeeklyAllowanceSec {
		user.Exhausted = true
		user.LastEnforcementReason = "weekly allowance exhausted"
		result.TriggerEnforcement = true
		result.Countdown = countdown()
	} else if user.ConsumedSec[day] >= user.AllocationsSec[day] {
		user.DayExhausted = true
		user.LastEnforcementReason = "daily allocation exhausted"
		result.TriggerEnforcement = true
		result.Countdown = countdown()
	}

	state.WeeklyUsers[active.UserSID] = user
	result.State = state
	return result
}

func NormalizeUser(now time.Time, active model.ActiveUser, user model.WeeklyUserState, defaultWeeklyAllowanceSec int64) model.WeeklyUserState {
	weekStart := WeekStart(now).Format("2006-01-02")
	if defaultWeeklyAllowanceSec <= 0 {
		defaultWeeklyAllowanceSec = 25200
	}
	if user.UserSID == "" {
		user = model.WeeklyUserState{
			UserSID:            active.UserSID,
			Username:           active.Username,
			WeekStart:          weekStart,
			WeeklyAllowanceSec: defaultWeeklyAllowanceSec,
			AllocationsSec:     DefaultDistribution(defaultWeeklyAllowanceSec),
		}
	}
	user.UserSID = active.UserSID
	user.Username = active.Username
	if user.WeeklyAllowanceSec == 0 {
		user.WeeklyAllowanceSec = defaultWeeklyAllowanceSec
	}
	if user.WeekStart != weekStart {
		user.WeekStart = weekStart
		user.ConsumedSec = [7]int64{}
		user.Exhausted = false
		user.DayExhausted = false
		user.StartupWarningSent = false
		user.HalfwayWarningSent = false
		user.FiveMinWarningSent = false
		user.ReenforcementPending = false
		user.ReenforcementDeadline = time.Time{}
		user.LastEnforcementReason = ""
	}
	if distributionSum(user.AllocationsSec) != user.WeeklyAllowanceSec {
		user.AllocationsSec = DefaultDistribution(user.WeeklyAllowanceSec)
	}
	user.RecalculateWeeklyRemaining()
	return user
}

func ValidateDistribution(current model.WeeklyUserState, dist [7]int64) error {
	if distributionSum(dist) != current.WeeklyAllowanceSec {
		return fmt.Errorf("distribution sum must equal weekly allowance")
	}
	capSec := current.WeeklyAllowanceSec / 2
	for idx, value := range dist {
		if value < 0 {
			return fmt.Errorf("day %d allocation cannot be negative", idx)
		}
		if value%SlotSec != 0 {
			return fmt.Errorf("day %d allocation must use 15-minute increments", idx)
		}
		if value > capSec {
			return fmt.Errorf("day %d allocation cannot exceed 50%% of weekly allowance", idx)
		}
		if value < current.ConsumedSec[idx] {
			return fmt.Errorf("day %d allocation cannot be lower than already consumed time", idx)
		}
	}
	return nil
}

func ApplyDistribution(now time.Time, current model.WeeklyUserState, dist [7]int64) (model.WeeklyUserState, error) {
	if err := ValidateDistribution(current, dist); err != nil {
		return model.WeeklyUserState{}, err
	}
	current.AllocationsSec = dist
	day := WeekdayIndex(now)
	if current.WeeklyConsumedSec() < current.WeeklyAllowanceSec && current.ConsumedSec[day] < current.AllocationsSec[day] {
		current.DayExhausted = false
		current.ReenforcementPending = false
		current.ReenforcementDeadline = time.Time{}
	}
	current.RecalculateWeeklyRemaining()
	return current, nil
}

func distributionSum(values [7]int64) int64 {
	var sum int64
	for _, value := range values {
		sum += value
	}
	return sum
}

func countdown() []string {
	return []string{"10", "9", "8", "7", "6", "5", "4", "3", "2", "1"}
}
