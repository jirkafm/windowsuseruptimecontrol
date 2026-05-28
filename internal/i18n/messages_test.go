package i18n

import "testing"

func TestNormalizeLanguageSupportsEnglishAndCzech(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "", want: "en"},
		{input: "en-US", want: "en"},
		{input: "cs-CZ", want: "cs"},
		{input: "de-DE", want: "en"},
	} {
		if got := NormalizeLanguage(tc.input); got != tc.want {
			t.Fatalf("NormalizeLanguage(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSpeechCultureReturnsMicrosoftVoiceCulture(t *testing.T) {
	t.Parallel()

	if got := SpeechCulture("cs"); got != "cs-CZ" {
		t.Fatalf("SpeechCulture(cs) = %q, want cs-CZ", got)
	}
	if got := SpeechCulture("en"); got != "en-US" {
		t.Fatalf("SpeechCulture(en) = %q, want en-US", got)
	}
}

func TestCzechAnnouncementMessages(t *testing.T) {
	t.Parallel()

	if got := DailyRemaining("cs", 12); got != "Zbývá ti 12 minut." {
		t.Fatalf("DailyRemaining(cs) = %q", got)
	}
	if got := WeeklyNoTime("cs", 180); got != "Nezbývá žádný čas. Počítač se uspí za 180 sekund." {
		t.Fatalf("WeeklyNoTime(cs) = %q", got)
	}
}
