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
	help        bool
}

type instancePaths struct {
	InstanceName    string
	ControlPanelDir string
	OwlcmsDir       string
	TrackerDir      string
}

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
		case "--init":
			opts.init = true
		case "--help", "-h":
			opts.help = true
		}
	}

	return opts
}

func printUsage() {
	fmt.Println("Usage: controlpanel [options]")
	fmt.Println("")
	fmt.Println("Interactive mode:")
	fmt.Println("  controlpanel")
	fmt.Println("  controlpanel --instance-dir <name-or-path>")
	fmt.Println("")
	fmt.Println("Instance setup:")
	fmt.Println("  controlpanel --instance-dir <name-or-path> [--runtime-dir <name-or-path>] --init")
	fmt.Println("")
	fmt.Println("Headless daemon launch:")
	fmt.Println("  controlpanel [--instance-dir <name-or-path>] --owlcms <version|latest|stop>")
	fmt.Println("  controlpanel [--instance-dir <name-or-path>] --tracker <version|latest|stop>")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --instance-dir, --instance_dir  Instance name or absolute control panel directory")
	fmt.Println("                                  Default: the main owlcms-controlpanel instance")
	fmt.Println("  --runtime-dir,  --runtime_dir   Shared runtime directory for Java/Node/FFmpeg")
	fmt.Println("                                  Default: the runtime directory under the default main instance")
	fmt.Println("  --init                          Initialize the instance directories and exit")
	fmt.Println("  --owlcms <value>                Start/stop OWLCMS headlessly for a version, latest, or stop")
	fmt.Println("  --tracker <value>               Start/stop Tracker headlessly for a version, latest, or stop")
	fmt.Println("  --help, -h                      Show this help and exit")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  Initialize a new sibling instance named dev2:")
	fmt.Println("    controlpanel --instance-dir dev2 --init")
	fmt.Println("")
	fmt.Println("  Open the dev2 instance in normal interactive mode:")
	fmt.Println("    controlpanel --instance-dir dev2")
	fmt.Println("")
	fmt.Println("  Start OWLCMS and Tracker headlessly for dev2 using the latest installed versions:")
	fmt.Println("    controlpanel --instance-dir dev2 --owlcms latest --tracker latest")
	fmt.Println("")
	fmt.Println("  Start specific installed versions for dev2:")
	fmt.Println("    controlpanel --instance-dir dev2 --owlcms 64.2.0+records --tracker 2.3.0")
	fmt.Println("")
	fmt.Println("  Stop the running OWLCMS daemon for dev2:")
	fmt.Println("    controlpanel --instance-dir dev2 --owlcms stop")
	fmt.Println("")
	fmt.Println("  Stop the running Tracker daemon for dev2:")
	fmt.Println("    controlpanel --instance-dir dev2 --tracker stop")
	fmt.Println("")
	fmt.Println("  Stop both daemons for dev2:")
	fmt.Println("    controlpanel --instance-dir dev2 --owlcms stop --tracker stop")
}

func applyCLIInstanceOptions(opts cliOptions) error {
	if strings.TrimSpace(opts.instanceArg) == "" {
		return nil
	}

	paths, err := resolveInstancePaths(opts.instanceArg)
	if err != nil {
		return err
	}

	runtimeDir, err := resolveRequestedRuntimeDir(paths.ControlPanelDir, opts.runtimeArg, opts.init)
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
				return fmt.Errorf("instance %q is not initialized; run with --instance-dir %s --init first", paths.InstanceName, paths.InstanceName)
			}
			return err
		}
		return nil
	}

	if err := shared.EnsureDir0755(runtimeDir); err != nil {
		return fmt.Errorf("create runtime dir %s: %w", runtimeDir, err)
	}
	if err := shared.EnsureDir0755(paths.ControlPanelDir); err != nil {
		return fmt.Errorf("create control panel dir %s: %w", paths.ControlPanelDir, err)
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
			OwlcmsDir:       filepath.Join(parent, "owlcms-"+instanceName),
			TrackerDir:      filepath.Join(parent, "owlcms-tracker-"+instanceName),
		}, nil
	}

	if strings.Contains(spec, string(os.PathSeparator)) {
		return nil, fmt.Errorf("relative instance dir %q must be a simple name", spec)
	}

	instanceName := spec
	return &instancePaths{
		InstanceName:    instanceName,
		ControlPanelDir: filepath.Join(baseDir, "owlcms-controlpanel-"+instanceName),
		OwlcmsDir:       filepath.Join(baseDir, "owlcms-"+instanceName),
		TrackerDir:      filepath.Join(baseDir, "owlcms-tracker-"+instanceName),
	}, nil
}

func deriveInstanceName(base string) string {
	base = strings.TrimSpace(base)
	for _, prefix := range []string{"owlcms-controlpanel-", "owlcms-controlpanel_", "owlcms-", "owlcms-tracker-"} {
		if strings.HasPrefix(base, prefix) {
			trimmed := strings.TrimSpace(strings.TrimPrefix(base, prefix))
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return base
}

func resolveRequestedRuntimeDir(controlPanelDir, runtimeArg string, init bool) (string, error) {
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

	if init {
		return shared.DefaultControlPanelInstallDir(), nil
	}

	return "", fmt.Errorf("instance %q has no stored runtime dir; run with --init first", filepath.Base(controlPanelDir))
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
