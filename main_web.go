//go:build web

package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"YT-GO/internal/core"
	"YT-GO/internal/httpapi"
	"YT-GO/internal/platform"
)

func main() {
	platform.EnableUTF8Console()
	service := core.NewService(currentAppVersion())
	if err := service.Startup(); err != nil {
		log.Printf("service startup failed: %v", err)
		return
	}

	apiServer, err := httpapi.New(service)
	if err != nil {
		log.Printf("web server configuration failed: %v", err)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if shutdownErr := service.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Printf("service shutdown failed: %v", shutdownErr)
		}
		return
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if shutdownErr := service.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Printf("service shutdown failed: %v", shutdownErr)
		}
		if closeErr := apiServer.Close(); closeErr != nil {
			log.Printf("api server close failed: %v", closeErr)
		}
	}()
	service.SetHooks(core.Hooks{
		AppLog: func(msg string) {
			log.Println(msg)
			apiServer.Hub().Emit("app:log", msg)
		},
		DownloadUpdate: func(task *core.DownloadTask) {
			if task != nil {
				apiServer.Hub().Emit("download:update", task)
			}
		},
		DownloadRemove: func(taskID string) {
			apiServer.Hub().Emit("download:remove", taskID)
		},
		DownloadLog: func(taskID string, line string) {
			apiServer.Hub().Emit("download:log", map[string]string{"taskId": taskID, "line": line})
		},
	})

	addr := os.Getenv("YTGO_WEB_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if err := httpapi.ValidateListenAddress(addr, os.Getenv("YTGO_AUTH_TOKEN")); err != nil {
		log.Printf("refusing insecure web listener: %v", err)
		return
	}

	httpServer := httpapi.NewHTTPServer(addr, webHandler(apiServer.Handler()))
	httpServer.RegisterOnShutdown(apiServer.Hub().Close)
	errCh := make(chan error, 1)
	go func() {
		log.Printf("YT-GO web mode listening on %s (download root: %s)", addr, apiServer.DownloadRoot())
		errCh <- httpServer.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("web server failed: %v", err)
		}
		return
	case <-signalCtx.Done():
		log.Printf("shutting down web server")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown timed out: %v", err)
		_ = httpServer.Close()
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
