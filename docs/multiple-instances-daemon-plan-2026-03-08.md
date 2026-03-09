# Multiple Control Panel Instances And OWLCMS Daemon Mode Plan

Date: 2026-03-08

## Summary

The goal is to support running two OWLCMS instances simultaneously on a Linux/Raspberry Pi server, with each OWLCMS (and its tracker) surviving control panel restarts.

This is scoped to **OWLCMS and tracker only**. Cameras, firmata (Arduino), and replays are intrinsically local operations tied to physical hardware or local media — they are not suited to cloud/daemon use and are excluded from this plan.

The approach combines two things:

1. **Daemon mode** — OWLCMS and tracker keep running when the control panel closes, and the control panel reconnects to them on restart
2. **Script-based instance creation** — a second instance is created by a setup script that copies the control panel directory structure and symlinks shared runtimes (Java, Node.js), rather than by adding `--instance` infrastructure to the Go code

This keeps the Go code changes minimal. The control panel already works with one OWLCMS and one tracker. The daemon changes teach it to let them survive and to reconnect. The script handles the rest.

## Scope

**In scope:**

- OWLCMS daemon mode (survives control panel close, reconnect on restart)
- Tracker daemon mode (same lifecycle as OWLCMS)
- Runtime metadata persistence for reconnect
- PID-based stop for reconnected daemons
- Linux/Raspberry Pi only
- Script-based creation of additional instances

**Out of scope:**

- Cameras, firmata (Arduino), replays — local-only, always stopped with the control panel
- Windows and macOS multi-instance support
- Built-in `--instance` CLI flag or shared-library instance ID infrastructure
- Any changes to the cameras, firmata, or replays packages

## Current Architecture Constraints

### 1. OWLCMS and tracker are built around package-level singleton state

Observed in the current code:

- `owlcms/config.go`: package-level `installDir = shared.GetOwlcmsInstallDir()`
- `owlcms/launch.go`: package-level `lockFilePath`, `pidFilePath`, `javaPID`, `lock`, `currentProcess`, `currentVersion`
- `owlcms/tab.go`: package-level UI objects and state
- `tracker/config.go`: package-level `installDir = shared.GetTrackerInstallDir()`
- `tracker/launch.go`: package-level `lockFilePath`, `pidFilePath`, `currentProcess`, `currentVersion`

Implication:

- one control panel process can only track one OWLCMS and one tracker
- this is fine — each control panel process manages one instance, and additional instances are separate control panel processes

### 2. Exit handling always stops all child processes

Current behavior in `main.go`:

- `requestExit()` checks `anyProgramRunning()` (OWLCMS, tracker, firmata, cameras, replays)
- if anything is running, it calls `stopAllRunningProcesses(w)` before closing
- `setupCleanupOnExit()` always goes through `requestExit()`
- signal handling also force-stops all running children

Implication:

- OWLCMS and tracker cannot behave as daemons today because closing the control panel always kills them
- this is the primary thing the Go code needs to change

### 3. Runtime and configuration directories are conflated

Currently `shared.GetControlPanelInstallDir()` is used for two distinct purposes:

- **Runtime location**: Java (`<dir>/java/`), Node.js (`<dir>/node/`), ffmpeg (`<dir>/ffmpeg/`) — large, version-pinned binaries that should be shared across instances
- **Configuration/state location**: Fyne preferences, control-panel-level state — per-instance

Other data directories:

- `shared.GetOwlcmsInstallDir()` → `~/.local/share/owlcms` (OWLCMS versions, env.properties, PID files)
- `shared.GetTrackerInstallDir()` → `~/.local/share/owlcms-tracker` (tracker versions, env.properties, PID files)

These all need to be per-instance. But the runtime binaries should not be duplicated.

The fix is to introduce an explicit **runtime directory** concept, separate from the control panel configuration directory:

- `GetRuntimeDir()` — where Java, Node.js, and ffmpeg live; shared across all instances; defaults to `GetControlPanelInstallDir()` but can be overridden via `RUNTIME_DIR` env var
- `GetControlPanelInstallDir()` — per-instance config/state directory; can be overridden via `CONTROLPANEL_INSTALLDIR`

For the default instance, both point to the same directory (current behavior, no change). For a named instance, the launcher script sets `RUNTIME_DIR` to the main instance's control panel directory and `CONTROLPANEL_INSTALLDIR` to the instance-specific directory.

## Target Behavior

### Default instance (no changes to current UX)

