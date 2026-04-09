# YT-GO

[English](README.md) | [简体中文](README.zh-CN.md)

YT-GO 是一个跨平台桌面 YouTube 下载工具，基于 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 驱动。粘贴链接、选择画质、指定保存目录，一键下载，无需命令行。

## 项目概览

YT-GO 聚焦于桌面端的下载工作流，核心目标是把常用的 yt-dlp 能力收敛到一个直接可用的图形界面里：

- 下载前先获取视频、播放列表或频道视频列表元信息
- 支持预设画质、单格式选择、视频+音频组合格式选择
- 在一个任务列表里同时管理多个下载任务
- 将代理、Cookies、限速、通知、并发等实用设置本地持久化

## 功能概览

- 一键获取 YouTube 视频、播放列表和频道视频列表元信息
- 下载前展示视频标题、上传者、时长、平台和缩略图
- 自动识别播放列表或频道视频列表，并支持批量加入下载任务
- 预设画质支持：最佳画质、1080p、720p、480p、360p、仅音频（MP3）
- 获取格式后支持两种精细选择方式：
	- 单格式选择
	- 视频轨 + 音频轨组合选择
- 支持配置最大并发数并同时下载多个任务
- 支持单任务重试和批量重试失败任务
- 取消中的任务会自动从任务列表中移除
- 实时展示日志、进度、速度、预计剩余时间和输出路径
- 下载完成后可直接打开文件或所在文件夹
- 使用 SQLite 持久化下载历史记录
- 使用 SQLite 持久化设置项：输出目录、默认画质、语言、主题、代理、限速、Cookies、通知、并发数
- 应用内检测 yt-dlp 版本并支持更新
- 自动从系统 PATH 或应用程序目录检测 yt-dlp
- 支持中英文界面切换

## 使用方式

1. 粘贴视频链接、播放列表链接，或支持的频道视频列表链接。
2. 点击“获取信息”。
3. 如有需要，检测格式并选择单格式或视频+音频组合格式。
4. 选择保存目录。
5. 开始下载，或将识别出的整个集合批量加入下载任务。

## 常见问题

- 如果 YouTube 提示需要登录验证，请在设置中配置浏览器 Cookies 或 cookies.txt 文件。
- 如果 YouTube 可用格式不完整，请确认本机安装了 Node.js，以便 yt-dlp 使用可用的 JS runtime。
- 如果未检测到 yt-dlp，请先安装后再点击应用内的“重新检测”。

## 前置要求

需要安装 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 并确保其在系统 `PATH` 中可用，或将其放置在 YT-GO 可执行文件的同一目录下。

```bash
# 通过 pip 安装
pip install yt-dlp

# Windows 通过 winget 安装
winget install yt-dlp

# macOS 通过 Homebrew 安装
brew install yt-dlp
```

## 下载

可在 [Releases](https://github.com/igeekfan/YT-GO/releases) 页面获取对应平台版本。

| 平台 | 安装包 | 便携版 |
|------|--------|--------|
| Windows | `YT-GO_Setup_{version}_windows_x64.exe` | `YT-GO_Portable_{version}_windows_x64.zip` |
| macOS | `YT-GO_{version}_mac_arm64.dmg` / `YT-GO_{version}_mac_intel.dmg` | - |
| Linux | `YT-GO_{version}_linux_amd64.deb` | `YT-GO_{version}_linux_amd64.AppImage` |

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

## 路线图

- 继续打磨格式选择和媒体选项体验
- README 作为产品功能说明主入口，PLAN 仅保留后续规划

## 技术栈

- [Wails v2](https://wails.io) — Go + React 桌面框架
- Go
- React + TypeScript
- Vite
- Tailwind CSS + shadcn/ui
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — 视频下载后端

## License

MIT
