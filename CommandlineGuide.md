# OWLCMS Control Panel Command-Line Guide

The **OWLCMS Control Panel** can be launched in either interactive (graphical) mode or as a headless background utility. 

An OWLCMS **instance** represents a set of programs (modules) talking to one another. Most people will only launch the standalone **`owlcms`** engine/module proper to run a competition. But owlcms can also interact with its companion **tracker** module for producing documents, running a jury review, etc. Together these constitute an instance.

---

## Command-Line Usage Syntax

```bash
controlpanel [options]
```

---

## 1. Setting and Selecting the Scope

When invoking the Control Panel, you can target a specific execution scope defined by a **Module** (owlcms, tracker, etc.) and an **Instance Name**. 

> Note: it is possible to run several instances of OWLCMS at the same time. This advanced scenario is covered in the [Multiple Instances](#7-multiple-instances) section at the end of this document.

### Module Scope
Designates the module process to manage. Each execution targets exactly one module.
* `-m`, `--module <name>`: Expects **`owlcms`** or **`tracker`**.

### Version Scope
Locates the specific directory and binary version to execute during launch actions.
* `--version <v>`: Targets a specific folder, or `latest` (highest version value), or `previous` (the penultimate version in the version order). If omitted entirely, `latest` is the default.

---

## 2. Managing the Running State

The running state of target modules can be managed using combinations of launch, stop, port configurations, and background execution switches.

### Launching Modules (Foreground vs. Detached Background)
* **Interactive/Supervised (Blocks the terminal):**  
  Launches the module in the foreground. Execution outputs directly to the console window.
  
  ```bash
  controlpanel --module owlcms --launch
  ```
* **Background Detached Mode (Relinquishes the terminal):**  
  Launches the process in the background and immediately relinquishes terminal control back. Use `--background` or its equivalent technical spelling `--daemon-mode`.
  
  ```bash
  controlpanel --module owlcms --launch --background
  ```

* **Auto-connecting to local tracker via `--local-tracker [port]`:**
  When launching the main `owlcms` module, you can specify that it should connect to a companion local tracker. If no port is specified, it assumes the tracker is on default port **`8096`**.

  ```bash
  # Starts owlcms and instructs it to connect to a local tracker running on port 8096
  controlpanel --module owlcms --launch --local-tracker
  ```

### Advanced Port and Module Interconnection Options

* **Specifying exact ports via `--port <port>`:**
  Launches the selected module (either `owlcms` or `tracker`) on the specified port number.
  
  ```bash
  # Start tracker on port 8097
  controlpanel --module tracker --launch --port 8097
  
  # Start owlcms on port 8081 and instruct it to connect to a local tracker on a custom port 8097
  controlpanel --module owlcms --launch --port 8081 --local-tracker 8097
  ```

### Stopping Active Modules
Terminates running processes and returns the port resources. This will stop both normal and daemon processes.
```bash
controlpanel --module owlcms --stop
```

---

## 3. Maintenance Activities (Install, Update, Duplicate, Import, Remove)

You can run isolated maintenance and package activities headlessly, which completely bypasses the loading of graphical Fyne UI frameworks. These activities cleanly differentiate between downloading a brand-new release, updating/migrating an existing release (retaining data and settings), and duplicating version folders.

### Listing Local Modules Versions
Outputs installed version directories on the standard out for the target module:
```bash
controlpanel --module owlcms --list
```

### A. Clean Installation / Downloading a New Version
The `--install` switch performs a clean download and setup of a target release. It downloads zip/jar archives directly from GitHub to the target instance directory and extracts them into a fresh folder.
* **Separation of Concerns:** A clean installation starts with a **blank database and default system configuration**, keeping it completely pristine and isolated. No previous data or configurations are carried over.
```bash
# Clean install/download of a specific release version
controlpanel --module owlcms --install 65.0.0

# Clean install/download of the latest release version on a fresh slate
controlpanel --module owlcms --install latest

# Clean install/download of the latest tracker release
controlpanel --module tracker --install latest
```

### B. Local ZIP Installation and Archive Creation
The `--install-zip` and `--create-zip` switches are command-line equivalents of the **Files** menu entries on the **OWLCMS** and **Tracker** tabs, not entries in the application-level main menu. They correspond to **Install version from ZIP** and **Save installed version as ZIP**. Use them when you are moving an installed version between machines, preserving a prepared setup, or installing a build that is already available as a local ZIP file.

`--install-zip` takes a ZIP file path. If the file name contains a semantic version, such as `owlcms_66.0.0.zip` or `owlcms-tracker_3.4.0.zip`, the Control Panel uses that version name automatically. If the file name does not contain a version, provide the installed version name with `--version`.

```bash
# Install an OWLCMS version from a local ZIP whose filename contains the version
controlpanel --module owlcms --install-zip C:/Downloads/owlcms_66.0.0.zip

# Install a tracker version from a local ZIP whose filename contains the version
controlpanel --module tracker --install-zip C:/Downloads/owlcms-tracker_3.4.0.zip

# Install a local ZIP with a custom filename by providing the installed version name
controlpanel --module owlcms --install-zip C:/Downloads/competition-build.zip --version 66.0.0+venue
```

`--create-zip` takes either an output ZIP file path or an existing directory. When you pass a file path ending in `.zip`, that exact file is created. When you pass an existing directory, the Control Panel creates a standard timestamped ZIP filename inside that directory.

```bash
# Save the latest installed OWLCMS version to a specific ZIP file
controlpanel --module owlcms --create-zip C:/Backups/owlcms-current.zip --version latest

# Save a specific tracker version to a specific ZIP file
controlpanel --module tracker --create-zip C:/Backups/tracker-3.4.0.zip --version 3.4.0

# Save the latest OWLCMS version to a timestamped ZIP in an existing directory
controlpanel --module owlcms --create-zip C:/Backups --version latest
```

### C. In-Place Upgrading / Updating an Existing Version
To upgrade/update an existing version, we use the `--update-to <target-version>` flag. This downloads a newer release but **automatically copies/ports** vital live information from an older helper/source version (specified by `--version <from-version>`).

* **Distinguishing "latest":** 
  * `--version latest`: Refers to the **latest locally-installed version** containing your current competition database.
  * `--update-to latest`: Refers to the **latest available release version on GitHub** that you want to download and migrate to.

* **Separation of Concerns:** The updater copies over your **active database** and carries over any custom **local configurations** made to properties files (such as local env.properties files) since the selected source installation.
```bash
# Update from your latest locally-installed version to the latest GitHub release
controlpanel --module owlcms --version latest --update-to latest

# Update specifically from locally-installed version 64.0.0 to a targeted new release 65.0.0
controlpanel --module owlcms --version 64.0.0 --update-to 65.0.0
```

### D. Version Duplication
The `--duplicate` option duplicates an existing installed version directory, creating an identical copy under a new custom name.
* **Separation of Concerns:** Copies the database state, custom configuration details, and run settings exactly, allowing you to run side-by-side experiments or alternative profiles safely.
```bash
controlpanel --module owlcms --duplicate duplicate-65.0.0 --from-version 65.0.0
```

### E. Importing Configurations & Data Headlessly
Enables manual migration of database files, port settings, user customizations, and plugins between already installed local releases headlessly:
```bash
controlpanel --module tracker --import --from-version 3.3.0 --to-version 3.4.0
```

### F. Removing Installed Versions Headlessly
Uninstalls and cleans up unused module package directories permanently:
```bash
controlpanel --module tracker --remove 3.3.0
```

---

## 4. Full Scripting Examples

Here are the scripts for the two most common setup scenarios.

### Example 1: Standard Competition Setup (Standard Ports)
This starts both the database engine (`owlcms` on default port 8080) and the companion `tracker` on its default port (8096), establishing an automatic connection between them.

* **Lifting interactive foreground processes (supervised):**
  ```bash
  # Start companion tracker in first terminal window (port 8096)
  controlpanel --module tracker --launch
  
  # Start owlcms in second terminal window (port 8080) and auto-connect to tracker
  controlpanel --module owlcms --launch --local-tracker
  ```

* **Running background processes (detached):**
  ```bash
  # Start companion tracker in background
  controlpanel --module tracker --launch --background
  
  # Start owlcms in background and auto-connect to tracker
  controlpanel --module owlcms --launch --local-tracker --background
  ```

### Example 2: Non-Standard Setup (Custom Ports)
This launches the modules on non-standard custom ports to prevent overlaps or match specific system constraints. Here, OWLCMS is run on custom port **`8081`** and connects to the tracker hosted on non-standard port **`8097`**.

* **Lifting interactive foreground processes (supervised):**
  ```bash
  # Start companion tracker in first terminal window on custom port 8097
  controlpanel --module tracker --launch --port 8097
  
  # Start owlcms in second terminal window on custom port 8081 and connect to port 8097
  controlpanel --module owlcms --launch --port 8081 --local-tracker 8097
  ```

* **Running background processes (detached):**
  ```bash
  # Start companion tracker in background on custom port 8097
  controlpanel --module tracker --launch --port 8097 --background
  
  # Start owlcms in background on custom port 8081 and connect to port 8097
  controlpanel --module owlcms --launch --port 8081 --local-tracker 8097 --background
  ```

---

## 5. Command-Line Options Reference

| Argument / Switch | Expected Values | Purpose |
| :--- | :--- | :--- |
| `-i`, `--instance` | `<name>` | Selects a specific instance scope. Defaults to `owlcms`. |
| `-m`, `--module` | `owlcms`, `tracker` | Specifies which module process to manage. Targets exactly one module. |
| `--launch` | *(None)* | Launches the specified module. Keeps the terminal unless `--background` or `--daemon-mode` is provided. If no explicit `--version` is given, `latest` (or `previous` fallback) is implied. |
| `--stop` | *(None)* | Stops the specified running module. |
| `--list` | *(None)* | Lists all installed version directories for the specified module. |
| `--install` | `[version]`, `latest` | Downloads and performs a clean installation of the selected module version from GitHub (isolated database, default configs). |
| `--install-zip` | `<zip-file>` | Installs a local ZIP file (often provided by federation); use `--version` when the filename does not contain the installed version name. |
| `--create-zip` | `<zip-file>` or `<existing-directory>` | Creates a ZIP from the installed version selected by `--version`; a `.zip` path is used exactly, while an existing directory receives a timestamped ZIP filename. |
| `--update-to` | `[version]`, `latest` | Initiates an upgrade/update to the specified version target from GitHub, copying data from the version specified by `--version`. |
| `--duplicate` | `<new-name>` | Duplicates the version specified by `--from-version` into an independent copy directory named `<new-name>`. |
| `--import` | *(None)* | Initiates config/data porting. Requires `--from-version` and `--to-version`. |
| `--remove` | `<version>` | Uninstalls the targeted local version of the module. |
| `--from-version` | `<version-id>` | Source version target used during `--import` or `--duplicate` operations. |
| `--to-version` | `<version-id>` | Destination version target used during a headless `--import` operation. |
| `--background`, `--daemon-mode` | *(None)* | Runs the module in background detached mode, relinquishing the terminal immediately. |
| `--port` | `<port-number>` | Runs the specified module on a given port. |
| `--local-tracker` | `[port-number]` | For `owlcms` launch, configures linking to a locally running tracker. Defaults to port `8096` if no port is specified. |
| `--version` | `<version>`, `latest`, `previous` | The specific version to run. Defaults to `latest` (with `previous` identifying the penultimate version in the version order). |
| `--instance-dir` | `<path>` | Absolute folder path override of the target instance instead of utilizing the automatic sibling directory layouts. |
| `--runtime-dir` | `<path>` | Custom shared runtime directory containing platforms binaries (Java, Node.js, FFmpeg). |
| `--init` | *(None)* | Initializes the directory structures for the selected instance, prints resolved locations, and exits. |
| `--mqtt` | *(None)* | Enables the embedded MQTT broker for OWLCMS in headless mode. |
| `-h`, `--help` | *(None)* | Prints this command-line guide and exits. |

---

## 6. Overrides and Managed Directories Layout

When `--init` runs, the Control Panel creates and manages directories structured as follows:

* **Control Panel Dir:** Scope for general configuration property files.
* **OWLCMS Dir:** Subdirectory where downloaded OWLCMS modules (JARs) are stored.
* **Tracker Dir:** Subdirectory where downloaded Tracker modules (ZIPs) are stored.
* **Runtime Dir:** Common workspace directory housing binaries for Java (Temurin), Node.js, and FFmpeg platforms.

---

## 7. Multiple Instances

In complex multi-platform deployments (such as conducting two simultaneous sessions, or segregating real-time competition scoring from long-term reference logs), you can run multiple independent Control Panel instances on the same host machine.

### What is isolated under a sibling instance?
Each named instance maintains its own:
* Distinct filesystem folders for local config properties, downloaded module versions/JARs, and database files.
* Independent process tracking (allowing different versions of `owlcms` or `tracker` to run simultaneously).
* Conflicting port management, necessitating custom `--port` values to avoid conflicts on `localhost`.

### Example Scenario: Running Sibling "records" Instance
In this scenario, a primary default instance runs the current competition, while an isolated `records` instance runs on the side to manage records or historical reviews.

1. **Start the primary default (`owlcms`) processes:**
   ```bash
   # Primary OWLCMS on default port 8080 connecting to Tracker on port 8096
     controlpanel --module owlcms --launch --background --port 8080 --local-tracker 8096
     controlpanel --module tracker --launch --background --port 8096
   ```

2. **Start the secondary sibling (`records`) processes on custom ports:**
   ```bash
   # Secondary Sibling OWLCMS on custom port 8180 connecting to custom Sibling Tracker on port 8196
     controlpanel records --module owlcms --launch --background --port 8180 --local-tracker 8196
     controlpanel records --module tracker --launch --background --port 8196
   ```

By segregating these via `-i records` (or positional `records` shortcut), the respective folders are isolated and the run states do not interfere.
