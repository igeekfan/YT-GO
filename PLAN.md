# ytgo 开发计划

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

## 待办

| 优先级 | 事项 | 说明 |
|--------|------|------|
| P1 | 下载历史持久化 | 当前任务列表仅保存在内存，重启后丢失；需写入本地存储（SQLite 或 JSON） |
| P1 | 播放列表支持 | 支持输入播放列表 URL，批量添加下载任务，显示总进度 |
| P2 | 格式/画质探测 | 调用 `yt-dlp --list-formats` 展示目标视频实际可用格式，供用户精确选择 |
| P2 | yt-dlp 自动更新 | 应用内检测 yt-dlp 新版本并一键更新，减少用户手动维护 |
| P2 | 下载限速 | 支持设置最大下载速度，避免占满带宽 |
| P3 | 通知提醒 | 下载完成后发送系统桌面通知 |
| P3 | 代理设置 | 支持配置 HTTP/SOCKS5 代理并传递给 yt-dlp |

## 后续方向

- 持久化：下载记录写磁盘，重启后可恢复历史任务状态
- 批量能力：支持播放列表、频道批量下载，以及多任务并发数控制
- 格式能力：动态获取视频的可用格式列表，精确选择编码/分辨率
- 工具管理：内置 yt-dlp 版本检测与升级，降低外部依赖门槛
- 网络设置：代理、限速等网络参数配置
- 文档维护：产品功能说明集中放在 README，PLAN 只记录后续规划和未完成项
