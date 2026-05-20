package owlcms

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"controlpanel/shared"
)

const controlPanelStopPath = "/controlpanel/stop"

// StopProcessByPort asks OWLCMS to stop itself before falling back to OS process termination.
func StopProcessByPort(port string) error {
	if err := requestOwlcmsStop(port); err == nil {
		return nil
	} else {
		log.Printf("OWLCMS in-process stop failed on port %s; falling back to PID/port stop: %v", port, err)
	}

	return shared.StopPIDFileOrPortProcess(pidFilePath, port)
}

func requestOwlcmsStop(port string) error {
	trimmedPort := strings.TrimSpace(port)
	if trimmedPort == "" {
		return fmt.Errorf("missing port")
	}

	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%s%s", trimmedPort, controlPanelStopPath)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s returned %s: %s", controlPanelStopPath, resp.Status, strings.TrimSpace(string(body)))
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if shared.CheckPort(trimmedPort) != nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("OWLCMS accepted stop request but port %s stayed open", trimmedPort)
}
