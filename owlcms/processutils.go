package owlcms

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"owlcms-launcher/shared"
)

// GracefullyStopProcess attempts to stop a process with the given PID,
// using platform-specific methods to ensure Java shutdown hooks can execute.
func GracefullyStopProcess(pid int) error {
	if shared.GetGoos() == "windows" {
		log.Printf("Attempting graceful termination for PID %d\n", pid)
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
		err := cmd.Run()

		if err != nil {
			log.Printf("Graceful termination failed for PID %d: %v\n", pid, err)
			return ForcefullyKillProcess(pid)
		}

		time.Sleep(500 * time.Millisecond)
		if IsProcessRunning(pid) {
			log.Printf("Process %d still running after graceful termination, using forceful termination\n", pid)
			return ForcefullyKillProcess(pid)
		}

		log.Printf("Process (PID: %d) has been gracefully stopped\n", pid)
		return nil
	} else {
		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("failed to find process (PID: %d): %w", pid, err)
		}

		err = process.Signal(syscall.SIGINT)
		if err == nil {
			time.Sleep(1 * time.Second)
			if !IsProcessRunning(pid) {
				log.Printf("Process (PID: %d) has been gracefully stopped with SIGINT\n", pid)
				return nil
			}
		}

		log.Printf("SIGINT failed or process still running, trying SIGTERM for PID %d\n", pid)
		err = process.Signal(syscall.SIGTERM)
		if err == nil {
			time.Sleep(1 * time.Second)
			if !IsProcessRunning(pid) {
				log.Printf("Process (PID: %d) has been stopped with SIGTERM\n", pid)
				return nil
			}
		}

		log.Printf("Graceful termination failed for PID %d, using forceful termination\n", pid)
		return ForcefullyKillProcess(pid)
	}
}

// ForcefullyKillProcess uses the most aggressive methods available to kill a process
func ForcefullyKillProcess(pid int) error {
	log.Printf("FORCEFULLY terminating process with PID: %d\n", pid)

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process (PID: %d): %w", pid, err)
	}

	log.Printf("Attempting direct process kill for PID %d\n", pid)
	if err := process.Kill(); err != nil {
		log.Printf("Direct process kill failed for PID %d: %v\n", pid, err)
	} else {
		log.Printf("Kill signal sent to process %d\n", pid)
	}

	time.Sleep(500 * time.Millisecond)

	if IsProcessRunning(pid) {
		log.Printf("Process %d still running after Kill, using fallback termination method\n", pid)

		if shared.GetGoos() == "windows" {
			cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
			if out, err := cmd.CombinedOutput(); err != nil {
				log.Printf("taskkill fallback failed: %v - %s\n", err, string(out))
				return fmt.Errorf("all termination attempts failed for PID %d", pid)
			}
		} else {
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
	if shared.GetGoos() == "windows" {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), strconv.Itoa(pid))
	} else {
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}
}
