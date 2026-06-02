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
		// 1. 限制请求方法
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		// 2. 清理路径
		reqPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")

		// 3. 如果访问的是根目录，或者该路径在文件系统中不存在且没有后缀（视为前端 SPA 路由）
		//    直接使用 ServeFileFS 返回 index.html，不走 fileServer，绕过 301 陷阱
		if reqPath == "" || reqPath == "." {
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}

		info, err := fs.Stat(fsys, reqPath)

		// 4. 判断是否需要降级到 index.html (SPA 路由)
		isSpaRoute := false
		switch {
		case err == nil && !info.IsDir():
			// 正常存在的静态文件（如 js, css, image），不做处理
		case path.Ext(reqPath) != "":
			// 带后缀的文件不存在（例如 missing-avatar.png），直接 404
			http.NotFound(w, r)
			return
		default:
			// 不带后缀且不存在的路径（例如 /user/profile），判定为前端路由，准备降级
			isSpaRoute = true
		}

		if isSpaRoute {
			http.ServeFileFS(w, r, fsys, "index.html")
			return
		}

		// 5. 正常的静态资源，直接让 fileServer 处理（此时 r.URL.Path 保持原样，如 /css/main.css）
		fileServer.ServeHTTP(w, r)
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
