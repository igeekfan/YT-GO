# AGENTS.md - Guide for AI Coding Agents

## Project Overview

YT-GO is a video downloader supporting two runtime modes:
- **Desktop mode** (Wails v2): Go backend + React/TypeScript frontend bundled into a native binary
- **Web mode** (HTTP API): Go backend serving REST API + SSE events, same frontend served as SPA

Both modes share the same core business logic in `internal/core/`.

## Architecture

```
                    ┌─────────────┐
                    │  Frontend    │  (React + TypeScript, Vite)
                    │  App.tsx     │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │ desktop    │    web      │
              │ Wails bind │  fetch/SSE  │
              └──────┬─────┘─────┬──────┘
                     │           │
              ┌──────┴───┐ ┌────┴────────┐
              │ desktop/ │ │ httpapi/     │
              │ App.go   │ │ server.go    │
              └──────┬───┘ └─────┬───────┘
                     │           │
                    ┌┴───────────┴┐
                    │ internal/   │
                    │ core/Service│  ← shared business logic
                    └─────────────┘
```

## Directory Structure

```
.
├── main.go              # Desktop mode entry point (Wails app)
├── main_web.go          # Web mode entry point (HTTP server, build tag: web)
├── version.go           # App version
├── go.mod / go.sum      # Go module (Go 1.25, Wails v2.12.0)
├── wails.json           # Wails project config
├── Dockerfile           # Docker build for web mode
├── desktop/
│   ├── app.go           # Wails App struct, lifecycle hooks
│   ├── app_bindings.go  # Go method wrappers (desktop types ↔ core types)
│   ├── app_ui.go        # Desktop-only UI: SelectFolder, OpenFile, etc.
│   └── types.go         # Desktop-specific type definitions
├── internal/
│   ├── core/            # All business logic (shared by desktop & web)
│   │   ├── service.go   # Service struct, startup, hooks, env vars
│   │   ├── downloads.go # Download execution (go-ytdlp BuildCommand + manual exec)
│   │   ├── executor.go  # yt-dlp operations: check, update, install, version
│   │   ├── ytdlp.go     # Video info / format / playlist queries via go-ytdlp
│   │   ├── progress.go  # Progress regex patterns, helper functions
│   │   ├── diagnostics.go # Dependency detection via go-ytdlp Install
│   │   ├── douyin.go     # Douyin-specific download logic
│   │   ├── jsruntime.go  # Deno/Node runtime detection & install
│   │   ├── errhint.go    # Error message enhancement
│   │   ├── settings.go   # Settings CRUD (SQLite via GORM)
│   │   ├── i18n.go       # Backend i18n
│   │   ├── db.go         # Database setup, record mappings
│   │   └── types.go      # Shared type definitions
│   ├── httpapi/
│   │   ├── server.go     # REST API handlers + file serving
│   │   └── events.go     # SSE EventHub
│   └── platform/
│       └── hidecmd.go    # Windows CMD window hiding
├── frontend/
│   ├── src/
│   │   ├── App.tsx           # Main application component
│   │   ├── App.css           # All styles (single CSS file)
│   │   ├── lib/
│   │   │   ├── backend.ts    # Dual-mode API layer (Wails bind / HTTP fetch)
│   │   │   └── runtime.ts    # Dual-mode event system (Wails Events / SSE)
│   │   ├── components/
│   │   │   ├── SettingsDialog.tsx  # Settings modal (auto-save)
│   │   │   ├── SetupWizard.tsx     # First-run wizard
│   │   │   ├── DownloadList.tsx    # Download queue container
│   │   │   ├── DownloadItem.tsx    # Single download item (desktop: open, web: download link)
│   │   │   ├── UpdateDialog.tsx    # Update notification
│   │   │   └── DirBrowser.tsx      # Server-side directory browser (web mode)
│   │   ├── i18n/
│   │   │   ├── context.tsx    # useI18n hook
│   │   │   ├── zh-CN.ts       # Chinese translations (source of truth)
│   │   │   └── en-US.ts       # English translations
│   │   └── types.ts           # TypeScript type definitions
│   ├── wailsjs/          # Auto-generated Wails bindings (do not edit)
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
└── build/                # Platform-specific build assets
```

## Build / Dev / Test Commands

### Full Application (Desktop)
| Command | Description |
|---------|-------------|
| `wails dev` | Run dev server with hot reload (Go + frontend) |
| `wails build` | Build production binary to `build/bin/` |
| `wails build -debug` | Build with devtools enabled |

### Web Mode
| Command | Description |
|---------|-------------|
| `go build -tags web -o yt-go-server .` | Build web server binary |
| `docker build -t yt-go .` | Build Docker image |

