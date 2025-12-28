package firmata

import "owlcms-launcher/shared"

// CheckForInternet checks if there is an internet connection
func CheckForInternet() bool {
	return shared.CheckForInternet()
}
