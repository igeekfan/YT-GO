# YT-GO

[English](README.md) | [简体中文](README.zh-CN.md)

YT-GO is a cross-platform desktop video downloader powered by [yt-dlp](https://github.com/yt-dlp/yt-dlp). Simply paste a URL, pick a quality, and download — no command line needed.

## Supported Platforms

YT-GO inherits **all platforms supported by yt-dlp** (1800+ sites), including:

- **Video**: YouTube, TikTok, 抖音 (Douyin), Bilibili, Twitter/X, Instagram, Facebook, Vimeo, Dailymotion, etc.
- **Music**: Spotify, SoundCloud, Apple Music, YouTube Music, etc.
- **Live**: Twitch, YouTube Live, etc.

## Features

- One-click metadata fetch for video, playlist, and channel URLs
- Preview with title, uploader, duration, platform, and thumbnail
- **Preset qualities**: Best, 1080p, 720p, 480p, 360p, and audio-only (MP3)
- Format probing with single or combined video+audio selection
- Batch download for playlists and channels with item selection
- Concurrent downloads with configurable parallelism
- Download history with search, filter, retry, and re-download
- Real-time progress, speed, ETA tracking
- Subtitle download with language selection and optional embedding
- Chapter embedding and SponsorBlock markers
- Sidecar files: thumbnail and description export
- Proxy, cookies, custom filename, and container format support
- Persistent settings: output directory, download options, appearance themes
- Built-in yt-dlp and FFmpeg detection with one-click update
- English and Simplified Chinese UI

## Cookie Export (Required for Douyin/TikTok)

1. Install browser extension [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/fcnalolhneacngmkjfgnmalmefjancoh)
2. Open douyin.com and log in, play any video
3. Click the extension icon → Export as Netscape format (e.g., `E:\cookies.txt`)
4. Configure the path in Settings → Network & Auth

## Usage

1. Paste a video, playlist, or supported URL.
2. Click **Get Info**.
3. Choose a quality preset or select a specific format.
4. Set the output directory.
5. Click **Download**.

## Requirements

[yt-dlp](https://github.com/yt-dlp/yt-dlp) must be installed:

```bash
# via pip
pip install yt-dlp

# Windows
winget install yt-dlp

# macOS
brew install yt-dlp
```

## Downloads

| Platform | Installer | Portable |
|----------|-----------|----------|
| Windows | `YT-GO_Setup_{version}_windows_x64.exe` | `YT-GO_Portable_{version}_windows_x64.zip` |
| macOS | `YT-GO_{version}_mac_arm64.dmg` / `YT-GO_{version}_mac_intel.dmg` | - |
| Linux | `YT-GO_{version}_linux_amd64.deb` | `YT-GO_{version}_x86_64.AppImage` |

Get the latest release from [Releases](https://github.com/igeekfan/YT-GO/releases).

### macOS Security Prompt

The macOS DMG is currently not notarized by Apple. If macOS says `YTGO.app` is damaged, cannot be verified, or should be moved to the Trash:

1. Drag `YTGO.app` into **Applications**.
2. Open the DMG again and double-click **Repair**.
3. Enter your macOS login password when prompted.
4. Open `YTGO.app` from Applications.

## Dev

```bash
wails dev
```

## Build

```bash
# Desktop app
wails build

# Web server
go build -tags web -o build/bin/yt-go-web .
```

## Docker

The Docker image runs YT-GO in web mode and includes yt-dlp, Deno, and a compact shared FFmpeg/FFprobe distribution. Node.js and the Go toolchain are used only during compilation and are not included in the runtime image.

### Secure deployment

```bash
cp .env.example .env
# Edit .env and set a long, random YTGO_AUTH_TOKEN first.
docker compose up -d --build
```

Open `http://localhost:8080`. Compose refuses to start without `YTGO_AUTH_TOKEN`. Settings/caches and downloads are persisted in the `ytgo-config` and `ytgo-downloads` named volumes. The container runs as an unprivileged user with all Linux capabilities dropped, a read-only root filesystem, `no-new-privileges`, and configurable CPU, memory, and PID limits.

To expose the service through a TLS reverse proxy, also set its external URL and, only when needed, the exact cross-origin frontend origin:

```dotenv
YTGO_AUTH_TOKEN=replace-with-a-strong-random-token
YTGO_EXTERNAL_URL=https://yt.example.com
```

Optional Compose variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `YTGO_BIND_ADDRESS` | Host address used by the published Compose port | `127.0.0.1` |
| `YTGO_PORT` | Host port | `8080` |
| `YTGO_VERSION` | Image/app version used for a local build | `0.0.0` |
| `YTGO_AUTH_TOKEN` | Web login token; required by Compose | none |
| `YTGO_EXTERNAL_URL` | Public base URL for download links | empty |
| `YTGO_CORS_ORIGIN` | Allowed cross-origin frontend URL | empty |
| `YTGO_CPU_LIMIT` | Container CPU limit | `2.0` |
| `YTGO_MEMORY_LIMIT` | Container memory limit | `2g` |
| `YTGO_PIDS_LIMIT` | Container PID limit | `256` |
| `TZ` | Container timezone | `Asia/Shanghai` |
| `NODE_IMAGE` | Frontend builder base image | `docker.m.daocloud.io/library/node:22-alpine` |
| `GO_IMAGE` | Backend builder base image | `docker.m.daocloud.io/library/golang:1.25-alpine` |
| `RUNTIME_IMAGE` | Runtime base image | `docker.m.daocloud.io/library/debian:bookworm-slim` |

Compose defaults to a Docker Hub mirror for reliable builds in mainland China. Set the three image variables in `.env` to use another registry; direct Dockerfile and CI builds still default to the official images. The published image supports `linux/amd64` and `linux/arm64`.

Compose publishes the port on host loopback by default. Set `YTGO_BIND_ADDRESS=0.0.0.0` only when direct network access is intentional and protect it with TLS at a reverse proxy. For a bare web build, the default listener is `127.0.0.1:8080` and the default web download root is `~/Downloads`. Use `YTGO_WEB_DOWNLOAD_ROOT` to choose another root. A non-loopback `YTGO_WEB_ADDR` is rejected unless `YTGO_AUTH_TOKEN` is set. Every API-selected output directory must remain under the configured root, and filename templates cannot contain paths.

The API rejects initial URLs that resolve to loopback, private, link-local, reserved, or common metadata endpoints. yt-dlp is a separate process and performs its own DNS lookups and redirects, so application validation cannot prevent later DNS rebinding or redirects by itself. Public deployments must also enforce outbound firewall/container egress rules that deny private, link-local, and cloud metadata networks while allowing normal public video sites. Rebuild the image to update bundled yt-dlp/Deno/FFmpeg; the hardened non-root container intentionally cannot replace system binaries in place.

## Troubleshooting

- **Missing formats**: Ensure Deno 2 or later is installed for JS runtime support. Deno is already included in the Docker image.
- **yt-dlp missing**: Place it in the app directory or click Re-check in Tools.

## Stack

- [Wails v2](https://wails.io) — Go + React desktop
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — video downloading backend

## License

MIT

## Disclaimer

By downloading or using this project, you agree to the following terms:

**For Learning & Research Only**
This project is for personal learning, research, and data management only. Commercial use or any illegal purposes are strictly prohibited.

**Legal Compliance**
You must comply with all applicable laws including but not limited to: cybersecurity laws, data protection laws, privacy laws, copyright laws, and the platform's Terms of Service and Privacy Policy.

**Respect Copyright & Privacy**
All downloaded content copyright belongs to the original creators. Without authorization, do not use downloaded content for redistribution, commercial purposes, or any infringing activities. Do not download or share content involving others' privacy.

**No Abuse**
Do not use this tool for: large-scale data scraping, disrupting platform operations, bypassing security mechanisms, spreading illegal content, or harassing creators.

**Data Security**
This tool does not collect, upload, or share any user data. Cookies are stored locally only—do not share them publicly.

**Account Risk**
Using automation tools may violate platform Terms of Service and could result in account suspension. You assume all risks.

**Platform Rules First**
Platforms reserve the right to adjust APIs and anti-crawling policies. Please respect platform rules, do not send high-frequency requests, and keep download intervals above the default rate limit.

**No Warranty**
This software is provided "AS IS" without warranties. The author is not liable for any consequences including account suspension, data loss, legal disputes, or losses caused by using this tool.

**If you do not agree to these terms, stop using this project immediately.**

## Gallery

| <a href="images/en-US/start-1.png"><img src="images/en-US/start-1.png" width="200"/></a> | <a href="images/en-US/start-2.png"><img src="images/en-US/start-2.png" width="200"/></a> | <a href="images/en-US/getinfo.png"><img src="images/en-US/getinfo.png" width="200"/></a> |
|:---:|:---:|:---:|
| Main Interface | Playlist Selection | Get Info |

| <a href="images/en-US/setting-download.png"><img src="images/en-US/setting-download.png" width="200"/></a> | <a href="images/en-US/setting-network.png"><img src="images/en-US/setting-network.png" width="200"/></a> | <a href="images/en-US/light.png"><img src="images/en-US/light.png" width="200"/></a> |
|:---:|:---:|:---:|
| Download Settings | Network Settings | Light Theme