- `controlpanel` launches the control panel using the standard directories
- OWLCMS and tracker can be started and stopped from their tabs
- closing the control panel leaves OWLCMS and tracker running (daemon mode)
- reopening the control panel reconnects to the running OWLCMS and tracker
- cameras, firmata, and replays are stopped on close as they are today

### Second instance (created by script)

- a setup script creates a second directory tree (e.g. `~/.local/share/owlcms-gym2/`, `~/.local/share/owlcms-tracker-gym2/`, `~/.local/share/owlcms-controlpanel-gym2/`)
- the launcher script sets `RUNTIME_DIR` to the main instance's control panel directory so Java and Node.js are found there without symlinks or copies
- a launcher script or desktop entry sets the other environment overrides and runs the same `controlpanel` binary
- the second control panel manages its own OWLCMS and tracker with its own ports, PID files, and env.properties

### Daemon behavior

Three exit scenarios:

- **Window close**: OWLCMS and tracker keep running; cameras, firmata, replays are stopped
- **Stop button**: stops the specific process (OWLCMS or tracker) for this instance
- **OS signal (SIGINT/SIGTERM)**: stops everything including OWLCMS and tracker (conservative — the session or machine is shutting down)

## Design

### Instance Creation Model

A second instance is created by a **setup script**, not by Go code.

The script:

1. creates a new directory tree:
   - `~/.local/share/owlcms-gym2/`
   - `~/.local/share/owlcms-tracker-gym2/`
   - `~/.local/share/owlcms-controlpanel-gym2/`
2. creates a default `env.properties` in the new OWLCMS directory with a different port (e.g. `OWLCMS_PORT=8081`)
3. creates a default `env.properties` in the new tracker directory with a different port (e.g. `TRACKER_PORT=8097`)
4. creates a launcher script (e.g. `controlpanel-gym2`) that sets environment overrides and runs the same binary:
   ```bash
   #!/bin/bash
   export RUNTIME_DIR="$HOME/.local/share/owlcms-controlpanel"
   export OWLCMS_INSTALLDIR="$HOME/.local/share/owlcms-gym2"
   export TRACKER_INSTALLDIR="$HOME/.local/share/owlcms-tracker-gym2"
   export CONTROLPANEL_INSTALLDIR="$HOME/.local/share/owlcms-controlpanel-gym2"
   export CONTROLPANEL_INSTANCE="gym2"
   exec controlpanel "$@"
   ```
5. optionally creates a `.desktop` entry for the second instance

No symlinks or copies needed for Java/Node/ffmpeg. The `RUNTIME_DIR` variable tells the control panel where to find them.

**Go code changes needed**:

- Add `GetRuntimeDir()`: returns `RUNTIME_DIR` env var if set, otherwise falls back to `GetControlPanelInstallDir()`. All callers that look for Java, Node.js, or ffmpeg (`GetSharedJavaDir`, `FindLocalJavaForVersion`, `FindLocalNodeForVersion`, `GetSharedFFmpegDir`, etc.) use this instead of `GetControlPanelInstallDir()`.
- The `Get*InstallDir()` functions for OWLCMS, tracker, and control panel check their respective env var overrides before returning defaults.
- If `CONTROLPANEL_INSTANCE` is set, derive a distinct Fyne app ID (e.g. `app.owlcms.controlpanel.gym2`) and window title (e.g. `OWLCMS Control Panel (gym2)`).

### Shared vs. Per-Instance Resources

| Resource | Location | Shared via |
|----------|----------|------------|
| Java runtime (~200MB) | `RUNTIME_DIR/java/` | `RUNTIME_DIR` env var |
| Node.js runtime (~50MB) | `RUNTIME_DIR/node/` | `RUNTIME_DIR` env var |
| ffmpeg (~100MB) | `RUNTIME_DIR/ffmpeg/` | `RUNTIME_DIR` env var |
| OWLCMS versions (~100-150MB each) | `OWLCMS_INSTALLDIR/<version>/` | per-instance |
| Tracker versions | `TRACKER_INSTALLDIR/<version>/` | per-instance |
| env.properties | `*_INSTALLDIR/env.properties` | per-instance |
| PID files, lock files | `*_INSTALLDIR/` | per-instance |
| Runtime metadata (owlcms-run.json) | `OWLCMS_INSTALLDIR/` | per-instance |
| Fyne preferences/state | Fyne app data | per-instance via distinct app ID |

For the **default instance**, `RUNTIME_DIR` is not set, so `GetRuntimeDir()` falls back to `GetControlPanelInstallDir()` — identical to current behavior.

