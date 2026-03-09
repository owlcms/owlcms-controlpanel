package shared

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	psnet "github.com/shirou/gopsutil/net"
)

// RuntimeMetadata stores the minimum process identity needed to reconnect
// to a previously launched background service safely.
type RuntimeMetadata struct {
	PID               int    `json:"pid"`
	Version           string `json:"version"`
	Port              string `json:"port"`
	ProcessStartTicks uint64 `json:"processStartTicks"`
	StartedAt         string `json:"startedAt"`
}

const RunAsDaemonEnv = "CONTROLPANEL_RUN_AS_DAEMON"

// IsRunAsDaemonEnabled returns true when daemon mode is enabled for this control panel process.
func IsRunAsDaemonEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(RunAsDaemonEnv)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

// SetRunAsDaemonEnabled updates the process environment so runtime behavior can change immediately.
func SetRunAsDaemonEnabled(enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return os.Setenv(RunAsDaemonEnv, value)
}

// LoadRuntimeMetadata reads a runtime metadata file.
func LoadRuntimeMetadata(filePath string) (*RuntimeMetadata, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var metadata RuntimeMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal runtime metadata: %w", err)
	}

	return &metadata, nil
}

// WriteRuntimeMetadata writes runtime metadata atomically.
func WriteRuntimeMetadata(filePath string, pid int, version, port string) (*RuntimeMetadata, error) {
	if err := EnsureDir0755(filepath.Dir(filePath)); err != nil {
		return nil, fmt.Errorf("creating runtime metadata directory: %w", err)
	}

	startTicks, err := ReadProcessStartTicks(pid)
	if err != nil {
		return nil, err
	}

	metadata := &RuntimeMetadata{
		PID:               pid,
		Version:           version,
		Port:              strings.TrimSpace(port),
		ProcessStartTicks: startTicks,
		StartedAt:         time.Now().UTC().Format(time.RFC3339),
	}

	content, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal runtime metadata: %w", err)
	}

	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, content, 0644); err != nil {
		return nil, fmt.Errorf("write runtime metadata temp file: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("rename runtime metadata temp file: %w", err)
	}

	return metadata, nil
}

// ClearRuntimeMetadata removes the runtime metadata file if present.
func ClearRuntimeMetadata(filePath string) error {
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsProcessRunning returns true when a PID still exists.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	if GetGoos() == "windows" {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), strconv.Itoa(pid))
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// ReadProcessStartTicks returns the Linux /proc start time ticks for a PID.
// On non-Linux platforms it returns 0 with no error so callers can degrade gracefully.
func ReadProcessStartTicks(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid PID %d", pid)
	}

	if GetGoos() != "linux" {
		return 0, nil
	}

	content, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, fmt.Errorf("read /proc stat for pid %d: %w", pid, err)
	}

	statLine := strings.TrimSpace(string(content))
	idx := strings.LastIndex(statLine, ")")
	if idx == -1 || idx+2 >= len(statLine) {
		return 0, fmt.Errorf("unexpected /proc stat format for pid %d", pid)
	}

	fields := strings.Fields(statLine[idx+2:])
	if len(fields) <= 19 {
		return 0, fmt.Errorf("missing start time in /proc stat for pid %d", pid)
	}

	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse start ticks for pid %d: %w", pid, err)
	}

	return startTicks, nil
}

// PIDMatchesStartTicks validates that the current process for a PID is the same one
// that was originally recorded in runtime metadata.
func PIDMatchesStartTicks(pid int, expected uint64) bool {
	if !IsProcessRunning(pid) {
		return false
	}

	if expected == 0 {
		return true
	}

	actual, err := ReadProcessStartTicks(pid)
	if err != nil {
		return false
	}

	return actual == expected
}

// GracefullyStopPID attempts to stop a process while allowing normal shutdown hooks to run.
func GracefullyStopPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID %d", pid)
	}

	if GetGoos() == "windows" {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
		if err := cmd.Run(); err != nil {
			return ForcefullyKillPID(pid)
		}

		time.Sleep(500 * time.Millisecond)
		if IsProcessRunning(pid) {
			return ForcefullyKillPID(pid)
		}

		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := process.Signal(syscall.SIGINT); err == nil {
		time.Sleep(1 * time.Second)
		if !IsProcessRunning(pid) {
			return nil
		}
	}

	if err := process.Signal(syscall.SIGTERM); err == nil {
		time.Sleep(1 * time.Second)
		if !IsProcessRunning(pid) {
			return nil
		}
	}

	return ForcefullyKillPID(pid)
}

// ForcefullyKillPID terminates a process immediately.
func ForcefullyKillPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID %d", pid)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := process.Kill(); err != nil && GetGoos() != "windows" {
		return fmt.Errorf("kill pid %d: %w", pid, err)
	}

	time.Sleep(500 * time.Millisecond)
	if !IsProcessRunning(pid) {
		return nil
	}

	if GetGoos() == "windows" {
		cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("force taskkill pid %d: %w (%s)", pid, err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := process.Kill(); err != nil {
		return fmt.Errorf("kill pid %d after retry: %w", pid, err)
	}

	return nil
}

// FindPIDByPort returns the PID of the process listening on the given TCP port.
// Returns 0 with no error if no listener is found.
func FindPIDByPort(port int) (int, error) {
	conns, err := psnet.Connections("tcp")
	if err != nil {
		return 0, fmt.Errorf("list TCP connections: %w", err)
	}
	for _, c := range conns {
		if c.Laddr.Port == uint32(port) && c.Status == "LISTEN" {
			return int(c.Pid), nil
		}
	}
	return 0, nil
}

// CheckPort tries to connect to localhost:port and returns nil if a server is responding.
func CheckPort(port string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s", port))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// CheckDaemonRunning loads runtime metadata and checks whether the daemon is
// still alive.  It first validates the PID (with start-ticks on Linux).  If the
// PID is gone it falls back to probing the recorded port.  Returns the metadata
// and true when the daemon appears to be running.
func CheckDaemonRunning(metadataPath string) (*RuntimeMetadata, bool) {
	metadata, err := LoadRuntimeMetadata(metadataPath)
	if err != nil || metadata == nil {
		return nil, false
	}
	if strings.TrimSpace(metadata.Version) == "" || strings.TrimSpace(metadata.Port) == "" {
		return nil, false
	}

	// Primary: PID check (with start-ticks validation on Linux)
	if metadata.PID > 0 && PIDMatchesStartTicks(metadata.PID, metadata.ProcessStartTicks) {
		return metadata, true
	}

	// Fallback: something is listening on the expected port
	if CheckPort(metadata.Port) == nil {
		// Try to resolve the actual PID owning this port
		portNum, err := strconv.Atoi(strings.TrimSpace(metadata.Port))
		if err == nil {
			if pid, err := FindPIDByPort(portNum); err == nil && pid > 0 {
				log.Printf("Resolved PID %d from port %s", pid, metadata.Port)
				metadata.PID = pid
			}
		}
		return metadata, true
	}

	return nil, false
}