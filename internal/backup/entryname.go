package backup

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unicode"
)

// errInvalidTreeEntry marks a tree entry name that the backup does not write to
// disk: git itself refuses such names in a worktree, and the pull materializes
// trees without go-git's worktree checks, so it applies the same rules here.
var errInvalidTreeEntry = errors.New("invalid tree entry name")

// validateTreeEntryName rejects a single tree entry name the backup does not
// write to disk. It is the one entry point for both the pull and the commit
// side, so the two always agree.
//
// On every OS it applies go-git's internal pathutil.ValidTreePath (which
// cannot be imported) per entry: control characters, empty, "." and "..",
// separators of any platform, and .git with its NTFS and HFS+ aliases. go-git
// refuses to push such names, so the local history never holds them and a
// base tree always passes.
//
// On Windows it also applies git for Windows' is_valid_win32_path: the
// characters Windows does not allow, a trailing dot or space, and device
// names. These are valid elsewhere (an upload keeps its raw extension, and
// device names are valid page slugs), so they may already be in an existing
// backup; refusing them there would make every later pull of it fail.
//
// The name is quoted in the error so control characters stay readable in logs
// and status.
func validateTreeEntryName(name string) error {
	reason := gitTreeNameProblem(name)
	if reason == "" && runtime.GOOS == "windows" {
		reason = windowsNameProblem(name)
	}
	if reason == "" {
		return nil
	}
	return fmt.Errorf("%w %q: %s", errInvalidTreeEntry, name, reason)
}

// gitTreeNameProblem returns why git refuses name as a tree entry, or "".
func gitTreeNameProblem(name string) string {
	switch {
	case name == "":
		return "empty"
	case name == "." || name == "..":
		return "relative path component"
	case strings.ContainsAny(name, `/\`):
		return "contains a path separator"
	case strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }):
		return "contains a control character"
	case isNTFSDotGit(name) || isHFSDotGit(name):
		return "refers to .git"
	}
	return ""
}

// windowsNameProblem returns why Windows cannot store name as a file, or "".
func windowsNameProblem(name string) string {
	switch {
	case strings.ContainsAny(name, `:<>"|?*`):
		return "contains a character Windows does not allow"
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return "ends with a dot or space, which Windows drops"
	case isWindowsReservedName(name):
		return "is a Windows device name"
	}
	return ""
}

// windowsReservedNames are the device names Windows resolves in every
// directory, whatever extension follows them (git for Windows'
// is_valid_win32_path).
var windowsReservedNames = func() map[string]struct{} {
	names := map[string]struct{}{"CON": {}, "PRN": {}, "AUX": {}, "NUL": {}, "CONIN$": {}, "CONOUT$": {}}
	for _, digit := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "\u00b9", "\u00b2", "\u00b3"} {
		names["COM"+digit] = struct{}{}
		names["LPT"+digit] = struct{}{}
	}
	return names
}()

// isWindowsReservedName reports whether Windows resolves name to a device, as
// for "NUL", "nul.txt" or "COM1 .md": the part before the first dot counts,
// case-insensitively and without trailing spaces.
func isWindowsReservedName(name string) bool {
	base, _, _ := strings.Cut(name, ".")
	_, reserved := windowsReservedNames[strings.ToUpper(strings.TrimRight(base, " "))]
	return reserved
}

// isNTFSDotGit reports whether NTFS resolves name to ".git": ".git" or its 8.3
// short name "git~1", case-insensitively, followed by any trailing spaces or
// periods (NTFS drops them) and an optional ":<stream>" suffix.
func isNTFSDotGit(name string) bool {
	var rest string
	switch {
	case len(name) >= 4 && strings.EqualFold(name[:4], ".git"):
		rest = name[4:]
	case len(name) >= 5 && strings.EqualFold(name[:5], "git~1"):
		rest = name[5:]
	default:
		return false
	}
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case ':':
			return true
		case '.', ' ':
		default:
			return false
		}
	}
	return true
}

// hfsIgnoredCodepoints are the code points HFS+ drops when it normalizes a
// name, so ".g‌it" is ".git" there.
var hfsIgnoredCodepoints = map[rune]struct{}{
	0x200c: {}, 0x200d: {}, 0x200e: {}, 0x200f: {},
	0x202a: {}, 0x202b: {}, 0x202c: {}, 0x202d: {}, 0x202e: {},
	0x206a: {}, 0x206b: {}, 0x206c: {}, 0x206d: {}, 0x206e: {}, 0x206f: {},
	0xfeff: {},
}

// isHFSDotGit reports whether HFS+ resolves name to ".git" after dropping
// ignored code points and folding ASCII case.
func isHFSDotGit(name string) bool {
	var folded []rune
	for _, r := range name {
		if _, ignored := hfsIgnoredCodepoints[r]; ignored {
			continue
		}
		if r > unicode.MaxASCII {
			return false
		}
		folded = append(folded, unicode.ToLower(r))
	}
	return string(folded) == ".git"
}
