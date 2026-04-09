# YT-GO 开发计划

## 技术架构

```
Go 后端                                 React 前端
┌──────────────────────────┐            ┌──────────────────────────────┐
│ types.go                 │            │ App.tsx (主布局)              │
│ - YtDlpStatus            │            │ ├── URL 输入区                │
│ - VideoInfo              │  Wails     │ ├── 视频信息预览卡片           │
│ - DownloadRequest        │ ◄────────► │ ├── 画质 / 目录选择           │
│ - DownloadTask           │  Bind      │ └── DownloadList.tsx          │
│                          │            │     └── DownloadItem.tsx      │
│ app.go                   │            └──────────────────────────────┘
│ - CheckYtDlp()           │
│ - GetVideoInfo()         │            事件总线
│ - GetDefaultDownloadDir()│            ┌─────────────────┐
│ - SelectFolder()         │            │ download:update │
│ - StartDownload()        │ ──────────►│ (DownloadTask)  │
│ - CancelDownload()       │            └─────────────────┘
│ - GetDownloads()         │
│ - ClearCompleted()       │
│ - OpenFolder()           │
│ - OpenFile()             │
│ - SetLang() / GetLang()  │
└──────────────────────────┘
```

---

## 当前状态

当前已实现功能以 [README.md](README.md) 为准，PLAN 仅保留后续开发方向和仍未完成的事项，避免文档重复维护。

## 已完成的核心功能（P0）

| 功能 | 说明 |
|------|------|
| yt-dlp 检测 | 自动从 PATH 或可执行目录查找 yt-dlp，展示版本信息 |
| 视频信息获取 | 通过 `yt-dlp --dump-json` 获取标题、封面、时长、上传者、平台 |
| 画质选择 | 支持 best / 1080p / 720p / 480p / 360p / 仅音频(MP3) |
| 下载目录选择 | 默认 Downloads 目录，支持浏览器对话框选择 |
| 实时下载进度 | 百分比、速度、剩余时间、文件大小实时更新 |
| 取消下载 | 随时取消进行中的任务 |
| 多任务管理 | 同时管理多个下载任务，清除已完成项 |
| 打开文件/文件夹 | 完成后可直接打开文件或所在目录 |
| 多语言 | 支持中文/英文界面切换 |
| 主题切换 | 支持深色/浅色主题，设置持久化到 localStorage |

## 待办

| 优先级 | 事项 | 说明 | 状态 |
|--------|------|------|------|
| P0 | 日志输出 | 前台可直接看到命令行执行的日志 | ✅ 已完成 |
| P0 | 设置持久化 | 增加设置项（代理、限速、通知等），使用 SQLite 持久化 | ✅ 已完成 |
| P1 | 下载历史持久化 | 任务列表使用 SQLite 持久化到 `%APPDATA%/YT-GO/history.db` | ✅ 已完成 |
| P1 | 播放列表支持 | 支持输入播放列表 URL，批量添加下载任务 | ✅ 已完成 |
| P2 | 格式/画质探测 | 调用 `yt-dlp --dump-json` 展示目标视频实际可用格式，供用户精确选择 | ✅ 已完成 |
| P2 | yt-dlp 自动更新 | 应用内检测 yt-dlp 新版本并一键更新 | ✅ 已完成 |
| P2 | 下载限速 | 支持设置最大下载速度（`--rate-limit`） | ✅ 已完成 |
| P3 | 通知提醒 | 下载完成后发送系统桌面通知（Web Notification API） | ✅ 已完成 |
| P3 | 代理设置 | 支持配置 HTTP/SOCKS5 代理并传递给 yt-dlp（`--proxy`） | ✅ 已完成 |

## 后续方向

- 批量能力：频道批量下载，以及多任务并发数控制
- 格式能力：进一步优化格式选择 UI，支持组合选择（视频+音频）
- 工具管理：内置 yt-dlp 安装引导
- 文档维护：产品功能说明集中放在 README，PLAN 只记录后续规划和未完成项
