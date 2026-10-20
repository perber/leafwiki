package usersettings

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestAllowedLanguages_MatchShippedUILocales guards the allowlist against
// drifting from the frontend: every locale folder the UI ships must be
// accepted here, and nothing else.
func TestAllowedLanguages_MatchShippedUILocales(t *testing.T) {
	localesDir := filepath.Join("..", "..", "ui", "leafwiki-ui", "src", "locales")
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		t.Fatalf("read %s: %v", localesDir, err)
	}
	var shipped []string
	for _, e := range entries {
		if e.IsDir() {
			shipped = append(shipped, e.Name())
		}
	}
	sort.Strings(shipped)

	if got := AllowedLanguages(); !reflect.DeepEqual(got, shipped) {
		t.Fatalf("AllowedLanguages() = %v, want the locale folders in %s: %v", got, localesDir, shipped)
	}
}

func TestAllowedLanguages_ReturnsSortedShippedCodes(t *testing.T) {
	got := AllowedLanguages()

	want := []string{"de", "en", "es", "fr", "zh"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestIsAllowedLanguage_AcceptsShippedCodesAndRejectsUnknownOnes(t *testing.T) {
	for _, lang := range []string{"en", "de", "es", "fr", "zh"} {
		if !IsAllowedLanguage(lang) {
			t.Errorf("expected %q to be allowed", lang)
		}
	}
	if IsAllowedLanguage("xx-not-a-real-language") {
		t.Error("expected an unshipped language code to be rejected")
	}
}
