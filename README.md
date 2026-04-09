# YT-GO

[English](README.md) | [简体中文](README.zh-CN.md)

YT-GO is a cross-platform desktop YouTube downloader powered by [yt-dlp](https://github.com/yt-dlp/yt-dlp). Paste a URL, pick a quality, choose an output folder, and download — no command line needed.

## Overview

YT-GO focuses on a simple desktop workflow built on top of yt-dlp:

- Fetch video or playlist metadata before downloading
- Choose preset quality, a single detected format, or a combined video+audio format
- Manage multiple concurrent download tasks in one desktop app
- Keep practical settings such as proxy, cookies, rate limit, notifications, and concurrency persisted locally

## Features

- One-click metadata fetch for YouTube videos and playlist URLs
- Video preview with title, uploader, duration, platform, and thumbnail
- Playlist detection and batch enqueue for playlist items
- Preset qualities: Best, 1080p, 720p, 480p, 360p, and audio-only (MP3)
- Format probing after metadata fetch, including:
	- single format selection
	- combined video + audio format selection
- Concurrent downloads with configurable max parallel tasks
- Retry failed tasks or retry all failed tasks from the download list
- Cancel a running task and automatically remove it from the queue
- Real-time logs, progress, speed, ETA, and output path tracking
- Open the downloaded file or its folder directly after completion
- Persistent download history stored locally in SQLite
- Persistent settings for output dir, default quality, language, theme, proxy, rate limit, cookies, notifications, and concurrency
- Built-in yt-dlp update check and in-app update action
- Automatic yt-dlp detection from PATH or the application directory
- English and Simplified Chinese UI

## Usage

1. Paste a video or playlist URL.
2. Click Get Info.
3. Optionally detect formats and choose either a single format or a combined video+audio pair.
4. Choose the output directory.
5. Start download, or enqueue the entire playlist.

## Troubleshooting

- If YouTube asks you to sign in, configure browser cookies or a cookies.txt file in Settings.
- If some YouTube formats are missing, ensure Node.js is installed so yt-dlp can use a supported JS runtime.
- If yt-dlp is missing, install it and click Re-check in the app.

## Requirements

[yt-dlp](https://github.com/yt-dlp/yt-dlp) must be installed and available in your system `PATH`, or placed in the same directory as the YT-GO executable.

```bash
# Install yt-dlp via pip
pip install yt-dlp

# Or via winget (Windows)
winget install yt-dlp

# Or via Homebrew (macOS)
brew install yt-dlp
```

## Downloads

Download the latest release from the [Releases](https://github.com/igeekfan/YT-GO/releases) page.

| Platform | Installer | Portable |
|----------|-----------|----------|
| Windows | `YT-GO_Setup_{version}_windows_x64.exe` | `YT-GO_Portable_{version}_windows_x64.zip` |
| macOS | `YT-GO_{version}_mac_arm64.dmg` / `YT-GO_{version}_mac_intel.dmg` | - |
| Linux | `YT-GO_{version}_linux_amd64.deb` | `YT-GO_{version}_linux_amd64.AppImage` |

## Development

Requirements: Go 1.23+, Node.js 18+, and [Wails CLI](https://wails.io/docs/gettingstarted/installation)

Development mode:

```bash
wails dev
```

Build:

```bash
cd frontend && npm install && npm run build && cd ..
wails build
```

Build outputs are generated in `build/bin/`.

## Roadmap

- Channel-level batch download workflow
- Continued refinement of format selection and media options
- Keep README as the product-facing source of truth while PLAN tracks only future work

## Stack

- [Wails v2](https://wails.io) — Go + React desktop framework
- Go
- React + TypeScript
- Vite
- Tailwind CSS + shadcn/ui
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — video downloading backend

## License

MIT