package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"controlpanel/owlcms"
	"controlpanel/shared"
	"controlpanel/tracker"

	"github.com/magiconair/properties"
)

const controlPanelEnvFileName = "env.properties"

type cliOptions struct {
	instanceArg string
	runtimeArg  string
	init        bool
	mqtt        bool
	help        bool
}

type instancePaths struct {
	InstanceName    string
	ControlPanelDir string
	OwlcmsDir       string
	TrackerDir      string
}

const mainInstanceName = "owlcms"

func parseCLIOptions(args []string) cliOptions {
	var opts cliOptions

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--instance-dir", "--instance_dir":
			if i+1 < len(args) {
				i++
				opts.instanceArg = strings.TrimSpace(args[i])
			}
		case "--runtime-dir", "--runtime_dir":
			if i+1 < len(args) {
				i++
				opts.runtimeArg = strings.TrimSpace(args[i])
			}
		case "--owlcms", "--tracker":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
			}
		case "--init":
			opts.init = true
		case "--mqtt":
			opts.mqtt = true
		case "--help", "-h":
			opts.help = true
		default:
			if opts.instanceArg == "" && !strings.HasPrefix(args[i], "-") {
				opts.instanceArg = strings.TrimSpace(args[i])
			}
		}
	}

	return opts
}

func printUsage() {
	fmt.Println("Usage: controlpanel [options]")
	fmt.Println("")
	fmt.Println("Interactive mode:")
	fmt.Println("  controlpanel")
	fmt.Println("  controlpanel <instance-name>")
	fmt.Println("  controlpanel --instance-dir <name-or-path>")
	fmt.Println("")
	fmt.Println("Instance setup:")
	fmt.Println("  controlpanel --instance-dir <name-or-path> [--runtime-dir <name-or-path>] --init")
	fmt.Println("")
	fmt.Println("Headless daemon launch:")
	fmt.Println("  controlpanel <instance-name> --owlcms [version|latest|stop|list]")
	fmt.Println("  controlpanel <instance-name> --tracker [version|latest|stop|list]")
	fmt.Println("  controlpanel [--instance-dir <name-or-path>] --owlcms [version|latest|stop|list]")
	fmt.Println("  controlpanel [--instance-dir <name-or-path>] --tracker [version|latest|stop|list]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --instance-dir, --instance_dir  Instance name or absolute control panel directory")
	fmt.Println("                                  Default: the main owlcms-controlpanel instance")
	fmt.Println("  --runtime-dir,  --runtime_dir   Shared runtime directory for Java/Node/FFmpeg")
	fmt.Println("                                  Default: the runtime directory under the default main instance")
	fmt.Println("  --init                          Initialize the instance directories and exit")
	fmt.Println("  --owlcms [value]                Start/stop/list OWLCMS headlessly for a version, latest, stop, or list")
	fmt.Println("                                  If omitted, value defaults to latest")
	fmt.Println("  --tracker [value]               Start/stop/list Tracker headlessly for a version, latest, stop, or list")
	fmt.Println("                                  If omitted, value defaults to latest")
	fmt.Println("                                  If no --instance-dir is given and <value> matches an")
	fmt.Println("                                  initialized instance name, that instance is selected and")
	fmt.Println("                                  latest is used for that module")
	fmt.Println("  --mqtt                          Enable embedded MQTT when starting OWLCMS from the command line")
	fmt.Println("                                  Default for command-line OWLCMS launches: disabled")
	fmt.Println("  --help, -h                      Show this help and exit")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  Start OWLCMS interactively for the main instance")
	fmt.Println("    controlpanel")
	fmt.Println("    controlpanel owlcms")
	fmt.Println("")
	fmt.Println("  Start OWLCMS headlessly for the main instance using the latest installed version or a specific version:")
	fmt.Println("    controlpanel --owlcms")
	fmt.Println("    controlpanel --owlcms latest")
	fmt.Println("    controlpanel --owlcms 65.0.0-beta05")
	fmt.Println("")
	fmt.Println("  Start OWLCMS and Tracker headlessly for the main instance using the latest installed version or a specific version:")
	fmt.Println("    controlpanel --owlcms --tracker")
	fmt.Println("    controlpanel --owlcms latest --tracker latest")
	fmt.Println("    controlpanel --owlcms 65.0.0-beta05 --tracker 2.3.0")
	fmt.Println("")
	fmt.Println("  Stop the running OWLCMS daemon for the main instance:")
	fmt.Println("    controlpanel --owlcms stop")
	fmt.Println("  Stop both daemons for the main instance:")
	fmt.Println("    controlpanel --owlcms stop --tracker stop")
	fmt.Println("  List installed versions for the main instance:")
	fmt.Println("    controlpanel --owlcms list")
	fmt.Println("    controlpanel --tracker list")
	fmt.Println("")
	fmt.Println("  Initialize a new sibling instance named records:")
	fmt.Println("    controlpanel records --init")
	fmt.Println("")
	fmt.Println("  To control a specific control panel instance, specify the instance name")
	fmt.Println("  Open the records instance in normal interactive mode:")
	fmt.Println("    controlpanel records")
	fmt.Println("")
	fmt.Println("  Start OWLCMS headlessly for records using the latest installed version:")
	fmt.Println("    controlpanel records --owlcms")
	fmt.Println("")
	fmt.Println("  Start specific installed versions for records:")
	fmt.Println("    controlpanel records --owlcms 64.2.0+records --tracker 2.3.0")
	fmt.Println("")
	fmt.Println("  Stop both daemons for records:")
	fmt.Println("    controlpanel records --owlcms stop --tracker stop")
	fmt.Println("")
}