### Go Backend
| Command | Description |
|---------|-------------|
| `go build ./...` | Compile all Go packages |
| `go vet ./...` | Run Go static analysis |
| `go test ./...` | Run all Go tests |
| `go test -run TestName ./path` | Run a single Go test by name |
| `gofmt -w .` | Format all Go files |
| `goimports -w .` | Format and fix imports |

### Frontend
| Command | Description |
|---------|-------------|
| `npm run dev` | Start Vite dev server only |
| `npm run build` | Type-check (`tsc`) then build with Vite |
| `npm run preview` | Preview production build |

All frontend commands run inside the `frontend/` directory.

## Dual-Mode Architecture

The frontend detects the runtime mode via `backendMode` in `lib/backend.ts`:
- **desktop**: `window.go.desktop.App` exists (Wails injects Go bindings)
- **web**: Falls back to HTTP API calls (`/api/*`) + SSE (`/api/events`)

All API functions in `backend.ts` use `getDesktop()` to dispatch:
- Desktop → calls Wails-generated Go binding
- Web → calls `apiFetch('/api/...')` or SSE EventSource

### Key Web-Mode Differences

| Feature | Desktop | Web |
|---------|---------|-----|
| Directory selection | Native dialog (`SelectFolder`) | DirBrowser component or text input |
| File access | `OpenFile`/`OpenFolder` | Download link (`/api/downloads/{id}/file`) |
| Cookies import | Read from browser | Upload file to server |
| Browser cookies | Dropdown (Chrome/Firefox/...) | Hidden (server can't read client browser) |
| Settings save | Auto-save via Wails binding | Auto-save via HTTP API |
| Events | Wails `EventsOn` | SSE `EventSource` |

### Environment Variables (Web Mode)

| Variable | Description | Default |
|----------|-------------|---------|
| `YTGO_WEB_ADDR` | Listen address | `:8080` |
| `YTGO_DOWNLOAD_DIR` | Fixed download directory (hides dir input in UI) | `""` (user selects) |
| `YTGO_EXTERNAL_URL` | External base URL for download links (reverse proxy) | `""` (same-origin) |
| `YTGO_YTDLP_PATH` | Explicit yt-dlp executable path (bypasses auto-detection) | `""` (auto-detect) |
| `XDG_CONFIG_HOME` | App data directory (SQLite DB, cookies) | OS default |

When `YTGO_DOWNLOAD_DIR` is set:
- SetupWizard skips directory selection
- Home page and Settings hide the directory input
- `StartDownload` overrides client `outputDir` with the configured value
- `GET /api/config` returns `hasFixedDir: true`

When `YTGO_EXTERNAL_URL` is set:
- `getDownloadFileURL()` uses it as base URL for browser download links
- Enables download links when server is behind reverse proxy (e.g. `https://yt.example.com`)

When `YTGO_YTDLP_PATH` is set:
- Used as the yt-dlp executable directly, skipping system PATH search
- Useful when yt-dlp is installed but not found by auto-detection (e.g. web mode with non-standard PATH)

## Key Libraries

### go-ytdlp (v1.3.5)
Used for all yt-dlp operations. Key patterns:
- **Builder pattern**: `ytdlp.New().Format("best").Output("/path").BuildCommand(ctx, url)`
- **Install/resolve**: `ytdlp.Install(ctx, &ytdlp.InstallOptions{DisableDownload: true})` for PATH resolution
- **Download execution**: Use `BuildCommand()` → manual `exec.Cmd` for cancel support (not `Run()`)
- **Video info**: `ytdlp.New().DumpSingleJSON().BuildCommand()` → parse `ExtractedInfo`

## Code Style - Go

- **Formatting**: Use `gofmt` (tabs for indentation, no exceptions).
- **Imports**: Standard library first, blank line, then third-party. Use `goimports` to auto-organize.
- **Naming**: CamelCase for exported, camelCase for unexported. Receiver names are short (1-2 chars, e.g. `s *Service`).
- **Error handling**: Return `error` as the last return value. Check errors immediately; do not ignore them.
- **Comments**: Doc comments on exported types/functions. Keep comments minimal and meaningful.
- **Structs**: One struct per file preferred. Constructor pattern: `NewXxx() *Xxx`.
- **Wails bindings**: Methods in `desktop/app_bindings.go` wrap `core.Service` methods with type conversion.
- **Context**: Store `context.Context` from `startup` for use in methods requiring it.

### Go Import Example
```go
import (
    "context"
    "fmt"

    "github.com/lrstanley/go-ytdlp"
    "gorm.io/gorm"
)
```

## Code Style - TypeScript / React

- **Formatting**: No semicolons at end of statements (matches existing code style).
- **Components**: Functional components with hooks. No class components.
- **Imports**: Group order - React, external libs, internal modules, styles. No semicolons after import lines.
- **State**: Use `useState` hook. Destructure tuple: `const [val, setVal] = useState(initial)`.
- **Event handlers**: Inline arrow functions or named function declarations.
- **Naming**: PascalCase for components, camelCase for functions/variables. CSS class names use kebab-case.
- **CSS**: Single `App.css` file. Import at top of file. Use plain CSS with CSS variables (`var(--text-primary)`).
- **Exports**: Use `export default` for the main component in a file.
- **Backend calls**: Import from `../lib/backend`. Do NOT import directly from `wailsjs/`.
- **Mode checks**: Use `backendMode` from `lib/backend.ts` for desktop/web conditional rendering.
- **Web config**: Use `getWebConfig()` for `hasFixedDir`, `externalURL` etc.

### TypeScript Import Example
```typescript
import {useState} from 'react'
import {GetVideoInfo, StartDownload, backendMode, getWebConfig} from '../lib/backend'
import {useI18n} from '../i18n/context'
import './App.css'
```

## Key Conventions

- **Do not edit `frontend/wailsjs/`** - Auto-generated by `wails dev` or `wails build`.
- **Frontend dist is embedded**: `//go:embed all:frontend/dist` in `main.go`. Run `npm run build` before `wails build`.
- **Binding Go methods**: Add method to `desktop/app_bindings.go`, ensure `core.Service` has the method. Wails auto-generates JS bindings.
- **Settings auto-save**: SettingsDialog saves on every change via `autoSave()`, no manual save button.
- **Download cancel**: Uses `BuildCommand()` + manual `exec.Cmd` + `cmd.Process.Kill()` for reliable cancel.
- **TypeScript strict mode** (`"strict": true` in tsconfig). Avoid `any` where possible.
- **No test framework** for frontend. Install `vitest` if adding tests.

## Internationalization (i18n)

All user-facing text MUST support multiple languages (at minimum: `zh-CN` and `en-US`).

### Go Backend
- User-facing strings should be localized via `I18n` module (`internal/core/i18n.go`).
- **Current state**: many error hints in `errhint.go` and `jsruntime.go` are still hardcoded in Chinese. Refactor when modifying.
- When adding a new user-facing string, add entries for all supported languages.

### Frontend
- UI text MUST use `useI18n()` hook with `t('key')` calls.
- Translation keys defined in `frontend/src/i18n/zh-CN.ts` (source of truth) and `frontend/src/i18n/en-US.ts`.
- Never hardcode visible text in components. Use translation keys instead.
- When adding new UI text, add entries in both `zh-CN.ts` and `en-US.ts`.

### General Rules
- Language-neutral identifiers (variable names, function names, type names, commit messages) MUST be in English.
- Code comments should be in English.
- User-facing documentation (README, etc.) should have both Chinese and English versions.

## Code Quality Requirements

**IMPORTANT**: After every code change, you MUST verify that the code compiles without errors:

### Frontend Check
```bash
cd frontend && npm run build
```

### Go Backend Check
```bash
go build ./...
go vet ./...
```

Always run these checks before considering a task complete. If there are compilation errors, fix them immediately.

## Adding a New Feature

### Backend
1. Add method to `core.Service` in appropriate file under `internal/core/`.
2. Add wrapper in `desktop/app_bindings.go` with type conversion (if needed).
3. Add HTTP API handler in `internal/httpapi/server.go` and register in `registerRoutes()`.

### Frontend
1. Add function to `frontend/src/lib/backend.ts` using `getDesktop()` for dual-mode dispatch.
2. Import from `../lib/backend` in your component.
3. Add i18n keys in both `zh-CN.ts` and `en-US.ts`.

## Commit Rules

**After completing each independent feature or fix, commit immediately.** Do not batch multiple features into one commit.

Commit workflow:
1. `git status` — review changed files
2. `git diff` — review the diff
3. `git log --oneline -3` — check commit style
4. `git add <files> && git commit -m "<type>: <description>"`

Commit message format:
- `feat: <description>` — new feature
- `fix: <description>` — bug fix
- `docs: <description>` — documentation change
- `refactor: <description>` — refactor
- `chore: <description>` — miscellaneous

Rules:
- Each commit contains exactly one logical change
- Descriptions are concise and clear
- No period at the end of the commit message
- **Commit messages MUST be in English**
- Always run `go build ./...` and `npm run build` before committing

## Commit & Push

**Only commit and push when the user explicitly asks.** Do not auto-commit or auto-push.

Workflow (when user requests):
1. `go build ./...` and `npm run build` — verify compilation
2. `git status` — review changed files
3. `git diff` — review the diff
4. `git add <files> && git commit -m "<type>: <description>"`
5. `git push` (if user asks to push)

Notes:
- Each commit contains exactly one logical change
- Check that you are on the correct branch before pushing
- If there is a push conflict, pull first then push
