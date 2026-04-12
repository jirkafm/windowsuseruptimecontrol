package policy

import (
	"testing"
	"time"

	"windowsuseruptimecontrol/internal/model"
)

func TestEvaluateConsumesActiveUserAndEmitsWarnings(t *testing.T) {
	t.Parallel()

	engine := Engine{DefaultDailyAllowanceSec: 3600, ReenforcementDelaySec: 180}
	state := model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"sid-john": {
				UserSID:              "sid-john",
				Username:             "John",
				Date:                 "2026-04-01",
				DailyAllowanceSec:    3600,
				ConsumedSec:          1790,
				RemainingSec:         1810,
				StartupWarningSent:   true,
				HalfwayWarningSent:   false,
				FiveMinWarningSent:   false,
				ReenforcementPending: false,
			},
		},
	}

	result := engine.Evaluate(
		time.Date(2026, 4, 1, 10, 0, 10, 0, time.UTC),
		model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"},
		state,
		20,
	)

	user := result.State.Users["sid-john"]
	if user.ConsumedSec != 1810 {
		t.Fatalf("ConsumedSec = %d, want 1810", user.ConsumedSec)
	}
	if user.RemainingSec != 1790 {
		t.Fatalf("RemainingSec = %d, want 1790", user.RemainingSec)
	}
	if len(result.Messages) != 1 || result.Messages[0] != "You have 30 minutes remaining." {
		t.Fatalf("Messages = %#v, want halfway announcement", result.Messages)
	}
	if !user.HalfwayWarningSent {
		t.Fatalf("HalfwayWarningSent = false, want true")
	}
}

func TestEvaluateCreatesReenforcementDeadlineForSameDayReturn(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	engine := Engine{DefaultDailyAllowanceSec: 3600, ReenforcementDelaySec: 180}
	state := model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"sid-john": {
				UserSID:              "sid-john",
				Username:             "John",
				Date:                 "2026-04-01",
				DailyAllowanceSec:    3600,
				ConsumedSec:          3600,
				RemainingSec:         0,
				Exhausted:            true,
				ReenforcementPending: false,
			},
		},
	}

	result := engine.Evaluate(now, model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, state, 0)

	user := result.State.Users["sid-john"]
	if !user.ReenforcementPending {
		t.Fatalf("ReenforcementPending = false, want true")
	}
	if got := user.ReenforcementDeadline.Sub(now); got != 180*time.Second {
		t.Fatalf("ReenforcementDeadline delta = %v, want 180s", got)
	}
	if len(result.Messages) != 1 || result.Messages[0] != "No time remains for today. The computer will hibernate in 180 seconds." {
		t.Fatalf("Messages = %#v", result.Messages)
	}
}

func TestEvaluateTriggersEnforcementCountdown(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 1, 12, 3, 1, 0, time.UTC)
	engine := Engine{DefaultDailyAllowanceSec: 3600, ReenforcementDelaySec: 180}
	state := model.StateFile{
		ServiceDate: "2026-04-01",
		Users: map[string]model.UserDayState{
			"sid-john": {
				UserSID:               "sid-john",
				Username:              "John",
				Date:                  "2026-04-01",
				DailyAllowanceSec:     3600,
				ConsumedSec:           3600,
				RemainingSec:          0,
				Exhausted:             true,
				ReenforcementPending:  true,
				ReenforcementDeadline: time.Date(2026, 4, 1, 12, 3, 0, 0, time.UTC),
			},
		},
	}

	result := engine.Evaluate(now, model.ActiveUser{SessionID: 1, Username: "John", UserSID: "sid-john"}, state, 0)

	if !result.TriggerEnforcement {
		t.Fatalf("TriggerEnforcement = false, want true")
	}
	if len(result.Countdown) != 10 || result.Countdown[0] != "10" || result.Countdown[9] != "1" {
		t.Fatalf("Countdown = %#v, want 10..1", result.Countdown)
	}
}
