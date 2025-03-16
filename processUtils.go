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
	log.Printf("Attempting to gracefully stop process with PID: %d\n", pid)

	if downloadUtils.GetGoos() == "windows" {
		// For Windows, execute taskkill commands with a timeout
		done := make(chan error, 1)

		go func() {
			// First try gentle termination on Windows
			log.Printf("Using taskkill to stop process %d\n", pid)
			cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
			err := cmd.Run()

			if err != nil {
				log.Printf("Failed to gracefully stop process (PID: %d): %v\n", pid, err)

				// Try forceful termination if graceful approach fails
				log.Printf("Attempting forceful termination with taskkill /F for PID %d\n", pid)
				cmd = exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
				err = cmd.Run()

				if err != nil {
					done <- fmt.Errorf("failed to forcefully stop process (PID: %d): %w", pid, err)
					return
				}
			}

			done <- nil
		}()

		// Wait for the command to complete or timeout
		select {
		case err := <-done:
			if err != nil {
				return err
			}
		case <-time.After(3 * time.Second):
			// Final attempt - most forceful option with /T to kill tree
			log.Printf("Taskkill timed out for PID %d, trying final forceful termination\n", pid)
			cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("all termination attempts failed for PID %d", pid)
			}
		}

		log.Printf("Process (PID: %d) has been stopped on Windows\n", pid)
		return nil
	} else {
		// On Unix systems, try SIGINT first (equivalent to Ctrl+C)
		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("failed to find process (PID: %d): %w", pid, err)
		}

		err = process.Signal(syscall.SIGINT)
		if err != nil {
			log.Printf("Failed to send SIGINT to process (PID: %d): %v\n", pid, err)

			// Try SIGTERM next
			err = process.Signal(syscall.SIGTERM)
			if err != nil {
				log.Printf("Failed to send SIGTERM to process (PID: %d): %v\n", pid, err)

				// Fall back to SIGKILL as last resort
				err = process.Kill()
				if err != nil {
					return fmt.Errorf("failed to kill process (PID: %d): %w", pid, err)
				}
			}
		}

		// Verify the process is gone on Unix
		time.Sleep(500 * time.Millisecond) // Give the OS time
		if _, err := os.FindProcess(pid); err == nil {
			// On Unix, FindProcess always succeeds, so check if the process can be signaled with 0
			if err := process.Signal(syscall.Signal(0)); err == nil {
				log.Printf("Process %d still alive after termination attempts\n", pid)
				// Force kill as last resort
				process.Kill()
			}
		}
	}

	log.Printf("Process (PID: %d) has been stopped\n", pid)
	return nil
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

	// Check if process is still running
	isRunning := false
	if downloadUtils.GetGoos() == "windows" {
		// On Windows, use a simple process check
		checkCmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
		out, _ := checkCmd.CombinedOutput()
		isRunning = strings.Contains(string(out), strconv.Itoa(pid))
	} else {
		// On Unix, check if we can send signal 0 (process exists check)
		err := process.Signal(syscall.Signal(0))
		isRunning = (err == nil)
	}

	// If process is still running, use more aggressive approach
	if isRunning {
		log.Printf("Process %d still running after Kill, using fallback termination method\n", pid)

		if downloadUtils.GetGoos() == "windows" {
			// On Windows, as a last resort, use taskkill with all forceful options
			cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("taskkill fallback failed: %v - %s\n", err, string(out))
				return fmt.Errorf("all termination attempts failed for PID %d", pid)
			}
		} else {
			// On Unix, try SIGKILL one more time with a brief delay
			time.Sleep(100 * time.Millisecond)
			if err := process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process after multiple attempts (PID: %d): %w", pid, err)
			}
		}
	}

	log.Printf("Process (PID: %d) has been forcefully terminated\n", pid)
	return nil
}
