# ytgo

[English](README.md) | [简体中文](README.zh-CN.md)

ytgo 是一个跨平台桌面 YouTube 下载工具，基于 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 驱动。粘贴链接、选择画质、指定保存目录，一键下载，无需命令行。

## 功能概览

- 粘贴任意 YouTube（或 yt-dlp 支持的站点）链接，一键获取视频元信息
- 支持多种下载画质：最高画质、1080p、720p、480p、360p 及纯音频（MP3）
- 自定义输出目录，或使用系统默认下载文件夹
- 实时显示下载进度：百分比、速度、预计剩余时间和文件大小
- 随时取消正在进行的下载任务
- 同时管理多个下载任务，任务列表一目了然
- 下载前展示视频标题、上传者、时长和缩略图
- 支持中英文界面切换
- 自动从系统 PATH 或应用程序目录检测 yt-dlp

## 前置要求

需要安装 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 并确保其在系统 `PATH` 中可用，或将其放置在 ytgo 可执行文件的同一目录下。

```bash
# 通过 pip 安装
pip install yt-dlp

# Windows 通过 winget 安装
winget install yt-dlp

# macOS 通过 Homebrew 安装
brew install yt-dlp
```

## 下载

可在 [Releases](https://github.com/igeekfan/ytgo/releases) 页面获取对应平台版本。

| 平台 | 安装包 | 便携版 |
|------|--------|--------|
| Windows | `ytgo_Setup_{version}_windows_x64.exe` | `ytgo_Portable_{version}_windows_x64.zip` |
| macOS | `ytgo_{version}_mac_arm64.dmg` / `ytgo_{version}_mac_intel.dmg` | - |
| Linux | `ytgo_{version}_linux_amd64.deb` | `ytgo_{version}_linux_amd64.AppImage` |

## 开发

前置环境：Go 1.23+、Node.js 18+、[Wails CLI](https://wails.io/docs/gettingstarted/installation)

开发模式：

```bash
wails dev
```

构建：

```bash
cd frontend && npm install && npm run build && cd ..
wails build
```

构建产物位于 `build/bin/`。

## 技术栈

- [Wails v2](https://wails.io) — Go + React 桌面框架
- Go
- React + TypeScript
- Vite
- Tailwind CSS + shadcn/ui
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — 视频下载后端

## License

MIT
