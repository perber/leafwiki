package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// legacyBoolFlagNames are boolean flags that predate the enable-/disable-/
// allow- scheme in docs/development/conventions.md#naming. Renaming them would
// break existing deployments, so they stay; this list may only shrink.
var legacyBoolFlagNames = []string{
	"git-backup",
	"hide-link-metadata-section",
	"public-access",
	"smtp-insecure-skip-verify",
	"snapshot",
}

var boolFlagPrefixes = []string{"enable-", "disable-", "allow-"}

// TestServerFlags_EnvVarIsLeafwikiPlusFlagName checks the naming scheme for
// every server flag: exactly one environment variable, LEAFWIKI_ + the flag in
// upper snake case.
func TestServerFlags_EnvVarIsLeafwikiPlusFlagName(t *testing.T) {
	for _, flag := range (&serverConfig{}).flags() {
		name := flag.Names()[0]
		t.Run(name, func(t *testing.T) {
			envFlag, ok := flag.(cli.DocGenerationFlag)
			if !ok {
				t.Fatalf("flag --%s (%T) does not expose its environment variables", name, flag)
			}
			want := "LEAFWIKI_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
			if got := envFlag.GetEnvVars(); !slices.Equal(got, []string{want}) {
				t.Errorf("flag --%s env vars = %v, want [%s]", name, got, want)
			}
		})
	}
}

// TestServerFlags_BoolFlagsUseOptInOrOptOutPrefix checks that boolean flags
// say which way they switch: enable-, disable- or allow-.
func TestServerFlags_BoolFlagsUseOptInOrOptOutPrefix(t *testing.T) {
	for _, flag := range (&serverConfig{}).flags() {
		if _, isBool := flag.(*cli.BoolFlag); !isBool {
			continue
		}
		name := flag.Names()[0]
		t.Run(name, func(t *testing.T) {
			hasPrefix := slices.ContainsFunc(boolFlagPrefixes, func(p string) bool {
				return strings.HasPrefix(name, p)
			})
			isLegacy := slices.Contains(legacyBoolFlagNames, name)
			switch {
			case !hasPrefix && !isLegacy:
				t.Errorf("bool flag --%s must start with one of %v (conventions.md#naming)", name, boolFlagPrefixes)
			case hasPrefix && isLegacy:
				t.Errorf("bool flag --%s follows the scheme now; remove it from legacyBoolFlagNames", name)
			}
		})
	}
}

// TestLegacyBoolFlagNames_NameExistingBoolFlags keeps the allowlist from outliving the
// flags it excuses, and from excusing anything but a bool flag.
func TestLegacyBoolFlagNames_NameExistingBoolFlags(t *testing.T) {
	isBool := map[string]bool{}
	for _, flag := range (&serverConfig{}).flags() {
		_, isBool[flag.Names()[0]] = flag.(*cli.BoolFlag)
	}
	for _, legacy := range legacyBoolFlagNames {
		isBoolFlag, exists := isBool[legacy]
		switch {
		case !exists:
			t.Errorf("legacyBoolFlagNames lists --%s, which no longer exists; remove it", legacy)
		case !isBoolFlag:
			t.Errorf("legacyBoolFlagNames lists --%s, which is not a bool flag; remove it", legacy)
		}
	}
}
