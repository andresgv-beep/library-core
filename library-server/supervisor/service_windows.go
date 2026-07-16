//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type serviceHandler struct{ supervisor *supervisor }

func runPlatform(supervisor *supervisor) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		return runConsole(supervisor)
	}
	return svc.Run(serviceName, &serviceHandler{supervisor: supervisor})
}

func (h *serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.supervisor.run(ctx) }()
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	statuses <- current

	for {
		select {
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				statuses <- current
			case svc.Stop, svc.Shutdown:
				statuses <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		case err := <-done:
			cancel()
			if err != nil {
				return true, 1
			}
			return false, 0
		}
	}
}

func handleServiceCommand(command string) error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("conectar con Service Manager (ejecuta como administrador): %w", err)
	}
	defer manager.Disconnect()

	switch strings.ToLower(command) {
	case "install":
		if existing, openErr := manager.OpenService(serviceName); openErr == nil {
			existing.Close()
			configureRecovery()
			return nil
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		service, err := manager.CreateService(serviceName, executable, mgr.Config{
			DisplayName: "Nimos Library Server",
			Description: "Supervisa Library Server y lo recupera automaticamente.",
			StartType:   mgr.StartAutomatic,
		}, "run")
		if err != nil {
			return err
		}
		service.Close()
		configureRecovery()
		return nil
	case "uninstall":
		service, err := manager.OpenService(serviceName)
		if err != nil {
			return err
		}
		defer service.Close()
		_, _ = service.Control(svc.Stop)
		waitService(service, svc.Stopped, 15*time.Second)
		return service.Delete()
	case "start":
		service, err := manager.OpenService(serviceName)
		if err != nil {
			return err
		}
		defer service.Close()
		if status, queryErr := service.Query(); queryErr == nil && status.State == svc.Running {
			return nil
		}
		return service.Start()
	case "stop":
		service, err := manager.OpenService(serviceName)
		if err != nil {
			return err
		}
		defer service.Close()
		if status, queryErr := service.Query(); queryErr == nil && status.State == svc.Stopped {
			return nil
		}
		if _, err = service.Control(svc.Stop); err != nil {
			return err
		}
		return waitService(service, svc.Stopped, 30*time.Second)
	case "restart":
		service, err := manager.OpenService(serviceName)
		if err != nil {
			return err
		}
		defer service.Close()
		_, _ = service.Control(svc.Stop)
		if err := waitService(service, svc.Stopped, 30*time.Second); err != nil {
			return err
		}
		return service.Start()
	case "status":
		service, err := manager.OpenService(serviceName)
		if err != nil {
			return err
		}
		defer service.Close()
		status, err := service.Query()
		if err != nil {
			return err
		}
		fmt.Println(status.State)
		return nil
	default:
		return fmt.Errorf("comando desconocido %q (install, uninstall, start, stop, restart, status)", command)
	}
}

func waitService(service *mgr.Service, expected svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == expected {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timeout esperando estado %v", expected)
}

func configureRecovery() {
	command := exec.Command("sc.exe", "failure", serviceName, "reset=", "86400", "actions=", "restart/5000/restart/15000/restart/60000")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	_ = command.Run()
}
