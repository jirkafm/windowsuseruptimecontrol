package i18n

import (
	"fmt"
	"strings"
)

const (
	English = "en"
	Czech   = "cs"
)

func NormalizeLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.ReplaceAll(language, "_", "-")
	if language == Czech || strings.HasPrefix(language, Czech+"-") {
		return Czech
	}
	return English
}

func SpeechCulture(language string) string {
	if NormalizeLanguage(language) == Czech {
		return "cs-CZ"
	}
	return "en-US"
}

func DailyRemaining(language string, minutes int64) string {
	if NormalizeLanguage(language) == Czech {
		return fmt.Sprintf("Zbývá ti %d minut.", minutes)
	}
	return fmt.Sprintf("You have %d minutes remaining.", minutes)
}

func DailyHalfway(language string) string {
	if NormalizeLanguage(language) == Czech {
		return "Zbývá ti 30 minut."
	}
	return "You have 30 minutes remaining."
}

func DailyFiveMinutes(language string) string {
	if NormalizeLanguage(language) == Czech {
		return "Zbývá ti 5 minut."
	}
	return "You have 5 minutes remaining."
}

func DailyNoTime(language string, delaySec int64) string {
	if NormalizeLanguage(language) == Czech {
		return fmt.Sprintf("Pro dnešek nezbývá žádný čas. Počítač se uspí za %d sekund.", delaySec)
	}
	return fmt.Sprintf("No time remains for today. The computer will hibernate in %d seconds.", delaySec)
}

func WeeklyRemaining(language string, minutes int64) string {
	if NormalizeLanguage(language) == Czech {
		return fmt.Sprintf("Tento týden ti zbývá %d minut.", minutes)
	}
	return fmt.Sprintf("You have %d minutes remaining this week.", minutes)
}

func WeeklyHalfway(language string) string {
	if NormalizeLanguage(language) == Czech {
		return "Byla vyčerpána polovina týdenního času."
	}
	return "You have used half of your weekly time."
}

func WeeklyFiveMinutes(language string) string {
	if NormalizeLanguage(language) == Czech {
		return "Tento týden ti zbývá 5 minut."
	}
	return "You have 5 minutes remaining this week."
}

func WeeklyNoTime(language string, delaySec int64) string {
	if NormalizeLanguage(language) == Czech {
		return fmt.Sprintf("Nezbývá žádný čas. Počítač se uspí za %d sekund.", delaySec)
	}
	return fmt.Sprintf("No time remains. The computer will hibernate in %d seconds.", delaySec)
}

func AllowanceChanged(language string, minutes int64) string {
	if NormalizeLanguage(language) == Czech {
		return fmt.Sprintf("Zbývající čas se změnil. Zbývá ti %d minut.", minutes)
	}
	return fmt.Sprintf("Your remaining time has changed. You have %d minutes remaining.", minutes)
}
