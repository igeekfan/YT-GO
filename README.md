# ytgo

[English](README.md) | [简体中文](README.zh-CN.md)

ytgo is a cross-platform desktop YouTube downloader powered by [yt-dlp](https://github.com/yt-dlp/yt-dlp). Paste a URL, pick a quality, choose an output folder, and download — no command line needed.

## Features

- Paste any YouTube (or yt-dlp supported) URL and fetch video metadata with one click
- Choose download quality: Best, 1080p, 720p, 480p, 360p, or Audio-only (MP3)
- Select a custom output directory or use the default Downloads folder
- Real-time download progress: percentage, speed, ETA, and file size
- Cancel running downloads at any time
- Manage multiple downloads simultaneously with a task list
- Displays video title, uploader, duration, and thumbnail before downloading
- Switch between English and Chinese interface
- Automatic yt-dlp detection from PATH or the application directory

## Requirements

[yt-dlp](https://github.com/yt-dlp/yt-dlp) must be installed and available in your system `PATH`, or placed in the same directory as the ytgo executable.

```bash
# Install yt-dlp via pip
pip install yt-dlp

# Or via winget (Windows)
winget install yt-dlp

# Or via Homebrew (macOS)
brew install yt-dlp
```

## Downloads

Download the latest release from the [Releases](https://github.com/igeekfan/ytgo/releases) page.

| Platform | Installer | Portable |
|----------|-----------|----------|
| Windows | `ytgo_Setup_{version}_windows_x64.exe` | `ytgo_Portable_{version}_windows_x64.zip` |
| macOS | `ytgo_{version}_mac_arm64.dmg` / `ytgo_{version}_mac_intel.dmg` | - |
| Linux | `ytgo_{version}_linux_amd64.deb` | `ytgo_{version}_linux_amd64.AppImage` |

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

## Stack

- [Wails v2](https://wails.io) — Go + React desktop framework
- Go
- React + TypeScript
- Vite
- Tailwind CSS + shadcn/ui
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — video downloading backend

## License

MIT