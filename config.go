package main

var (
	launcherVersion = "1.0.0" // Default launcher version
	buildVersion    = "_TAG_" // Placeholder for build version
)

func init() {
	if buildVersion != "_TAG_" {
		launcherVersion = buildVersion
	}
}
