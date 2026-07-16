//go:build !windows

package main

import (
	"context"
	"os/exec"
	"os/signal"
	"syscall"
)

func setChildAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopProcessTree(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
