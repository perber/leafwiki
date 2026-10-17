package backup

import (
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestValidateTreeEntryName_Names_RejectsNonPortable tests which tree entry
// names from the backup remote may be written into a live content directory:
// names git refuses on every OS, names Windows cannot store only on Windows.
func TestValidateTreeEntryName_Names_RejectsNonPortable(t *testing.T) {
	onWindows := runtime.GOOS == "windows"
	tests := []struct {
		name    string
		entry   string
		wantErr bool
	}{
		{name: "plain file", entry: "page.md"},
		{name: "dotfile", entry: ".gitignore"},
		{name: "dot git prefix", entry: ".github"},
		{name: "short name other than git~1", entry: "git~2"},
		{name: "unicode", entry: "\u00fcber \u00f1.md"},
		{name: "inner dots", entry: "a..b.md"},
		{name: "empty", entry: "", wantErr: true},
		{name: "dot", entry: ".", wantErr: true},
		{name: "dot dot", entry: "..", wantErr: true},
		{name: "slash", entry: "a/b", wantErr: true},
		{name: "slash dot dot", entry: "../outside", wantErr: true},
		{name: "backslash dot dot", entry: `..\outside`, wantErr: true},
		{name: "backslash dot dot in the middle", entry: `x\..\outside`, wantErr: true},
		{name: "backslash", entry: `a\b`, wantErr: true},
		{name: "NUL", entry: "a\x00b", wantErr: true},
		{name: "newline", entry: "a\nb", wantErr: true},
		{name: "DEL", entry: "a\x7fb", wantErr: true},
		{name: "dot git", entry: ".git", wantErr: true},
		{name: "dot git upper case", entry: ".GIT", wantErr: true},
		{name: "NTFS short name", entry: "git~1", wantErr: true},
		{name: "NTFS short name upper case", entry: "GIT~1", wantErr: true},
		{name: "NTFS trailing space", entry: ".git ", wantErr: true},
		{name: "NTFS trailing dot", entry: ".git.", wantErr: true},
		{name: "NTFS alternate data stream", entry: ".git::$INDEX_ALLOCATION", wantErr: true},
		{name: "NTFS short name trailing dot", entry: "git~1.", wantErr: true},
		{name: "HFS ignored code point", entry: ".g\u200cit", wantErr: true},
		{name: "HFS leading ignored code point", entry: "\ufeff.GIT", wantErr: true},
		{name: "dot dot prefix", entry: "..foo"},
		{name: "drive letter", entry: "C:", wantErr: onWindows},
		{name: "colon", entry: "a:b", wantErr: onWindows},
		{name: "less than", entry: "a<b", wantErr: onWindows},
		{name: "greater than", entry: "a>b", wantErr: onWindows},
		{name: "double quote", entry: `a"b`, wantErr: onWindows},
		{name: "pipe", entry: "a|b", wantErr: onWindows},
		{name: "question mark", entry: "a?b", wantErr: onWindows},
		{name: "asterisk", entry: "a*b", wantErr: onWindows},
		{name: "trailing dot", entry: "page.", wantErr: onWindows},
		{name: "trailing space", entry: "page.md ", wantErr: onWindows},
		{name: "device name, refused on Windows only", entry: "aux.md", wantErr: onWindows},
		{name: "device name directory, refused on Windows only", entry: "CON", wantErr: onWindows},
		{name: "device name prefix", entry: "auxiliary.md"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTreeEntryName(tc.entry)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateTreeEntryName(%q) error = %v, wantErr %v", tc.entry, err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), strconv.Quote(tc.entry)) {
				t.Errorf("error %q does not name the entry as %s", err, strconv.Quote(tc.entry))
			}
		})
	}
}

// TestIsWindowsReservedName_Names_DetectsDeviceNames tests which names Windows
// resolves to a device, whatever OS the test runs on.
func TestIsWindowsReservedName_Names_DetectsDeviceNames(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		want  bool
	}{
		{name: "CON", entry: "CON", want: true},
		{name: "PRN", entry: "PRN", want: true},
		{name: "AUX", entry: "AUX", want: true},
		{name: "NUL", entry: "NUL", want: true},
		{name: "CONIN$", entry: "CONIN$", want: true},
		{name: "CONOUT$", entry: "CONOUT$", want: true},
		{name: "COM0", entry: "COM0", want: true},
		{name: "COM9", entry: "COM9", want: true},
		{name: "LPT0", entry: "LPT0", want: true},
		{name: "LPT9", entry: "LPT9", want: true},
		{name: "superscript one", entry: "COM\u00b9", want: true},
		{name: "superscript two", entry: "LPT\u00b2", want: true},
		{name: "superscript three", entry: "com\u00b3.txt", want: true},
		{name: "lower case", entry: "nul", want: true},
		{name: "mixed case", entry: "Aux", want: true},
		{name: "with extension", entry: "NUL.txt", want: true},
		{name: "with two extensions", entry: "con.tar.gz", want: true},
		{name: "space before extension", entry: "COM1 .md", want: true},
		{name: "longer name", entry: "console.md"},
		{name: "prefix", entry: "auxiliary"},
		{name: "COM without digit", entry: "COM"},
		{name: "COM two digits", entry: "COM10"},
		{name: "superscript four", entry: "COM\u2074"},
		{name: "device name after a dot", entry: "x.con"},
		{name: "plain page", entry: "page.md"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isWindowsReservedName(tc.entry); got != tc.want {
				t.Errorf("isWindowsReservedName(%q) = %v, want %v", tc.entry, got, tc.want)
			}
		})
	}
}
