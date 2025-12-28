package shared

import (
	"embed"
	"strings"
)

//go:embed assets/*.md
var assetFS embed.FS

// GetAssetContent returns the content of the embedded asset. It accepts either
// "asset/name.md" or "name.md" and returns an empty string if not found.
func GetAssetContent(path string) string {
	name := path
	if strings.HasPrefix(name, "asset/") {
		name = strings.TrimPrefix(name, "asset/")
	}
	b, err := assetFS.ReadFile("assets/" + name)
	if err != nil {
		return ""
	}
	return string(b)
}
