package shared

import (
	"bytes"
	"io"
	"strings"
)

// ExtractPackagePath reduces a full source path to a stable, readable package-relative path.
// It strips common repo/module roots while preserving subpackages (e.g. owlcms/tab.go).
// For non-project paths (deps, stdlib), it returns the original string.
func ExtractPackagePath(p string) string {
	// Normalize for matching; we output forward slashes for our own paths.
	norm := strings.ReplaceAll(p, "\\", "/")

	// Strip repo/module roots when present (dev builds, or builds without -trimpath).
	for _, marker := range []string{"/owlcms-controlpanel/", "/controlpanel/", "/owlcms-launcher/"} {
		if idx := strings.LastIndex(norm, marker); idx >= 0 {
			return norm[idx+len(marker):]
		}
	}

	// Also handle when the path is already relative (common in release builds).
	for _, prefix := range []string{"owlcms-controlpanel/", "controlpanel/"} {
		if strings.HasPrefix(norm, prefix) {
			return norm[len(prefix):]
		}
	}

	// If it's already one of our package roots, keep it as-is.
	for _, prefix := range []string{"main.go", "owlcms/", "tracker/", "firmata/", "shared/"} {
		if strings.HasPrefix(norm, prefix) {
			return norm
		}
	}

	// Unknown origin (deps, stdlib, etc.) - leave unchanged.
	return p
}

// NewLogPathShorteningWriter returns a writer that rewrites stdlib log output so the
// file path portion is shortened using ExtractPackagePath.
func NewLogPathShorteningWriter(underlying io.Writer) io.Writer {
	return &logPathShorteningWriter{underlying: underlying}
}

type logPathShorteningWriter struct {
	underlying io.Writer
}

func (w *logPathShorteningWriter) Write(p []byte) (int, error) {
	// log.Printf typically writes a single line, but be resilient.
	parts := bytes.SplitAfter(p, []byte("\n"))
	var buf bytes.Buffer
	buf.Grow(len(p))
	for _, part := range parts {
		buf.Write(shortenLogLine(part))
	}
	_, err := w.underlying.Write(buf.Bytes())
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func shortenLogLine(line []byte) []byte {
	// Target format is: "... <file>:<line>: <msg>" (Llongfile or Lshortfile).
	// The message may contain ':' (e.g., "Tracker:updateTitle"), so we cannot
	// use the last ':' in the line. Instead, find the last occurrence of the
	// pattern ".go:<digits>:".

	marker := []byte(".go:")
	idx := 0
	best := -1
	for {
		pos := bytes.Index(line[idx:], marker)
		if pos < 0 {
			break
		}
		pos += idx
		afterGo := pos + len(marker)
		j := afterGo
		if j >= len(line) || line[j] < '0' || line[j] > '9' {
			idx = afterGo
			continue
		}
		for j < len(line) && line[j] >= '0' && line[j] <= '9' {
			j++
		}
		if j < len(line) && line[j] == ':' {
			best = pos
		}
		idx = afterGo
	}
	if best < 0 {
		return line
	}

	// We found ".go:<digits>:"; locate the start of the file path by scanning
	// left from '.go' to the preceding whitespace (space or tab).
	fileStart := -1
	for k := best - 1; k >= 0; k-- {
		if line[k] == ' ' || line[k] == '\t' {
			fileStart = k + 1
			break
		}
	}
	if fileStart < 0 {
		return line
	}
	fileEnd := best // exclusive, start of ".go:"
	if fileStart >= fileEnd {
		return line
	}

	// Expand fileEnd to include the ".go" extension.
	fileEnd += len(".go")

	orig := string(line[fileStart:fileEnd])
	short := ExtractPackagePath(orig)
	if short == orig {
		return line
	}

	out := make([]byte, 0, len(line)-len(orig)+len(short))
	out = append(out, line[:fileStart]...)
	out = append(out, short...)
	out = append(out, line[fileEnd:]...)
	return out
}
