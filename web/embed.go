package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var rawDist embed.FS

// DistFS exposes the built frontend files from web/dist.
var DistFS, _ = fs.Sub(rawDist, "dist")
