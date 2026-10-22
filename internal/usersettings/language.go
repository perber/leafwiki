package usersettings

import "sort"

// allowedLanguages is the set of language codes UserSettings will accept.
// It must equal the locale folders under ui/leafwiki-ui/src/locales/;
// TestAllowedLanguages_MatchShippedUILocales fails when they drift.
var allowedLanguages = map[string]bool{
	DefaultLanguage: true,
	"de":            true,
	"es":            true,
	"fr":            true,
	"zh":            true,
}

// IsAllowedLanguage reports whether lang is a language UserSettings accepts.
func IsAllowedLanguage(lang string) bool {
	return allowedLanguages[lang]
}

// AllowedLanguages returns the accepted language codes, sorted for stable output.
func AllowedLanguages() []string {
	langs := make([]string, 0, len(allowedLanguages))
	for lang := range allowedLanguages {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}
