package shared

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// CheckAndInstallJava centralizes the pattern of checking for and installing
// a Java runtime for a module. The concrete check/install work is performed
// by the provided callback `javaCheck`, which should encapsulate module-\
// specific behavior (install dir, version selection, etc.). `requiredVersion`
// is informational and currently unused by shared code but provided for
// future extensibility.
func CheckAndInstallJava(requiredVersion string, statusLabel *widget.Label, w fyne.Window, javaCheck func(*widget.Label) error) error {
	if javaCheck == nil {
		return fmt.Errorf("no java check function provided")
	}

	if err := javaCheck(statusLabel); err != nil {
		dialog.ShowError(fmt.Errorf("Java runtime check/install failed: %w", err), w)
		return err
	}
	return nil
}
