package usersettings

import "testing"

func TestIsAllowedDateFormat(t *testing.T) {
	for _, ok := range []string{"locale", "iso", "dmy_dot", "mdy_slash", "dmy_slash"} {
		if !IsAllowedDateFormat(ok) {
			t.Errorf("expected %q to be an allowed date format", ok)
		}
	}
	for _, bad := range []string{"", "LOCALE", "dd.MM.yyyy", "ymd", "nonsense"} {
		if IsAllowedDateFormat(bad) {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
}

func TestIsAllowedTimeFormat(t *testing.T) {
	for _, ok := range []string{"locale", "24h", "12h"} {
		if !IsAllowedTimeFormat(ok) {
			t.Errorf("expected %q to be an allowed time format", ok)
		}
	}
	for _, bad := range []string{"", "24", "HH:mm", "am/pm"} {
		if IsAllowedTimeFormat(bad) {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
}

func TestAllowedFormats_AreSorted(t *testing.T) {
	dates := AllowedDateFormats()
	for i := 1; i < len(dates); i++ {
		if dates[i-1] > dates[i] {
			t.Fatalf("AllowedDateFormats not sorted: %v", dates)
		}
	}
	times := AllowedTimeFormats()
	for i := 1; i < len(times); i++ {
		if times[i-1] > times[i] {
			t.Fatalf("AllowedTimeFormats not sorted: %v", times)
		}
	}
}

func TestDefaultUserSettings_HasLocaleFormats(t *testing.T) {
	got := DefaultUserSettings("user-1")
	if got.DateFormat != DefaultDateFormat || got.TimeFormat != DefaultTimeFormat {
		t.Fatalf("expected locale formats by default, got %+v", got)
	}
}
