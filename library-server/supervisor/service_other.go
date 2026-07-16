//go:build !windows

package main

import "fmt"

func runPlatform(supervisor *supervisor) error { return runConsole(supervisor) }

func handleServiceCommand(command string) error {
	return fmt.Errorf("%q solo esta integrado en Windows; usa systemd/Docker para ejecutar library-supervisor run", command)
}
