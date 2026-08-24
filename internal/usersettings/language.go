package usersettings

import "sort"

// allowedLanguages is the set of language codes UserSettings will accept.
// The frontend only ships an "en" locale today (see ui/leafwiki-ui/src/lib/i18n.ts),
// so this list is deliberately just the default for now — extend it here
// once more locales are registered.
var allowedLanguages = map[string]bool{
	DefaultLanguage: true,
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
