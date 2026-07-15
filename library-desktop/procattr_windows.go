//go:build windows

package main

import (
	"log"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CREATE_NO_WINDOW: el sidecar es de subsistema consola; sin esto Windows le
// abriría una ventana de consola al lanzarlo desde una app GUI.
const createNoWindow = 0x08000000

// jobHandle agrupa a los sidecars en un Job Object con KILL_ON_JOB_CLOSE: cuando
// el proceso de la app termina —cierre normal, crash o kill a la fuerza— Windows
// cierra el último handle del job y mata a todos sus miembros. Esto cubre el caso
// que OnShutdown no puede (un TerminateProcess nunca ejecuta el handler).
var jobHandle windows.Handle

func init() {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		log.Printf("job object: %v (sin supervisión de crash)", err)
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		h,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		log.Printf("set job info: %v", err)
		windows.CloseHandle(h)
		return
	}
	jobHandle = h
}

func noWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}

// superviseChild mete al hijo recién arrancado en el job. Se llama tras Start().
func superviseChild(cmd *exec.Cmd) {
	if jobHandle == 0 || cmd.Process == nil {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		log.Printf("OpenProcess pid=%d: %v", cmd.Process.Pid, err)
		return
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(jobHandle, h); err != nil {
		log.Printf("AssignProcessToJobObject pid=%d: %v", cmd.Process.Pid, err)
	}
}
