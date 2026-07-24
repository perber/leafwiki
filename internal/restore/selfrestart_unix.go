//go:build !windows

package restore

import (
	"os"
	"syscall"
)

// execFn is a seam over syscall.Exec: replacing the process image can't be
// exercised directly in a test without terminating the test binary itself,
// so tests stub this to capture the computed exe/argv/env instead.
var execFn = syscall.Exec

// SelfRestart replaces the current process image in place (same PID) via
// exec(3), re-running main()'s cold boot against whatever is currently on
// disk. Used as the last-resort recovery path when a restore's rollback
// itself fails and the instance may be left in a partially-restored state —
// a fresh process is guaranteed correct by construction (NewWiki() always
// does a full cold boot), unlike trying to programmatically reconcile
// whatever state a failed rollback left behind.
func SelfRestart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return execFn(exe, os.Args, os.Environ())
}
