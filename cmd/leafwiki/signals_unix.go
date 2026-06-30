//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyReloadSignals(ch chan os.Signal) {
	signal.Notify(ch, syscall.SIGUSR1, syscall.SIGHUP)
}
