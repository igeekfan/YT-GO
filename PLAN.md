# Architecture Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Commit after each round once verification passes.

**Goal:** Improve YT-GO's maintainability by fixing known web-mode defects first, then deepening the highest-friction modules behind smaller interfaces.

**Architecture:** Keep the existing dual-mode design: desktop uses Wails bindings, web uses HTTP plus SSE, and both share `internal/core`. The first rounds repair correctness and security regressions. Later rounds split shallow modules only where the split improves locality, leverage, and testability.

**Tech Stack:** Go 1.25, Wails v2, React, TypeScript, Vite, SQLite via GORM, `github.com/lrstanley/go-ytdlp`.

---

## Current Findings

- `frontend/src/lib/backend.ts` calls `/api/ytdlp/version-check`, but `internal/httpapi/server.go` does not register that route.
- `internal/httpapi/server.go` allows `/api/events` without auth even when `YTGO_AUTH_TOKEN` is set, while `frontend/src/lib/runtime.ts` already sends the token in the SSE query string.
- `frontend/src/lib/runtime.ts` imports `getAuthToken` from `./backend`, which couples runtime events back to the large dual-mode backend module.
- `internal/core/downloads.go` combines download request validation, task lifecycle, command construction, progress parsing, persistence, and cancellation in one wide module.
- `frontend/src/App.tsx` combines media probing, format selection, playlist handling, download request construction, settings sync, event subscription, auth, notifications, console logs, and update checks in one module.
- `internal/httpapi/server.go` mixes middleware, routing, JSON response helpers, file serving, directory browsing, cookies upload, and all handlers in one module.

---

## Round 1 - Web Route And SSE Auth Fix

**Files:**
- Modify: `internal/httpapi/server.go`
- Test: `go build ./...`
- Test: `go vet ./...`
- Test: `cd frontend && npm run build`

- [ ] **Step 1: Add missing yt-dlp version route**

Register `GET /api/ytdlp/version-check` in `registerRoutes()`:

```go
s.mux.HandleFunc("/api/ytdlp/version-check", s.handleYtDlpVersionCheck)
```

Add handler:

```go
func (s *Server) handleYtDlpVersionCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	result, err := s.service.CheckYtDlpVersion()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 2: Require auth for SSE when auth is enabled**

Remove `/api/events` from `isAuthWhitelisted`. `checkAuth()` already supports `?token=...`, and `frontend/src/lib/runtime.ts` already appends the token.

- [ ] **Step 3: Verify**

Run:

```bash
go build ./...
go vet ./...
cd frontend && npm run build
```

Expected: all commands exit 0.

- [ ] **Step 4: Commit**

```bash
git status
git diff
git log --oneline -3
git add internal/httpapi/server.go
git commit -m "fix: repair web ytdlp version and event auth"
```

---

## Round 2 - HTTP Behavior Tests

**Files:**
- Create: `internal/httpapi/server_test.go`
- Test: `go test ./internal/httpapi`
- Test: `go build ./...`
- Test: `go vet ./...`
- Test: `cd frontend && npm run build`

- [ ] **Step 1: Test auth whitelist behavior**

Add tests that assert:

```go
func TestIsAuthWhitelistedExcludesEvents(t *testing.T) {
	if isAuthWhitelisted("/api/events") {
		t.Fatal("/api/events must require auth when YTGO_AUTH_TOKEN is set")
	}
}

