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

当前已实现功能统一维护在 [README.md](README.md) 与 [README.zh-CN.md](README.zh-CN.md) 中，PLAN 只保留未来工作项、优先级和推进顺序，避免重复维护。

## 后续任务队列

| 优先级 | 方向 | 说明 | 状态 |
|--------|------|------|------|
| P1 | 批量能力：频道批量下载 | 支持频道页、频道视频列表、频道 shorts 列表的批量获取与批量下载 | 进行中 |
| P2 | 格式能力：继续打磨选择体验 | 在已支持单格式/组合格式的基础上，继续优化筛选、排序、默认推荐与可读性 | 待开始 |
| P3 | 文档维护 | README 作为产品文档主入口，PLAN 仅跟踪未来路线、优先级与里程碑 | 持续进行 |

## 后续方向

- 下一个核心目标：频道批量下载
- 次级优化方向：格式选择体验继续打磨
- 文档策略：README 记录已完成能力，PLAN 只记录未来工作
