package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"owlcms-launcher/downloadUtils"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// GracefullyStopProcess attempts to stop a process with the given PID,
// using platform-specific methods to ensure Java shutdown hooks can execute.
func GracefullyStopProcess(pid int) error {
	// log.Printf("Attempting to gracefully stop process with PID: %d\n", pid)

	if downloadUtils.GetGoos() == "windows" {
		// First try graceful termination (allows shutdown hooks)
		log.Printf("Attempting graceful termination for PID %d\n", pid)
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
		err := cmd.Run()

		if err != nil {
			log.Printf("Graceful termination failed for PID %d: %v\n", pid, err)
			// Fall back to forceful termination
			return ForcefullyKillProcess(pid)
		}

		// Wait a moment and check if process is still running
		time.Sleep(500 * time.Millisecond)
		if IsProcessRunning(pid) {
			log.Printf("Process %d still running after graceful termination, using forceful termination\n", pid)
			return ForcefullyKillProcess(pid)
		}

		log.Printf("Process (PID: %d) has been gracefully stopped\n", pid)
		return nil
	} else {
		// On Unix systems, try SIGINT first (equivalent to Ctrl+C)
		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("failed to find process (PID: %d): %w", pid, err)
		}

		// Try SIGINT (graceful)
		err = process.Signal(syscall.SIGINT)
		if err == nil {
			// Wait a moment and check if process terminated gracefully
			time.Sleep(1 * time.Second)
			if !IsProcessRunning(pid) {
				log.Printf("Process (PID: %d) has been gracefully stopped with SIGINT\n", pid)
				return nil
			}
		}

		// Try SIGTERM (polite termination)
		log.Printf("SIGINT failed or process still running, trying SIGTERM for PID %d\n", pid)
		err = process.Signal(syscall.SIGTERM)
		if err == nil {
			// Wait a moment and check if process terminated
			time.Sleep(1 * time.Second)
			if !IsProcessRunning(pid) {
				log.Printf("Process (PID: %d) has been stopped with SIGTERM\n", pid)
				return nil
			}
		}

		// Fall back to forceful termination
		log.Printf("Graceful termination failed for PID %d, using forceful termination\n", pid)
		return ForcefullyKillProcess(pid)
	}
}

// ForcefullyKillProcess uses the most aggressive methods available to kill a process
// This should only be used when graceful termination fails
func ForcefullyKillProcess(pid int) error {
	log.Printf("FORCEFULLY terminating process with PID: %d\n", pid)

	// Find the process by PID
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process (PID: %d): %w", pid, err)
	}

	// First attempt: Use the OS-specific Kill method directly
	log.Printf("Attempting direct process kill for PID %d\n", pid)
	if err := process.Kill(); err != nil {
		log.Printf("Direct process kill failed for PID %d: %v\n", pid, err)
	} else {
		log.Printf("Kill signal sent to process %d\n", pid)
	}

	// Give the process a moment to terminate
	time.Sleep(500 * time.Millisecond)

	// Check if process is still running using the unified function
	if IsProcessRunning(pid) {
		log.Printf("Process %d still running after Kill, using fallback termination method\n", pid)

		if downloadUtils.GetGoos() == "windows" {
			// On Windows, as a last resort, use taskkill with all forceful options
			cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("taskkill fallback failed: %v - %s\n", err, string(out))
				return fmt.Errorf("all termination attempts failed for PID %d", pid)
			}
		} else {
			// On Unix, try SIGKILL one more time after a brief delay
			time.Sleep(200 * time.Millisecond)
			if err := process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process after multiple attempts (PID: %d): %w", pid, err)
			}
		}
	}

	log.Printf("Process (PID: %d) has been forcefully terminated\n", pid)
	return nil
}

// IsProcessRunning checks if a process with the given PID is currently running
func IsProcessRunning(pid int) bool {
	if downloadUtils.GetGoos() == "windows" {
		// Windows: Use tasklist to check if process exists
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), strconv.Itoa(pid))
	} else {
		// Unix-like systems (Linux, macOS): Use signal 0 to check process existence
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		// Signal 0 doesn't actually send a signal, just checks if the process exists
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}
}