For a **named instance**, `RUNTIME_DIR` points to the main control panel directory. Java/Node/ffmpeg are found there. No duplication, no symlinks.

OWLCMS versions are duplicated per instance (~100-150MB each). This is acceptable — sharing them would require cross-instance coordination that adds complexity without meaningful benefit for a typical 1-2 version deployment.

### Daemon Ownership Model

Each control panel process owns at most one OWLCMS process and one tracker process.

Per instance:

- one `java.pid` and `java.lock` for OWLCMS
- one `tracker.pid` and `tracker.lock` for tracker
- one `OWLCMS_PORT` (shared within the instance, not per-version)
- one `TRACKER_PORT` (shared within the instance)
- one current OWLCMS version running
- one current tracker version running

Tracker receives its port via the `PORT` environment variable (mapped from `TRACKER_PORT` in launch.go). This already works the same way OWLCMS receives `OWLCMS_PORT`, so port isolation between instances is handled by the per-instance env.properties — no tracker code changes needed for port handling.

### Reconnect Model

On startup, each daemon-capable tab (OWLCMS, tracker) checks for an already-running process:

1. read the instance's runtime metadata file (e.g. `owlcms-run.json` or `tracker-run.json`)
2. if metadata is missing → normal startup UI
3. if metadata exists but PID is dead → clear stale files, normal startup UI
4. if PID exists and process is alive:
   - verify the expected port responds
   - if yes → switch to running mode (show status, stop button, browser link)
   - if no → the process may be hung or the port changed; clear metadata, normal startup UI

Important detail:

`currentProcess` (`*exec.Cmd`) cannot survive across control panel restarts. Reconnect mode needs to support stopping by PID. This applies to both OWLCMS and tracker.

### PID Validation and Stale-PID Protection

Since this is Linux-only, we can use reliable OS-level validation:

**Primary defense — process start time**:

- read `/proc/<pid>/stat` field 22 (start time in clock ticks since boot)
- persist this value in runtime metadata at launch
- on reconnect, compare stored start time against actual — if they disagree, the PID has been reused

**Secondary defense — port responds**:

- check that the expected port actually responds to an HTTP request
- this confirms the service is alive, not just that a process exists

**Optional hardening — PID-to-port ownership**:

- `ss -ltnp sport = :<port>` to verify which PID owns the listening socket
- only proceed with stop/reconnect if the socket-owning PID matches the stored PID
- `ss` is widely available on Linux; fall back gracefully if missing

The layered check on reconnect:

1. read metadata (PID, port, process start time, version)
2. verify PID exists in `/proc/<pid>/`
3. verify start time matches
4. verify port responds
5. if all pass → attached; if any fail → stale, clear metadata

Before killing a reconnected daemon:

1. verify PID start time still matches
2. optionally verify PID owns the expected port via `ss`
3. proceed with kill only if validated

## Detailed Implementation Plan

### Phase 1: Separate runtime directory from configuration directories

Goal:

Introduce `GetRuntimeDir()` as the single source of truth for shared runtime binaries (Java, Node.js, ffmpeg), separate from per-instance configuration directories. Allow all relevant directories to be overridden via environment variables.

Proposed work in `shared/java.go`:

- Add `GetRuntimeDir()`: returns `RUNTIME_DIR` env var if set, otherwise falls back to `GetControlPanelInstallDir()`
- Change `GetSharedJavaDir()` to use `GetRuntimeDir()` instead of `GetControlPanelInstallDir()`
- Change `GetControlPanelInstallDir()`: check `CONTROLPANEL_INSTALLDIR` env var first, fall back to current default

Proposed work in `shared/platformutils.go`:

- `GetOwlcmsInstallDir()`: check `OWLCMS_INSTALLDIR` env var first, fall back to current default
- `GetTrackerInstallDir()`: check `TRACKER_INSTALLDIR` env var first, fall back to current default

Proposed work in `shared/javacheck.go`:

- `FindLocalJavaForVersion()`: use `GetRuntimeDir()` instead of `GetControlPanelInstallDir()` when scanning for installed Java

Proposed work in `shared/nodecheck.go`:

- All Node.js discovery and install functions: use `GetRuntimeDir()` instead of `GetControlPanelInstallDir()`

Proposed work in `shared/ffmpegcheck.go`:

- `GetSharedFFmpegDir()`: use `GetRuntimeDir()` instead of `GetControlPanelInstallDir()`

Proposed work in `main.go`:

