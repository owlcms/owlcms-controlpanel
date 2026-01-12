package shared

var (
	launcherVersion = "3.0.0-rc05"
	buildVersion    = "_TAG_"
)

func init() {
	if buildVersion != ("_" + "TAG" + "_") {
		// not running in a development environment
		launcherVersion = buildVersion
	}
}

// GetLauncherVersion returns the current launcher version
func GetLauncherVersion() string {
	return launcherVersion
}
