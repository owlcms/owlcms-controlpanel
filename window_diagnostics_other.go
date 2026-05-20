//go:build !windows

package main

import "fyne.io/fyne/v2"

func logWindowDiagnostics(string, fyne.Window) {}

func forceNativeWindowRedraw(fyne.Window) {}