func maybeApplyImplicitInstanceForHeadless(opts cliOptions, owlcmsVersion, trackerVersion string) (string, string, error) {
	if strings.TrimSpace(opts.instanceArg) != "" {
		return owlcmsVersion, trackerVersion, nil
	}
	if strings.TrimSpace(os.Getenv("CONTROLPANEL_INSTANCE")) != "" {
		return owlcmsVersion, trackerVersion, nil
	}

	candidate, err := inferImplicitInstanceName(owlcmsVersion, trackerVersion)
	if err != nil || candidate == "" {
		return owlcmsVersion, trackerVersion, err
	}

	if err := applyCLIInstanceOptions(cliOptions{instanceArg: candidate}); err != nil {
		return owlcmsVersion, trackerVersion, err
	}

	if strings.EqualFold(strings.TrimSpace(owlcmsVersion), candidate) {
		owlcmsVersion = "latest"
	}
	if strings.EqualFold(strings.TrimSpace(trackerVersion), candidate) {
		trackerVersion = "latest"
	}

	return owlcmsVersion, trackerVersion, nil
}

func inferImplicitInstanceName(owlcmsVersion, trackerVersion string) (string, error) {
	owlcmsCandidate := implicitInstanceCandidate(owlcmsVersion, shared.GetOwlcmsInstallDir())
	trackerCandidate := implicitInstanceCandidate(trackerVersion, shared.GetTrackerInstallDir())

	switch {
	case owlcmsCandidate == "" && trackerCandidate == "":
		return "", nil
	case owlcmsCandidate == "":
		return trackerCandidate, nil
	case trackerCandidate == "":
		return owlcmsCandidate, nil
	case strings.EqualFold(owlcmsCandidate, trackerCandidate):
		return owlcmsCandidate, nil
	default:
		return "", fmt.Errorf("headless launch is ambiguous: %q looks like instance %q and %q; specify --instance-dir explicitly", strings.TrimSpace(owlcmsVersion)+"/"+strings.TrimSpace(trackerVersion), owlcmsCandidate, trackerCandidate)
	}
}

func implicitInstanceCandidate(requested, installDir string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" || strings.EqualFold(requested, "latest") || strings.EqualFold(requested, "stop") || strings.EqualFold(requested, "list") {
		return ""
	}

	if _, err := os.Stat(filepath.Join(installDir, requested)); err == nil {
		return ""
	}

	paths, err := resolveInstancePaths(requested)
	if err != nil {
		return ""
	}

	if _, err := os.Stat(controlPanelEnvPath(paths.ControlPanelDir)); err == nil {
		return requested
	}

	return ""
}

