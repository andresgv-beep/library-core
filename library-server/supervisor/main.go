package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	serviceName     = "NimosLibraryServer"
	restartExitCode = 75
)

type supervisor struct {
	binDir string
	logDir string

	mu       sync.Mutex
	children map[string]*exec.Cmd
}

func main() {
	if len(os.Args) > 1 && os.Args[1] != "run" {
		if err := handleServiceCommand(os.Args[1]); err != nil {
			log.Fatal(err)
		}
		return
	}

	logPath, err := supervisorLogPath()
	if err == nil {
		if file, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); openErr == nil {
			defer file.Close()
			log.SetOutput(file)
		}
	}

	s, err := newSupervisor()
	if err != nil {
		log.Fatal(err)
	}
	if err := runPlatform(s); err != nil {
		log.Fatal(err)
	}
}

func newSupervisor() (*supervisor, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	logDir, err := supervisorDataDir("logs")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	return &supervisor{binDir: filepath.Dir(exe), logDir: logDir, children: map[string]*exec.Cmd{}}, nil
}

func (s *supervisor) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var workers sync.WaitGroup

	if translate := findExecutable(s.binDir, "translate-wrap"); translate != "" {
		workers.Add(1)
		go func() {
			defer workers.Done()
			s.runLoop(ctx, "translate", translate, s.translateEnv(), false)
		}()
	}

	core := findExecutable(s.binDir, "library-server", "core")
	if core == "" {
		return fmt.Errorf("no se encontro library-server junto a %s", s.binDir)
	}
	s.runLoop(ctx, "core", core, s.coreEnv(), true)
	cancel()
	workers.Wait()
	return nil
}

func (s *supervisor) runLoop(ctx context.Context, name, executable string, env []string, core bool) {
	delay := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		started := time.Now()
		cmd := exec.Command(executable)
		cmd.Dir = s.binDir
		cmd.Env = env
		setChildAttributes(cmd)
		var output *os.File
		if file, err := s.processLog(name); err == nil {
			output = file
			cmd.Stdout, cmd.Stderr = output, output
		}
		if err := cmd.Start(); err != nil {
			log.Printf("%s no pudo arrancar: %v", name, err)
			if output != nil {
				output.Close()
			}
		} else {
			s.setChild(name, cmd)
			log.Printf("%s iniciado pid=%d", name, cmd.Process.Pid)
			wait := make(chan error, 1)
			go func() { wait <- cmd.Wait() }()
			select {
			case <-ctx.Done():
				stopProcessTree(cmd)
				select {
				case <-wait:
				case <-time.After(10 * time.Second):
					_ = cmd.Process.Kill()
					<-wait
				}
				s.clearChild(name, cmd)
				return
			case err := <-wait:
				s.clearChild(name, cmd)
				exitCode := commandExitCode(err)
				if core && exitCode == restartExitCode {
					log.Print("reinicio administrativo solicitado")
					delay = 300 * time.Millisecond
				} else {
					log.Printf("%s termino (codigo=%d, uptime=%s): %v", name, exitCode, time.Since(started).Round(time.Second), err)
				}
			}
			if output != nil {
				output.Close()
			}
		}

		if time.Since(started) > 30*time.Second {
			delay = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 30*time.Second {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}
}

func (s *supervisor) setChild(name string, cmd *exec.Cmd) {
	s.mu.Lock()
	s.children[name] = cmd
	s.mu.Unlock()
}

func (s *supervisor) clearChild(name string, cmd *exec.Cmd) {
	s.mu.Lock()
	if s.children[name] == cmd {
		delete(s.children, name)
	}
	s.mu.Unlock()
}

func (s *supervisor) processLog(name string) (*os.File, error) {
	return os.OpenFile(filepath.Join(s.logDir, name+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func (s *supervisor) coreEnv() []string {
	extra := map[string]string{"LIBRARY_SUPERVISED": "1"}
	if os.Getenv("POOL_ROOT") == "" {
		if root, err := supervisorDataDir("data"); err == nil {
			extra["POOL_ROOT"] = root
		}
	}
	if os.Getenv("TRANSLATE_URL") == "" && findExecutable(s.binDir, "translate-wrap") != "" {
		extra["TRANSLATE_URL"] = "http://127.0.0.1:" + env("TRANSLATE_PORT", "8091")
	}
	return mergeEnv(os.Environ(), extra)
}

func (s *supervisor) translateEnv() []string {
	return mergeEnv(os.Environ(), map[string]string{
		"PORT": env("TRANSLATE_PORT", "8091"),
		"BIND": "127.0.0.1",
	})
}

func mergeEnv(base []string, extra map[string]string) []string {
	for key, value := range extra {
		prefix := strings.ToUpper(key) + "="
		filtered := base[:0]
		for _, entry := range base {
			if !strings.HasPrefix(strings.ToUpper(entry), prefix) {
				filtered = append(filtered, entry)
			}
		}
		base = append(filtered, key+"="+value)
	}
	return base
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func findExecutable(dir string, names ...string) string {
	for _, name := range names {
		candidate := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func supervisorDataDir(child string) (string, error) {
	if root := os.Getenv("NIMOS_LIBRARY_DATA"); root != "" {
		return filepath.Join(root, child), nil
	}
	if runtime.GOOS == "windows" {
		root := os.Getenv("ProgramData")
		if root == "" {
			root = `C:\ProgramData`
		}
		return filepath.Join(root, "Nimos Library", child), nil
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "nimos-library", child), nil
}

func supervisorLogPath() (string, error) {
	dir, err := supervisorDataDir("logs")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "supervisor.log"), nil
}

func runConsole(s *supervisor) error {
	ctx, cancel := signalContext()
	defer cancel()
	return s.run(ctx)
}
