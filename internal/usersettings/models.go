// Package usersettings stores each user's private preferences (language,
// autosave, ...). Like favorites, this data is not derived from the
// filesystem tree and must never be touched by resync (see ADR-0001).
package usersettings

import "time"

// DefaultLanguage is the language assigned to a user who has never set one.
const DefaultLanguage = "en"

// UserSettings holds a single user's preferences.
type UserSettings struct {
	UserID    string    `json:"userId"`
	Language  string    `json:"language"`
	AutoSave  bool      `json:"autoSave"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// DefaultUserSettings returns the settings a user has before ever saving any
// preferences — never persisted by itself, only returned in-memory by Get.
func DefaultUserSettings(userID string) *UserSettings {
	return &UserSettings{
		UserID:   userID,
		Language: DefaultLanguage,
		AutoSave: true,
	}
}
