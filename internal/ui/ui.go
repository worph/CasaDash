// Package ui embeds the built Svelte frontend so the whole app ships as one
// static binary. Vite writes its output to internal/ui/dist (see web/vite.config.ts).
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the embedded frontend rooted at the dist directory.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
