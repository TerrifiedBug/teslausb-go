package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticFiles embed.FS

// EmbeddedStaticFS returns the embedded static files, or nil if only the
// .gitkeep placeholder is present (dev builds without web assets).
func EmbeddedStaticFS() fs.FS {
	// Check if real assets exist (not just .gitkeep)
	if _, err := fs.Stat(staticFiles, "static/index.html"); err != nil {
		return nil
	}
	sub, _ := fs.Sub(staticFiles, "static")
	return sub
}