func applyCLIInstanceOptions(opts cliOptions) error {
	instanceArg := strings.TrimSpace(opts.instanceArg)
	if instanceArg == "" && opts.init {
		instanceArg = mainInstanceName
	}

	if instanceArg == "" {
		return nil
	}

	paths, err := resolveInstancePaths(instanceArg)
	if err != nil {
		return err
	}

	runtimeDir, err := resolveRequestedRuntimeDir(paths.ControlPanelDir, paths.InstanceName, opts.runtimeArg, opts.init)
	if err != nil {
		return err
	}

	if err := os.Setenv("CONTROLPANEL_INSTALLDIR", paths.ControlPanelDir); err != nil {
		return err
	}
	if err := os.Setenv("OWLCMS_INSTALLDIR", paths.OwlcmsDir); err != nil {
		return err
	}
	if err := os.Setenv("TRACKER_INSTALLDIR", paths.TrackerDir); err != nil {
		return err
	}
	if err := os.Setenv("RUNTIME_DIR", runtimeDir); err != nil {
		return err
	}
	if err := os.Setenv("CONTROLPANEL_INSTANCE", paths.InstanceName); err != nil {
		return err
	}

	owlcms.SetInstallDir(paths.OwlcmsDir)
	tracker.SetInstallDir(paths.TrackerDir)

	if !opts.init {
		if _, err := os.Stat(controlPanelEnvPath(paths.ControlPanelDir)); err != nil {
			if os.IsNotExist(err) {
				if isMainInstance(paths.InstanceName) {
					return nil
				}
				return fmt.Errorf("instance %q is not initialized; run with --instance-dir %s --init first", paths.InstanceName, paths.InstanceName)
			}
			return err
		}
		return nil
	}

	return initializeInstanceLayout(paths, runtimeDir)
}

func initializeInstanceLayout(paths *instancePaths, runtimeDir string) error {
	if err := shared.EnsureDir0755(runtimeDir); err != nil {
		return fmt.Errorf("create runtime dir %s: %w", runtimeDir, err)
	}

	for _, dir := range []struct {
		label string
		path  string
	}{
		{label: "control panel", path: paths.ControlPanelDir},
		{label: "owlcms", path: paths.OwlcmsDir},
		{label: "tracker", path: paths.TrackerDir},
	} {
		if err := shared.EnsureDir0755(dir.path); err != nil {
			return fmt.Errorf("create %s dir %s: %w", dir.label, dir.path, err)
		}
	}

	if err := writeControlPanelEnv(paths.ControlPanelDir, runtimeDir, paths.InstanceName); err != nil {
		return err
	}
	if err := owlcms.InitEnv(); err != nil {
		return err
	}
	if err := tracker.InitEnv(); err != nil {
		return err
	}

	return nil
}

func resolveInstancePaths(spec string) (*instancePaths, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty instance dir")
	}

	defaultControlPanelDir := shared.DefaultControlPanelInstallDir()
	baseDir := filepath.Dir(defaultControlPanelDir)

	if filepath.IsAbs(spec) {
		controlPanelDir := filepath.Clean(spec)
		instanceName := deriveInstanceName(filepath.Base(controlPanelDir))
		if instanceName == "" {
			return nil, fmt.Errorf("could not derive instance name from %q", spec)
		}
		parent := filepath.Dir(controlPanelDir)
		return &instancePaths{
			InstanceName:    instanceName,
			ControlPanelDir: controlPanelDir,
			OwlcmsDir:       resolveOwlcmsDir(parent, instanceName),
			TrackerDir:      filepath.Join(parent, trackerDirName(instanceName)),
		}, nil
	}

	if strings.Contains(spec, string(os.PathSeparator)) {
		return nil, fmt.Errorf("relative instance dir %q must be a simple name", spec)
	}

	instanceName := spec
	return &instancePaths{
		InstanceName:    instanceName,
		ControlPanelDir: filepath.Join(baseDir, controlPanelDirName(instanceName)),
		OwlcmsDir:       resolveOwlcmsDir(baseDir, instanceName),
		TrackerDir:      filepath.Join(baseDir, trackerDirName(instanceName)),
	}, nil
}

