//go:build windows

package main

import "os"

func notifyReloadSignals(_ chan os.Signal) {
	// SIGUSR1 and SIGHUP do not exist on Windows; live reload via signals is unavailable.
}
