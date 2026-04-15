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
| Linux | `YT-GO_{version}_linux_amd64.deb` | `YT-GO_{version}_linux_amd64.AppImage` |

Get the latest release from [Releases](https://github.com/igeekfan/YT-GO/releases).

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

```bash
docker build -t yt-go:local .
docker run --rm -p 8080:8080 yt-go:local
```

## Troubleshooting

- **Missing formats**: Ensure Node.js is installed for JS runtime support.
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