func deriveInstanceName(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}

	for _, suffix := range []string{"-controlpanel", "-owlcms", "-tracker"} {
		if strings.HasSuffix(base, suffix) {
			trimmed := strings.TrimSpace(strings.TrimSuffix(base, suffix))
			if trimmed != "" {
				return trimmed
			}
		}
	}

	for _, baseName := range []string{"owlcms-controlpanel", "owlcms", "owlcms-owlcms", "owlcms-tracker"} {
		if strings.EqualFold(base, baseName) {
			return mainInstanceName
		}
	}
	return base
}

func controlPanelDirName(instanceName string) string {
	if isMainInstance(instanceName) {
		return "owlcms-controlpanel"
	}
	return instanceName + "-controlpanel"
}

func trackerDirName(instanceName string) string {
	if isMainInstance(instanceName) {
		return "owlcms-tracker"
	}
	return instanceName + "-tracker"
}

func resolveOwlcmsDir(parentDir, instanceName string) string {
	defaultName := owlcmsDirName(instanceName)
	if !isMainInstance(instanceName) {
		return filepath.Join(parentDir, defaultName)
	}

	for _, name := range []string{"owlcms", "owlcms-owlcms"} {
		candidate := filepath.Join(parentDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return filepath.Join(parentDir, defaultName)
}

func owlcmsDirName(instanceName string) string {
	if isMainInstance(instanceName) {
		return "owlcms"
	}
	return instanceName + "-owlcms"
}

func isMainInstance(instanceName string) bool {
	return strings.EqualFold(strings.TrimSpace(instanceName), mainInstanceName)
}

func resolveRequestedRuntimeDir(controlPanelDir, instanceName, runtimeArg string, init bool) (string, error) {
	runtimeArg = strings.TrimSpace(runtimeArg)
	if runtimeArg != "" {
		return resolveRuntimeDir(runtimeArg), nil
	}

	stored, err := loadStoredRuntimeDir(controlPanelDir)
	if err != nil {
		return "", err
	}
	if stored != "" {
		return stored, nil
	}

	if isMainInstance(instanceName) {
		return shared.DefaultControlPanelInstallDir(), nil
	}

	if init {
		return shared.DefaultControlPanelInstallDir(), nil
	}

	return "", fmt.Errorf("instance %q has no stored runtime dir; run with --init first", instanceName)
}

func resolveRuntimeDir(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return shared.DefaultControlPanelInstallDir()
	}
	if filepath.IsAbs(spec) {
		return filepath.Clean(spec)
	}
	return filepath.Join(filepath.Dir(shared.DefaultControlPanelInstallDir()), spec)
}

func controlPanelEnvPath(controlPanelDir string) string {
	return filepath.Join(controlPanelDir, controlPanelEnvFileName)
}

func loadStoredRuntimeDir(controlPanelDir string) (string, error) {
	props, err := loadControlPanelEnv(controlPanelDir)
	if err != nil || props == nil {
		return "", err
	}
	value, ok := props.Get("RUNTIME_DIR")
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(value), nil
}

func loadControlPanelEnv(controlPanelDir string) (*properties.Properties, error) {
	envPath := controlPanelEnvPath(controlPanelDir)
	content, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", envPath, err)
	}

	props := properties.NewProperties()
	if err := props.Load(content, properties.UTF8); err != nil {
		return nil, fmt.Errorf("load %s: %w", envPath, err)
	}
	return props, nil
}

func writeControlPanelEnv(controlPanelDir, runtimeDir, instanceName string) error {
	props, err := loadControlPanelEnv(controlPanelDir)
	if err != nil {
		return err
	}
	if props == nil {
		props = properties.NewProperties()
	}
	props.Set("RUNTIME_DIR", runtimeDir)
	props.Set("CONTROLPANEL_INSTANCE", instanceName)

	envPath := controlPanelEnvPath(controlPanelDir)
	file, err := os.Create(envPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", envPath, err)
	}
	defer file.Close()

	if _, err := props.Write(file, properties.UTF8); err != nil {
		return fmt.Errorf("write %s: %w", envPath, err)
	}
	return nil
}
