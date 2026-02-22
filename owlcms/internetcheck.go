package owlcms

import "controlpanel/shared"

// CheckForInternet checks if there is an internet connection
func CheckForInternet() bool {
	return shared.CheckForInternet()
}