- check for `CONTROLPANEL_INSTANCE` env var
- if set, derive a distinct Fyne app ID: `app.owlcms.controlpanel.<instance>`
- if set, append instance name to window title: `OWLCMS Control Panel (<instance>)`

No changes needed for firmata, cameras, or replays — they are not part of multi-instance.

When no environment variables are set, all functions return the same paths as today. `GetRuntimeDir()` falls back to `GetControlPanelInstallDir()`, so the default instance works identically.

### Phase 2: Persist runtime metadata on launch

Goal:

Write enough state at launch time to support reconnect and safe PID-based stop.

Proposed files:

- `owlcms-run.json` in the OWLCMS data directory
- `tracker-run.json` in the tracker data directory

Contents:

```json
{
  "pid": 12345,
  "version": "64.0.0",
  "port": "8080",
  "processStartTicks": 123456789,
  "startedAt": "2026-03-08T14:30:00Z"
}
```

`processStartTicks` is read from `/proc/<pid>/stat` field 22 after `cmd.Start()`.

In `owlcms/launch.go`:

- after `cmd.Start()` succeeds, read `/proc/<pid>/stat` and write `owlcms-run.json`
- on stop (whether by button or signal), delete `owlcms-run.json`

In `tracker/launch.go`:

- same pattern: write `tracker-run.json` after node process starts, delete on stop

### Phase 3: Change close behavior to support daemon mode

Goal:

Closing the control panel window leaves OWLCMS and tracker running. Cameras, firmata, and replays are stopped as today.

Changes in `main.go`:

#### `requestExit(w)`

Replace the current blanket "stop everything" with selective behavior:

- check if cameras, firmata, or replays are running → stop those
- check if OWLCMS or tracker are running → leave them alone
- adjust confirmation dialog text:
  - if OWLCMS/tracker running: "OWLCMS and tracker will continue running in the background. Use the control panel to stop them later."
  - if only local modules running: "Running applications will be stopped. Exit?"
  - if nothing running: simple "Exit?"

#### `setupCleanupOnExit()`

- route through the updated `requestExit()` which now preserves OWLCMS/tracker

#### Signal handling (`SIGINT`, `SIGTERM`)

- keep the current conservative behavior: stop everything including OWLCMS and tracker
- signals mean the session or machine is shutting down — leaving headless Java/Node processes running would be surprising

#### `anyProgramRunning()` / `stopAllRunningProcesses()`

Rename or split:

- `anyLocalProgramRunning()` — checks firmata, cameras, replays only
- `anyDaemonRunning()` — checks OWLCMS, tracker
- `stopLocalProcesses(w)` — stops firmata, cameras, replays
- `stopAllProcesses(w)` — stops everything (used only for signal cleanup)

### Phase 4: Reconnect on startup for OWLCMS

Goal:

When the control panel opens, detect an already-running OWLCMS and attach to it.

Where: early in `owlcms.CreateTab(...)` or `initializeOwlcmsTab(...)`.

Detection sequence:

1. read `owlcms-run.json`
2. if missing → normal startup UI
3. if present:
   - check `/proc/<pid>/` exists
   - read `/proc/<pid>/stat` and compare start time to stored `processStartTicks`
   - if start time does not match → stale PID, clear metadata, normal UI
   - check HTTP response on stored port
   - if port responds → enter running mode
   - if all checks fail → clear stale metadata, normal UI

Running mode UI:

- status: `OWLCMS running (PID: 12345) on port 8080`
- stop button visible with version text
- browser link to `http://localhost:<port>`
- app directory link and tail-log link

### Phase 5: Reconnect on startup for tracker

Same logic as Phase 4, applied to the tracker tab.

Detection: read `tracker-run.json`, validate PID + start time + port.

Running mode UI (tracker-specific):

- status: `Tracker running (PID: 67890) on port 8096`
- stop button visible
- tracker URL link

### Phase 6: PID-based stop for reconnected daemons

Goal:

Allow the stop button to work for a process that was launched by a previous control panel session.

Current issue: `stopProcess()` in both `owlcms/monitor.go` and `tracker/monitor.go` requires a live `*exec.Cmd`.

Add a `stopByPID(pid int)` helper (can go in `shared/` since the pattern is identical):

- verify PID start time matches stored metadata
- optionally verify PID-to-port ownership via `ss -ltnp`
- send SIGINT, wait briefly, then SIGKILL if needed (same pattern as current stop logic)
- clear runtime metadata and PID/lock files

In `owlcms/tab.go` and `tracker/tab.go`, the stop button calls:

