package weekly

import (
	"strings"
	"testing"
	"time"

	"windowsuseruptimecontrol/internal/model"
)

func TestEvaluateCreatesWeeklyStateWithSevenHourDefault(t *testing.T) {
	t.Parallel()

	engine := Engine{DefaultWeeklyAllowanceSec: 25200, ReenforcementDelaySec: 180}
	result := engine.Evaluate(
		time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
		model.ActiveUser{Username: "John", UserSID: "sid-john"},
		model.StateFile{Users: map[string]model.UserDayState{}, WeeklyUsers: map[string]model.WeeklyUserState{}},
		1,
	)

	user := result.State.WeeklyUsers["sid-john"]
	if user.WeeklyAllowanceSec != 25200 {
		t.Fatalf("WeeklyAllowanceSec = %d, want 25200", user.WeeklyAllowanceSec)
	}
	if user.WeekStart != "2026-05-11" {
		t.Fatalf("WeekStart = %q, want 2026-05-11", user.WeekStart)
	}
	if user.WeeklyConsumedSec() != 1 {
		t.Fatalf("WeeklyConsumedSec = %d, want 1", user.WeeklyConsumedSec())
	}
}

func TestValidateDistributionRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	current := model.WeeklyUserState{
		WeeklyAllowanceSec: 25200,
		ConsumedSec:        [7]int64{1800, 0, 0, 0, 0, 0, 0},
	}

	cases := []struct {
		name string
		dist [7]int64
		want string
	}{
		{"wrong sum", [7]int64{900, 900, 900, 900, 900, 900, 900}, "sum"},
		{"not increment", [7]int64{1801, 3600, 3600, 3600, 3600, 3600, 5399}, "15-minute"},
		{"over day cap", [7]int64{13500, 1800, 1800, 1800, 1800, 1800, 2700}, "50%"},
		{"below consumed", [7]int64{900, 3600, 3600, 3600, 3600, 3600, 6300}, "already consumed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDistribution(current, tc.dist)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestEvaluateExhaustsCurrentDayBeforeWeek(t *testing.T) {
	t.Parallel()

	engine := Engine{DefaultWeeklyAllowanceSec: 25200, ReenforcementDelaySec: 180}
	state := model.StateFile{
		Users: map[string]model.UserDayState{},
		WeeklyUsers: map[string]model.WeeklyUserState{
			"sid-john": {
				UserSID:            "sid-john",
				Username:           "John",
				WeekStart:          "2026-05-11",
				WeeklyAllowanceSec: 25200,
				AllocationsSec:     [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600},
				ConsumedSec:        [7]int64{0, 3599, 0, 0, 0, 0, 0},
				StartupWarningSent: true,
			},
		},
	}

	result := engine.Evaluate(time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC), model.ActiveUser{Username: "John", UserSID: "sid-john"}, state, 1)

	user := result.State.WeeklyUsers["sid-john"]
	if !user.DayExhausted {
		t.Fatal("DayExhausted = false, want true")
	}
	if user.Exhausted {
		t.Fatal("Exhausted = true, want false because weekly time remains")
	}
	if !result.TriggerEnforcement {
		t.Fatal("TriggerEnforcement = false, want true for day exhaustion")
	}
}

func TestEvaluateExhaustsWholeWeek(t *testing.T) {
	t.Parallel()

	engine := Engine{DefaultWeeklyAllowanceSec: 25200, ReenforcementDelaySec: 180}
	state := model.StateFile{
		Users: map[string]model.UserDayState{},
		WeeklyUsers: map[string]model.WeeklyUserState{
			"sid-john": {
				UserSID:            "sid-john",
				Username:           "John",
				WeekStart:          "2026-05-11",
				WeeklyAllowanceSec: 25200,
				AllocationsSec:     [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3600},
				ConsumedSec:        [7]int64{3600, 3600, 3600, 3600, 3600, 3600, 3599},
				StartupWarningSent: true,
			},
		},
	}

	result := engine.Evaluate(time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC), model.ActiveUser{Username: "John", UserSID: "sid-john"}, state, 1)

	user := result.State.WeeklyUsers["sid-john"]
	if !user.Exhausted {
		t.Fatal("Exhausted = false, want true")
	}
	if user.LastEnforcementReason != "weekly allowance exhausted" {
		t.Fatalf("LastEnforcementReason = %q, want weekly allowance exhausted", user.LastEnforcementReason)
	}
}
