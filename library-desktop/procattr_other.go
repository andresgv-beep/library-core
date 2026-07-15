//go:build !windows

package main

import "os/exec"

// En Linux/macOS no hay ventanas de consola que evitar.
func noWindow(_ *exec.Cmd) {}

// superviseChild: en Linux/macOS el cierre limpio (OnShutdown) basta por ahora;
// la equivalencia al Job Object (prctl PR_SET_PDEATHSIG / process groups) queda
// para el port a Linux.
func superviseChild(_ *exec.Cmd) {}
