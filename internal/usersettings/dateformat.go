package usersettings

import "sort"

// allowedDateFormats is the set of date-format identifiers UserSettings will
// accept. Keep in sync with the options offered by the Account → Preferences
// picker in ui/leafwiki-ui. "locale" means "follow the active UI language".
var allowedDateFormats = map[string]bool{
	DefaultDateFormat: true, // locale
	"iso":             true, // 2026-08-27
	"dmy_dot":         true, // 27.08.2026
	"mdy_slash":       true, // 08/27/2026
	"dmy_slash":       true, // 27/08/2026
}

// allowedTimeFormats is the set of time-format identifiers UserSettings will
// accept. "locale" means "follow the active UI language".
var allowedTimeFormats = map[string]bool{
	DefaultTimeFormat: true, // locale
	"24h":             true, // 14:30
	"12h":             true, // 2:30 PM
}

// IsAllowedDateFormat reports whether format is a date format UserSettings accepts.
func IsAllowedDateFormat(format string) bool {
	return allowedDateFormats[format]
}

// IsAllowedTimeFormat reports whether format is a time format UserSettings accepts.
func IsAllowedTimeFormat(format string) bool {
	return allowedTimeFormats[format]
}

// AllowedDateFormats returns the accepted date-format identifiers, sorted.
func AllowedDateFormats() []string {
	return sortedKeys(allowedDateFormats)
}

// AllowedTimeFormats returns the accepted time-format identifiers, sorted.
func AllowedTimeFormats() []string {
	return sortedKeys(allowedTimeFormats)
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
