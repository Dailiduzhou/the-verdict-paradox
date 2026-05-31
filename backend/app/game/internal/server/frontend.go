package server

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const frontendDistEnv = "FRONTEND_DIST_DIR"

func frontendHandler() (http.Handler, string) {
	distDir := findFrontendDistDir()
	if distDir == "" {
		return nil, ""
	}

	fsys := os.DirFS(distDir)
	fileServer := http.FileServerFS(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		reqPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if reqPath == "" || reqPath == "." {
			reqPath = "index.html"
		}

		servePath := reqPath
		info, err := fs.Stat(fsys, servePath)
		switch {
		case err == nil && !info.IsDir():
		case path.Ext(servePath) != "":
			http.NotFound(w, r)
			return
		default:
			servePath = "index.html"
		}

		req := r.Clone(r.Context())
		urlCopy := *r.URL
		urlCopy.Path = "/" + servePath
		req.URL = &urlCopy
		fileServer.ServeHTTP(w, req)
	}), distDir
}

func findFrontendDistDir() string {
	candidates := []string{
		os.Getenv(frontendDistEnv),
		"frontend-dist",
		"frontend/dist",
		"../frontend/dist",
		"../../frontend/dist",
		"../../../frontend/dist",
		"../../../../frontend/dist",
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if dirHasIndex(candidate) {
			return candidate
		}
	}

	return ""
}

func dirHasIndex(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	indexInfo, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !indexInfo.IsDir()
}
