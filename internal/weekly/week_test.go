package weekly

import (
	"testing"
	"time"
)

func TestWeekStartReturnsMonday(t *testing.T) {
	t.Parallel()

	got := WeekStart(time.Date(2026, 5, 17, 18, 0, 0, 0, time.UTC))
	want := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("WeekStart = %s, want %s", got, want)
	}
}

func TestWeekdayIndexUsesMondayAsZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		date time.Time
		want int
	}{
		{"monday", time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC), 0},
		{"sunday", time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC), 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WeekdayIndex(tc.date); got != tc.want {
				t.Fatalf("WeekdayIndex = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDefaultDistributionSumsToAllowanceAndUsesFifteenMinuteSlots(t *testing.T) {
	t.Parallel()

	got := DefaultDistribution(25200)
	var sum int64
	for idx, value := range got {
		if value%900 != 0 {
			t.Fatalf("allocation[%d] = %d, want 15-minute increment", idx, value)
		}
		if value > 12600 {
			t.Fatalf("allocation[%d] = %d, want <= 50%% weekly allowance", idx, value)
		}
		sum += value
	}
	if sum != 25200 {
		t.Fatalf("sum = %d, want 25200", sum)
	}
}
