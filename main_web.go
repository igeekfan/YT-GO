//go:build web

package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"YT-GO/internal/core"
	"YT-GO/internal/httpapi"
	"YT-GO/internal/platform"
)

func main() {
	service := core.NewService(WailsInfo.Info.ProductVersion)
	service.SetHooks(core.Hooks{
		AppLog: func(msg string) {
			log.Println(msg)
		},
		DownloadLog: func(taskID string, line string) {
			log.Printf("[%s] %s", taskID, line)
		},
		HideCommand: platform.HideCmdWindow,
	})

	if err := service.Startup(); err != nil {
		log.Printf("service startup failed: %v", err)
	}

	apiServer := httpapi.New(service)
	addr := os.Getenv("YTGO_WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("YT-GO web mode listening on %s", addr)
	if err := http.ListenAndServe(addr, webHandler(apiServer.Handler())); err != nil {
		log.Fatal(err)
	}
}

func webHandler(apiHandler http.Handler) http.Handler {
	const distDir = "frontend/dist"
	indexPath := filepath.Join(distDir, "index.html")
	fileServer := http.FileServer(http.Dir(distDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		if _, err := os.Stat(indexPath); err != nil {
			http.Error(w, "frontend/dist not found, run npm run build in frontend first", http.StatusServiceUnavailable)
			return
		}

		requestPath := strings.TrimPrefix(pathClean(r.URL.Path), "/")
		if requestPath == "" {
			http.ServeFile(w, r, indexPath)
			return
		}

		assetPath := filepath.Join(distDir, filepath.FromSlash(requestPath))
		if info, err := os.Stat(assetPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, indexPath)
	})
}

func pathClean(path string) string {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if cleaned == "." {
		return "/"
	}
	if !strings.HasPrefix(cleaned, "/") {
		return "/" + cleaned
	}
	return cleaned
}
