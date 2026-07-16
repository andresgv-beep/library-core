//go:build windows

package main

import (
	"context"
	"fmt"
	"os/exec"
	"os/signal"
	"syscall"
)

func setChildAttributes(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
}

func stopProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprint(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_ = kill.Run()
	_ = cmd.Process.Kill()
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}