- `stopProcess()` if `currentProcess` is available (launched this session)
- `stopByPID()` if reattached from metadata (no live `*exec.Cmd`)

### Phase 7: Instance setup script

Goal:

Provide a script that creates a second instance directory structure.

Location: `RUNTIME_DIR/scripts/create-instance.sh` (i.e. `~/.local/share/owlcms-controlpanel/scripts/create-instance.sh` for the default runtime directory). The scripts are shipped alongside the control panel binary and installed into the runtime directory.

Usage: `create-instance.sh <name> <main-dir>` (e.g. `create-instance.sh gym2 ~/.local/share/owlcms-controlpanel`)

The first argument is the main instance's control panel directory (the runtime directory). The script derives all paths from it.

The script:

1. creates directories:
   - `~/.local/share/owlcms-<name>/`
   - `~/.local/share/owlcms-tracker-<name>/`
   - `~/.local/share/owlcms-controlpanel-<name>/`
2. creates `~/.local/share/owlcms-<name>/env.properties` with `OWLCMS_PORT=8081`
3. creates `~/.local/share/owlcms-tracker-<name>/env.properties` with `TRACKER_PORT=8097`
4. creates a launcher script `controlpanel-<name>`:
   ```bash
   #!/bin/bash
   export RUNTIME_DIR="<main-dir>"
   export OWLCMS_INSTALLDIR="$HOME/.local/share/owlcms-<name>"
   export TRACKER_INSTALLDIR="$HOME/.local/share/owlcms-tracker-<name>"
   export CONTROLPANEL_INSTALLDIR="$HOME/.local/share/owlcms-controlpanel-<name>"
   export CONTROLPANEL_INSTANCE="<name>"
   exec controlpanel "$@"
   ```
5. optionally creates a `.desktop` file for the second instance

No symlinks needed — `RUNTIME_DIR` tells the control panel where Java, Node.js, and ffmpeg are.

Prerequisite: Phase 1 must be complete so the control panel binary respects the environment variables.

## Suggested File-Level Changes

### `main.go`

- read `CONTROLPANEL_INSTANCE` env var
- derive instance-aware Fyne app ID and window title
- split `stopAllRunningProcesses()` into local-only and all-inclusive variants
- update `requestExit()` to preserve OWLCMS and tracker on window close
- keep signal cleanup as stop-everything

### `shared/java.go`

- Add `GetRuntimeDir()`: returns `RUNTIME_DIR` env var if set, else `GetControlPanelInstallDir()`
- `GetSharedJavaDir()`: use `GetRuntimeDir()` instead of `GetControlPanelInstallDir()`
- `GetControlPanelInstallDir()`: check `CONTROLPANEL_INSTALLDIR` env var, fall back to current logic

### `shared/platformutils.go`

- `GetOwlcmsInstallDir()`: check `OWLCMS_INSTALLDIR` env var, fall back to current logic
- `GetTrackerInstallDir()`: check `TRACKER_INSTALLDIR` env var, fall back to current logic

### `shared/javacheck.go`, `shared/nodecheck.go`, `shared/ffmpegcheck.go`

- Replace all `GetControlPanelInstallDir()` calls that locate runtime binaries with `GetRuntimeDir()`

### `shared/` (new file, e.g. `shared/daemon.go`)

- `WriteRuntimeMetadata(filePath, pid, version, port string)` — writes JSON + reads `/proc/<pid>/stat`
- `ReadRuntimeMetadata(filePath) (*RuntimeMetadata, error)`
- `ClearRuntimeMetadata(filePath)`
- `ValidatePID(pid int, storedStartTicks int64) bool` — checks `/proc/<pid>/stat`
- `StopByPID(pid int, storedStartTicks int64) error` — validated SIGINT/SIGKILL

### `owlcms/launch.go`

- write `owlcms-run.json` after launch
- clear `owlcms-run.json` on stop

### `owlcms/tab.go`

- on tab init, check for `owlcms-run.json` and reconnect if valid
- stop button handles both live `*exec.Cmd` and PID-based stop

### `tracker/launch.go`

- write `tracker-run.json` after launch
- clear `tracker-run.json` on stop

### `tracker/tab.go`

- on tab init, check for `tracker-run.json` and reconnect if valid
- stop button handles both live process and PID-based stop

### Instance setup script

- `scripts/create-instance.sh` — installed to `RUNTIME_DIR/scripts/`; creates directories, env.properties, launcher script (no symlinks needed)

No changes to `cameras/`, `firmata/`, or `replays/` packages.

