package tracker

import (
	"time"

	"fyne.io/fyne/v2/widget"
)

// startTimedProgress updates the progress bar from start to end over the given duration.
// It returns a stop function to halt updates when the task completes early.
func startTimedProgress(bar *widget.ProgressBar, start, end float64, duration time.Duration) func() {
	if bar == nil {
		return func() {}
	}
	if end < start {
		end = start
	}
	startTime := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond)
	done := make(chan struct{})

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				elapsed := time.Since(startTime)
				frac := float64(elapsed) / float64(duration)
				if frac > 1 {
					frac = 1
				}
				value := start + (end-start)*frac
				bar.SetValue(value)
				if frac >= 1 {
					return
				}
			}
		}
	}()

	return func() {
		select {
		case <-done:
			return
		default:
			close(done)
		}
	}
}
