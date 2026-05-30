package docs

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed swagger.html openapi.yaml
var Static embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(Static, ".")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}
