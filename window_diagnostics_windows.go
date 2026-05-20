//go:build windows

package main

import (
	"fmt"
	"log"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

type windowsRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type windowsPoint struct {
	X int32
	Y int32
}

var (
	procGetWindowRect   = user32.NewProc("GetWindowRect")
	procGetClientRect   = user32.NewProc("GetClientRect")
	procClientToScreen  = user32.NewProc("ClientToScreen")
	procGetDpiForWindow = user32.NewProc("GetDpiForWindow")
	procRedrawWindow    = user32.NewProc("RedrawWindow")
)

func forceNativeWindowRedraw(w fyne.Window) {
	nativeWindow, ok := w.(driver.NativeWindow)
	if !ok {
		log.Printf("window diagnostics [force-native-redraw]: native window interface not available")
		return
	}

	nativeWindow.RunNative(func(context any) {
		windowsContext, ok := context.(driver.WindowsWindowContext)
		if !ok {
			log.Printf("window diagnostics [force-native-redraw]: native context is %T, not WindowsWindowContext", context)
			return
		}
		redrawNativeWindow(windowsContext.HWND)
	})
}

func redrawNativeWindow(hwnd uintptr) {
	if hwnd == 0 {
		log.Printf("window diagnostics [force-native-redraw]: hwnd=0")
		return
	}

	const (
		rdwInvalidate  = 0x0001
		rdwAllChildren = 0x0080
		rdwUpdateNow   = 0x0100
		rdwFrame       = 0x0400
	)
	flags := uintptr(rdwInvalidate | rdwAllChildren | rdwUpdateNow | rdwFrame)
	ret, _, err := procRedrawWindow.Call(hwnd, 0, 0, flags)
	if ret == 0 {
		log.Printf("window diagnostics [force-native-redraw]: RedrawWindow failed hwnd=0x%x err=%v", hwnd, err)
		return
	}
	log.Printf("window diagnostics [force-native-redraw]: RedrawWindow ok hwnd=0x%x flags=0x%x", hwnd, flags)
}

func logWindowDiagnostics(label string, w fyne.Window) {
	canvasSize := w.Canvas().Size()
	contentSize := fyne.Size{}
	contentPos := fyne.Position{}
	if content := w.Content(); content != nil {
		contentSize = content.Size()
		contentPos = content.Position()
	}
	log.Printf(
		"window diagnostics [%s]: fyne canvas=%0.2fx%0.2f contentPos=%0.2f,%0.2f content=%0.2fx%0.2f contentBR=%0.2f,%0.2f",
		label,
		canvasSize.Width,
		canvasSize.Height,
		contentPos.X,
		contentPos.Y,
		contentSize.Width,
		contentSize.Height,
		contentPos.X+contentSize.Width,
		contentPos.Y+contentSize.Height,
	)

	nativeWindow, ok := w.(driver.NativeWindow)
	if !ok {
		log.Printf("window diagnostics [%s]: native window interface not available", label)
		return
	}

	nativeWindow.RunNative(func(context any) {
		windowsContext, ok := context.(driver.WindowsWindowContext)
		if !ok {
			log.Printf("window diagnostics [%s]: native context is %T, not WindowsWindowContext", label, context)
			return
		}
		logNativeWindowDiagnostics(label, windowsContext.HWND)
	})
}

func logNativeWindowDiagnostics(label string, hwnd uintptr) {
	if hwnd == 0 {
		log.Printf("window diagnostics [%s]: hwnd=0", label)
		return
	}

	windowRect, windowOK := getWindowRect(hwnd)
	clientRect, clientOK := getClientRect(hwnd)
	clientOrigin, originOK := getClientOriginOnScreen(hwnd)
	dpi := getDPIForWindow(hwnd)

	if !windowOK {
		log.Printf("window diagnostics [%s]: hwnd=0x%x GetWindowRect failed", label, hwnd)
		return
	}
	if !clientOK {
		log.Printf("window diagnostics [%s]: hwnd=0x%x GetClientRect failed window=%s dpi=%d", label, hwnd, describeRect(windowRect), dpi)
		return
	}
	if !originOK {
		log.Printf("window diagnostics [%s]: hwnd=0x%x ClientToScreen failed window=%s client=%s dpi=%d", label, hwnd, describeRect(windowRect), describeRect(clientRect), dpi)
		return
	}

	windowWidth := windowRect.Right - windowRect.Left
	windowHeight := windowRect.Bottom - windowRect.Top
	clientWidth := clientRect.Right - clientRect.Left
	clientHeight := clientRect.Bottom - clientRect.Top
	leftFrame := clientOrigin.X - windowRect.Left
	topFrame := clientOrigin.Y - windowRect.Top
	rightFrame := windowRect.Right - (clientOrigin.X + clientWidth)
	bottomFrame := windowRect.Bottom - (clientOrigin.Y + clientHeight)

	log.Printf(
		"window diagnostics [%s]: hwnd=0x%x dpi=%d window=%s windowSize=%dx%d client=%s clientSize=%dx%d clientOrigin=%d,%d frameLRTB=%d,%d,%d,%d",
		label,
		hwnd,
		dpi,
		describeRect(windowRect),
		windowWidth,
		windowHeight,
		describeRect(clientRect),
		clientWidth,
		clientHeight,
		clientOrigin.X,
		clientOrigin.Y,
		leftFrame,
		rightFrame,
		topFrame,
		bottomFrame,
	)
}

func getWindowRect(hwnd uintptr) (windowsRect, bool) {
	var rect windowsRect
	ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	return rect, ret != 0
}

func getClientRect(hwnd uintptr) (windowsRect, bool) {
	var rect windowsRect
	ret, _, _ := procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	return rect, ret != 0
}

func getClientOriginOnScreen(hwnd uintptr) (windowsPoint, bool) {
	point := windowsPoint{}
	ret, _, _ := procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&point)))
	return point, ret != 0
}

func getDPIForWindow(hwnd uintptr) uint32 {
	if err := procGetDpiForWindow.Find(); err != nil {
		return 0
	}
	ret, _, _ := procGetDpiForWindow.Call(hwnd)
	return uint32(ret)
}

func describeRect(rect windowsRect) string {
	return fmt.Sprintf("(%d,%d)-(%d,%d)", rect.Left, rect.Top, rect.Right, rect.Bottom)
}
