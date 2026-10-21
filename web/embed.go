package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

// StaticHandler returns an http.Handler that serves the embedded web files.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

// StaticFS returns the raw embedded filesystem.
func StaticFS() embed.FS {
	return staticFS
}