## Risks And Edge Cases

### 1. Stale PID with reused PID number

Mitigation:

- primary: verify process start time via `/proc/<pid>/stat` matches stored value
- secondary: verify expected port responds
- optional: verify PID-to-port ownership via `ss -ltnp`
- if any check fails → treat as stale, clear metadata

### 2. Runtime metadata diverges from actual process state

Mitigation:

- write metadata atomically after launch
- clear metadata on stop
- on reconnect failure, remove stale metadata and fall back to normal UI

### 3. Default-instance backward compatibility

- default instance uses current paths exactly — no environment variables set, no changes
- only script-created instances use alternate paths

### 4. Quit dialog semantics

Mitigation:

- when OWLCMS or tracker is running:
  - "OWLCMS will continue running in the background. Exit?" with choices `Exit` / `Stop All and Exit`
- when only local modules (cameras, firmata, replays) are running:
  - "Running applications will be stopped. Exit?"
- when nothing is running:
  - simple "Exit?"

### 5. Two instances on the same port

Mitigation:

- the existing `checkPort()` in both OWLCMS and tracker detects port conflicts before launch
- the instance setup script pre-configures distinct ports
- the error message should clearly say which port is in use

### 6. Shared runtime directory is deleted or moved

If the main instance's control panel directory (where Java/Node live) is deleted, the second instance's `RUNTIME_DIR` points to a missing location.

Mitigation:

- the control panel already checks for Java/Node availability on startup and offers to download them
- if `RUNTIME_DIR` exists but the runtime isn't there, the normal download flow handles it
- upgrading Java/Node from either instance writes to `RUNTIME_DIR`, so both instances see the update automatically

### 7. Tracker connection per OWLCMS version vs. shared tracker port

Tracker connection is per OWLCMS version (stored as `OWLCMS_VIDEODATA` in per-version env.properties), while `TRACKER_PORT` is shared within the instance.

This is fine — the tracker URL is constructed from `tracker.GetPort()` when enabling the connection, so it always picks up the instance's tracker port.

## Testing Plan

### Daemon mode tests (default instance)

1. Launch control panel, start OWLCMS on port 8080
2. Start tracker on port 8096
3. Close control panel window
4. Confirm both OWLCMS (port 8080) and tracker (port 8096) still respond
5. Reopen control panel
6. Confirm OWLCMS tab shows reconnected state with correct version and port
7. Confirm tracker tab shows reconnected state
8. Stop OWLCMS from the reconnected UI — confirm it stops and metadata is cleared
9. Stop tracker from the reconnected UI — confirm same

### Stale PID tests

10. Start OWLCMS and tracker, close control panel
11. Kill OWLCMS externally (`kill <pid>`)
12. Reopen control panel — confirm stale metadata is cleaned up, OWLCMS tab shows normal UI
13. Confirm tracker (still alive) is properly reconnected

### Multi-instance tests

14. Run `create-instance.sh gym2`
15. Launch second instance via `controlpanel-gym2`
16. Confirm second instance uses port 8081 / 8097
17. Start OWLCMS and tracker in both instances
18. Confirm all four services respond on their respective ports
19. Stop OWLCMS in one instance — confirm the other is unaffected
20. Close both control panels — confirm all daemons survive
21. Reopen both — confirm reconnect works independently

### Regression tests

- default single-instance usage unchanged (no env vars set = current behavior)
- existing install/update/uninstall flows work
- cameras, firmata, replays still stop on window close
- signal-based cleanup (Ctrl+C) still stops everything

## Implementation Order

1. Environment-based data directory overrides + instance identity (Phase 1)
2. Runtime metadata persistence for OWLCMS and tracker (Phase 2)
3. Selective exit behavior — preserve OWLCMS/tracker on close (Phase 3)
4. OWLCMS reconnect on startup (Phase 4)
5. Tracker reconnect on startup (Phase 5)
6. PID-based stop for reconnected daemons (Phase 6)
7. Instance setup script (Phase 7)

Phases 1-3 are the core daemon mode changes. Phases 4-6 complete the reconnect story. Phase 7 is the instance creation tooling.

## Recommendation

The Go code changes are concentrated in `main.go` (exit behavior), `owlcms/` (metadata + reconnect), `tracker/` (metadata + reconnect), and `shared/` (install dir overrides + daemon helpers). No changes to cameras, firmata, or replays packages.

The multi-instance capability is delivered by a setup script, not by built-in Go infrastructure. This keeps the scope small and avoids architectural changes to the module structure.
