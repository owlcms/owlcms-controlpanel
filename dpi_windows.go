//go:build windows

package main

import (
	"log"
	"syscall"
)

var (
	user32                            = syscall.NewLazyDLL("user32.dll")
	shcore                            = syscall.NewLazyDLL("shcore.dll")
	kernel                            = syscall.NewLazyDLL("kernel32.dll")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwareness        = shcore.NewProc("SetProcessDpiAwareness")
	procGetLastError                  = kernel.NewProc("GetLastError")
)

func configureProcessDPIAwareness() {
	if setPerMonitorV2DPIAwareness() {
		return
	}
	if setPerMonitorDPIAwareness() {
		return
	}
	if setSystemDPIAwareness() {
		return
	}
	log.Println("could not enable Windows DPI awareness")
}

func setPerMonitorV2DPIAwareness() bool {
	if err := procSetProcessDpiAwarenessContext.Find(); err != nil {
		return false
	}

	const errorAccessDenied = 5
	dpiAwarenessContextPerMonitorAwareV2 := uintptr(^uintptr(3))
	ret, _, _ := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	if ret != 0 {
		return true
	}

	lastErr, _, _ := procGetLastError.Call()
	return lastErr == errorAccessDenied
}

func setPerMonitorDPIAwareness() bool {
	if err := procSetProcessDpiAwareness.Find(); err != nil {
		return false
	}

	const processPerMonitorDPIAware = 2
	ret, _, _ := procSetProcessDpiAwareness.Call(processPerMonitorDPIAware)
	return ret == 0
}

func setSystemDPIAwareness() bool {
	if err := procSetProcessDPIAware.Find(); err != nil {
		return false
	}

	ret, _, _ := procSetProcessDPIAware.Call()
	return ret != 0
}
