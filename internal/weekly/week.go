package weekly

import "time"

const SlotSec int64 = 900

func WeekStart(now time.Time) time.Time {
	y, m, d := now.Date()
	loc := now.Location()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, loc)
	offset := (int(midnight.Weekday()) + 6) % 7
	return midnight.AddDate(0, 0, -offset)
}

func WeekdayIndex(now time.Time) int {
	return (int(now.Weekday()) + 6) % 7
}

func DefaultDistribution(allowanceSec int64) [7]int64 {
	var result [7]int64
	if allowanceSec <= 0 {
		return result
	}
	slots := allowanceSec / SlotSec
	for i := int64(0); i < slots; i++ {
		result[int(i%7)] += SlotSec
	}
	return result
}
