//go:build linux
// +build linux

package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

// selectLocalZip shows a native Fyne file open dialog on Linux.
func selectLocalZip(w fyne.Window, cb func(path string, err error)) {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			cb("", err)
			return
		}
		if reader == nil {
			// Cancelled
			cb("", nil)
			return
		}
		defer reader.Close()
		uri := reader.URI()
		if uri == nil {
			cb("", nil)
			return
		}
		cb(uri.Path(), nil)
	}, w)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".zip"}))
	fd.Show()
}
