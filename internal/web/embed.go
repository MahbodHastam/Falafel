// Package web embeds the compiled Vue client.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var content embed.FS

// FS returns the client build rooted at dist.
func FS() (fs.FS, error) {
	return fs.Sub(content, "dist")
}
