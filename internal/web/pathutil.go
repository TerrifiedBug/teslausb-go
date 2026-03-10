package web

import (
	"fmt"
	"path/filepath"
	"strings"
)

// safePath validates a user-supplied path against a base directory.
// Returns the full resolved path if it stays within base, or an error
// if the path would escape. A naive join of base + userPath is cleaned
// and checked; if the result would leave base the call is rejected.
func safePath(base, userPath string) (string, error) {
	// Clean the raw join first — this preserves ".." traversals so we
	// can detect them, unlike the "/" prefix trick which silently strips them.
	full := filepath.Clean(filepath.Join(base, userPath))
	rel, err := filepath.Rel(base, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal blocked: %q", userPath)
	}
	return full, nil
}
