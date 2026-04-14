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
- 通用站点支持：兼容 yt-dlp 支持的任意网站（Bilibili、Twitter/X 等）
- 下载前展示视频标题、上传者、时长、平台和缩略图
- 自动识别播放列表或频道视频列表，并支持批量加入下载任务
- 播放列表条目选择器：支持全选/全不选和逐条勾选
- 预设画质支持：最佳画质、1080p、720p、480p、360p、仅音频（MP3）
- 获取格式后支持两种精细选择方式：
	- 单格式选择，带类型标签 [V+A]、[V]、[A]、分辨率、帧率、编码和文件大小
	- 视频轨 + 音频轨组合选择
	- 智能格式排序：按类型、分辨率、码率排序
- 支持配置最大并发数并同时下载多个任务
- 下载记录搜索与按状态筛选（全部、下载中、已完成、失败）
- 支持单任务重试、重新下载已完成任务和批量重试失败任务
- 取消中的任务会自动从任务列表中移除
- 实时展示日志、进度、速度、预计剩余时间和输出路径
- 字幕下载：支持配置字幕语言和可选嵌入视频
- 章节信息写入与 SponsorBlock 标记
- 可选同时保存视频描述文件与缩略图文件
- 自定义文件名模板、输出容器格式（MP4/MKV/WebM）、音频格式（MP3/M4A/Opus/FLAC/WAV）
- 下载完成后可直接打开文件或所在文件夹
- 使用 SQLite 持久化下载历史记录
- 设置分为五个标签页：下载设置、媒体选项、网络与认证、工具与维护、外观与语言
- 工具中心：yt-dlp 更新、FFmpeg 检测、Node.js 运行时状态和完整诊断
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
- **Cookies 配置方式**：
  - **从浏览器导入**：设置 → 网络与认证 → Cookies 来源 → 选择你的浏览器（Chrome、Firefox、Edge 等），无需导出文件。
  - **cookies.txt 文件**：使用浏览器插件如“Get cookies.txt LOCALLY”（Chrome）或“cookies.txt”（Firefox）导出 YouTube cookies，然后在设置 → 网络与认证 → Cookies 文件 中配置该文件路径。
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

详见 [PLAN.md](PLAN.md) 了解详细开发路线图和未来工作项。

## 技术栈

- [Wails v2](https://wails.io) — Go + React 桌面框架
- Go
- React + TypeScript
- Vite
- Tailwind CSS + shadcn/ui
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — 视频下载后端

## License

MIT
