//go:build windows

package restore

import (
	"os"
	"os/exec"
)

// SelfRestart spawns a fresh copy of the current process with the same
// arguments and environment, then exits the current one. Windows has no
// exec(3) equivalent that replaces the running process image in place, so
// this uses spawn-then-exit instead — mirrors the
// signals_unix.go/signals_windows.go OS split already used in cmd/leafwiki
// for other process-lifecycle code.
func SelfRestart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}

	os.Exit(0)
	return nil // unreachable
}
