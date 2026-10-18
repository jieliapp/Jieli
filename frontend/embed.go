package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

func Dist() fs.FS {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return dist
}
