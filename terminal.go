//go:build desktop

package main

import (
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

func IsLaunchedFromTerminal() bool {
	switch runtime.GOOS {
	case "windows":
		return isLaunchedFromTerminalWindows()
	default:
		return isLaunchedFromTerminalUnix()
	}
}

func isLaunchedFromTerminalWindows() bool {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	attachConsole := kernel32.NewProc("AttachConsole")
	const ATTACH_PARENT_PROCESS = ^uintptr(0)
	ret, _, _ := attachConsole.Call(ATTACH_PARENT_PROCESS)
	return ret != 0
}

func AttachParentConsole() {
	hout, _ := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	herr, _ := windows.GetStdHandle(windows.STD_ERROR_HANDLE)
	os.Stdout = os.NewFile(uintptr(hout), "stdout")
	os.Stderr = os.NewFile(uintptr(herr), "stderr")
}

func isLaunchedFromTerminalUnix() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
