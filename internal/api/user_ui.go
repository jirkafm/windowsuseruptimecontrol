package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/user/*
var userUIAssets embed.FS

func userUIHandler() http.Handler {
	sub, err := fs.Sub(userUIAssets, "assets/user")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}
