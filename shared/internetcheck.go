package shared

import (
	"log"
	"net/http"
	"time"
)

// CheckForInternet checks if there is an internet connection by trying to perform an HTTP GET request to www.google.com with a timeout of 5 seconds.
func CheckForInternet() bool {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("http://www.google.com")
	if err != nil {
		log.Printf("Internet check failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	result := resp.StatusCode == http.StatusOK
	log.Printf("Internet check: status=%d, available=%v", resp.StatusCode, result)
	return result
}