func TestIsAuthWhitelistedAllowsHealthAndConfig(t *testing.T) {
	for _, path := range []string{"/api/health", "/api/config"} {
		if !isAuthWhitelisted(path) {
			t.Fatalf("%s should be whitelisted", path)
		}
	}
}
```

- [ ] **Step 2: Test bearer and query-token auth**

Add tests around `Server.checkAuth()`:

```go
func TestCheckAuthAcceptsBearerToken(t *testing.T) {
	s := &Server{authToken: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/api/downloads", nil)
	r.Header.Set("Authorization", "Bearer secret")
	if !s.checkAuth(r) {
		t.Fatal("expected bearer token to authenticate")
	}
}

func TestCheckAuthAcceptsQueryToken(t *testing.T) {
	s := &Server{authToken: "secret"}
	r := httptest.NewRequest(http.MethodGet, "/api/events?token=secret", nil)
	if !s.checkAuth(r) {
		t.Fatal("expected query token to authenticate")
	}
}
```

- [ ] **Step 3: Test version route registration without invoking external dependencies**

Use a lightweight route existence test that verifies `/api/ytdlp/version-check` is not a 404 or method mismatch for `OPTIONS`/method filtering. If a direct handler call would invoke yt-dlp, keep the test focused on route auth and method handling instead.

- [ ] **Step 4: Verify**

Run:

```bash
go test ./internal/httpapi
go build ./...
go vet ./...
cd frontend && npm run build
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit**

```bash
git status
git diff
git log --oneline -3
git add internal/httpapi/server_test.go
git commit -m "test: cover web auth routing behavior"
```

---

## Round 3 - Frontend Event Runtime Decoupling

**Files:**
- Modify: `frontend/src/lib/runtime.ts`
- Test: `cd frontend && npm run build`
- Test: `go build ./...`
- Test: `go vet ./...`

- [ ] **Step 1: Remove backend import from runtime**

Change:

```ts
import {getAuthToken} from './backend'
```

to:

```ts
import {getAuthToken} from './api_client'
```

This keeps the event runtime dependent only on the web API client rather than the full dual-mode backend module.

- [ ] **Step 2: Verify**

Run:

```bash
cd frontend && npm run build
go build ./...
go vet ./...
```

Expected: all commands exit 0.

- [ ] **Step 3: Commit**

```bash
git status
git diff
git log --oneline -3
git add frontend/src/lib/runtime.ts
git commit -m "refactor: decouple event runtime auth"
```

---

## Round 4 - Download Workflow Module Deepening

**Files:**
- Modify: `internal/core/downloads.go`
- Create: `internal/core/download_lifecycle.go`
- Create: `internal/core/download_executor.go`
- Test: existing `internal/core/*download*_test.go`
- Test: `go test ./internal/core`
- Test: `go build ./...`
- Test: `go vet ./...`
- Test: `cd frontend && npm run build`

- [ ] **Step 1: Extract task lifecycle helpers**

Move task map mutations, persistence calls, and event emission into focused helpers:

```go
func (s *Service) setDownloadStatus(taskID string, apply func(*DownloadTask)) (*DownloadTask, bool)
func (s *Service) removeDownloadTask(taskID string) bool
func (s *Service) persistDownloadSnapshot(task *DownloadTask)
```

The interface should make status changes explicit and keep locking local to the lifecycle helpers.

- [ ] **Step 2: Extract yt-dlp command construction**

Move the builder setup from `runDownload()` into:

```go
func (s *Service) buildDownloadCommand(ctx context.Context, req DownloadRequest, ytdlpPath string) (*exec.Cmd, error)
```

Keep subtitle, cookies, proxy, output template, merge format, audio extraction, and media command behavior identical.

- [ ] **Step 3: Verify focused tests**

Run:

```bash
go test ./internal/core
go build ./...
go vet ./...
cd frontend && npm run build
```

Expected: all commands exit 0.

- [ ] **Step 4: Commit**

```bash
git status
git diff
git log --oneline -3
git add internal/core/downloads.go internal/core/download_lifecycle.go internal/core/download_executor.go
git commit -m "refactor: deepen download workflow module"
```

---

## Round 5 - Frontend Download Composer Extraction

**Files:**
- Modify: `frontend/src/App.tsx`
- Create: `frontend/src/lib/downloadComposer.ts`
- Test: `cd frontend && npm run build`
- Test: `go build ./...`
- Test: `go vet ./...`

- [ ] **Step 1: Extract pure format helpers**

Move these functions from `App.tsx` into `downloadComposer.ts`:

```ts
getSubtitleSelectionKey
splitSelectedSubtitleLangs
parseResolutionHeight
formatOptionLabel
sortFormats
findFormatByID
```

- [ ] **Step 2: Extract request construction helpers**

Move download quality and options construction behind pure functions that take plain inputs:

```ts
export function resolveDownloadQuality(input: ResolveDownloadQualityInput): string
export function buildDownloadOptions(input: BuildDownloadOptionsInput): DownloadOptions | undefined
```

Keep current behavior unchanged for single format, combined tracks, audio-only, video-only, subtitles, sidecars, SponsorBlock, and filename template.

- [ ] **Step 3: Update App imports and calls**

Import helpers from `./lib/downloadComposer` and leave UI state in `App.tsx`.

- [ ] **Step 4: Verify**

Run:

```bash
cd frontend && npm run build
go build ./...
go vet ./...
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit**

```bash
git status
git diff
git log --oneline -3
git add frontend/src/App.tsx frontend/src/lib/downloadComposer.ts
git commit -m "refactor: extract frontend download composer"
```

---

## Later Candidates

These should be scheduled only after Rounds 1-5 are complete:

- Split `internal/httpapi/server.go` into middleware, response helpers, media routes, settings routes, and download routes.
- Split `frontend/src/components/SettingsDialog.tsx` into dependency diagnostics, appearance, download settings, media settings, and about modules.
- Create `CONTEXT.md` with domain vocabulary: download task, media probe, format selection, web config, dependency status, event stream.
- Add frontend tests with Vitest before any larger state-management extraction.
