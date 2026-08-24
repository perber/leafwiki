package usersettings

import (
	"reflect"
	"testing"
)

func TestAllowedLanguages_ReturnsSortedShippedCodes(t *testing.T) {
	got := AllowedLanguages()
	want := []string{"de", "en"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestIsAllowedLanguage_AcceptsShippedCodesAndRejectsUnknownOnes(t *testing.T) {
	for _, lang := range []string{"en", "de"} {
		if !IsAllowedLanguage(lang) {
			t.Errorf("expected %q to be allowed", lang)
		}
	}
	if IsAllowedLanguage("xx-not-a-real-language") {
		t.Error("expected an unshipped language code to be rejected")
	}
}
