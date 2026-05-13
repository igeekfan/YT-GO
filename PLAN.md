# YT-GO 开发计划

## 说明

- README 与 README.zh-CN 负责记录已完成能力、当前产品状态与结构演进。
- PLAN 仅保留未来路线、优先级、阶段目标与推进顺序，避免重复维护。
- 当前规划基于现有 shared-core + desktop/web 双入口架构继续推进。

## 规划原则

- 首页优先服务"分析链接 -> 选择格式 -> 开始下载"的核心路径，减少不必要跳转。
- 高使用频率的设置优先前置，低频维护类能力继续拆离主流程。
- 下载器核心优先做内部解耦，避免后续新增站点、任务类型或 UI 入口时反复穿透 App 层。
- 文档持续分层：README 讲现状，PLAN 讲下一步。

## 下一阶段路线图

| 优先级 | 方向 | 目标 | 状态 |
|--------|------|------|------|
| N1 | 下载器内核下沉 | 拆分 yt-dlp 参数构建、任务生命周期、日志解析、状态更新，形成更清晰的内部组件边界 | ✅ 已完成 |
| N2 | 首页常用设置下沉 | 将默认下载目录、格式偏好、字幕与媒体增强项的高频开关逐步下沉到主页面 | ✅ 已完成 |
| N2.5 | 引入 go-ytdlp 包 | 替换自有命令构建、JSON 解析、进度解析、yt-dlp/ffmpeg 安装与检测逻辑 | ✅ 已完成 |
| N3 | 工具中心独立页面 | 将诊断、更新、环境检测从设置弹窗中继续剥离，形成独立 Tools 页面 | ⏳ 待开始 |
| N4 | 队列管理增强 | 增加任务批量操作、排序控制、失败聚合处理与更明确的队列状态反馈 | ⏳ 待开始 |
| N5 | Web 模式补强 | 对齐 desktop/web 差异，补齐文件选择、下载文件获取、Cookies 上传和运行时提示的体验缺口 | ✅ 已完成 |
| N6 | 文档与发布规范 | 为桌面/Web/Docker 三种运行形态补齐更清晰的使用说明、限制说明与截图更新流程 | ⏳ 待开始 |

## 分阶段建议

### 第一阶段：稳定下载内核

- 优先完成下载器内核下沉，明确参数构建、命令执行、日志解析、状态持久化的责任边界。
- 为后续批量操作和更多下载类型预留统一任务接口。
- 先控制内部复杂度，再继续扩充前端入口。

### 第二阶段：把高频配置拉近主流程

- 将下载目录、格式偏好、字幕与媒体增强开关逐步移动到首页或折叠区域。
- 减少用户在"获取信息"和"开始下载"之间来回进入设置弹窗。
- 基于首页布局重新审视批量下载和单视频下载的默认交互。

### 第三阶段：抽离工具与维护区

- 将诊断、更新、环境检查整合为独立工具中心页面。
- 设置仅保留真正影响下载行为和界面偏好的配置项。
- 为后续增加日志查看、环境修复提示和问题排查入口做准备。

### 第四阶段：完善队列与多端体验

- 增强批量操作、失败重试策略、历史筛选和任务反馈。
- 继续补齐 Web 模式下与桌面端的交互差异。
- 根据运行模式差异整理更明确的限制提示和文档说明。

## 已完成的技术替换

### 引入 go-ytdlp 包替换自有实现（已完成）

- 包地址：[github.com/lrstanley/go-ytdlp](https://github.com/lrstanley/go-ytdlp) (v1.3.5)
- 已替换内容：
  - `executor.go`: 命令构建改用 `ytdlp.New()` builder 链式调用；`CheckYtDlp()`/`UpdateYtDlp()`/`CheckYtDlpVersion()` 使用 go-ytdlp 的 `Install()`/`Version()`/`Update()`；新增 `InstallYtDlp()` 自动下载安装
  - `ytdlp.go`: `GetVideoInfo()`/`GetPlaylistInfo()`/`GetFormats()` 使用 `DumpJSON()`/`DumpSingleJSON()` + `ExtractedInfo` 类型安全解析，替代手动 `map[string]interface{}` JSON 解析
  - `downloads.go`: 下载执行使用 go-ytdlp builder + `ProgressFunc()` 进度回调 + `StderrFunc()` 日志回调，替代手动 `exec.Cmd` + `lineWriter`
  - `diagnostics.go`: yt-dlp/ffmpeg 检测使用 `ytdlp.Install()`/`ytdlp.InstallFFmpeg()` (DisableDownload 模式)，替代手动 PATH/WinGet/Scoop 搜索
  - `service.go`: 移除 `ytdlpPath` 字段和 `HideCommand` hook，改用 go-ytdlp 内部缓存和 Windows CMD 隐藏
  - `progress.go`: 移除 `lineWriter`（已由 go-ytdlp 的 `timestampWriter` 替代），仅保留正则常量和辅助函数
  - `desktop/app.go`: 移除 `HideCommand` hook
- 保留不变：
  - `jsruntime.go`: Deno/Node 运行时检测逻辑保持独立（go-ytdlp 的 bun 安装是可选的，我们仍需要 Deno/Node 检测用于 YouTube 场景）
  - `douyin.go`: 抖音专用下载逻辑独立于 yt-dlp，无需改动
  - `errhint.go`: 错误增强逻辑保持不变
  - `platform/` 包: 仍被 jsruntime.go 用于 Deno 安装时的 CMD 隐藏

## 当前推荐推进顺序

1. ~~先做下载器内核下沉，避免新功能继续堆在现有调用链上。~~ ✅
2. ~~评估并引入 go-ytdlp 包，替换自有命令构建与安装更新逻辑。~~ ✅
3. ~~Web 模式补强：文件选择、下载文件获取、Cookies 上传、运行时提示对齐。~~ ✅
4. 再做首页常用设置下沉，优化高频下载流程。
5. 然后拆出独立工具中心，收敛低频维护功能。
6. 最后补齐队列管理增强与文档发布规范。

## 已完成的 Web 模式补强（N5）

### 后端新增 API 端点

- `POST /api/settings/browse-dir` — 列出服务器端目录内容，供 Web 端目录浏览
- `POST /api/cookies/upload` — 上传 cookies.txt 文件到服务器，返回服务器端路径
- `GET /api/downloads/{id}/file` — 下载已完成的文件（Web 端获取下载成果）
- `POST /api/ytdlp/install` — 安装 yt-dlp（Web 端诊断工具使用）
- `GET /api/diagnostics/deps` — 获取依赖状态

### Service 新增方法

- `GetDataDir()` — 返回应用数据目录路径
- `GetDownload(id)` — 按 ID 获取单个下载任务

### 前端 backend.ts 改进

- `SelectFolder()` — Web 模式返回空字符串，UI 改为服务器端路径文本输入
- `BrowseDir(path)` — 新增，调用 `/api/settings/browse-dir` 列出服务器端目录
- `UploadCookiesFile(file)` — 新增，上传 cookies 文件到服务器
- `getDownloadFileURL(taskID)` — 新增，返回 Web 端下载文件的 URL
- `InstallYtDlp()` — 新增，Web 端安装 yt-dlp

### 前端 UI 适配

- `DownloadItem` — Web 模式下"打开文件/文件夹"按钮替换为"下载文件"链接
- `SettingsDialog` — Web 模式下隐藏"从浏览器导入 Cookies"选项，浏览按钮替换为上传按钮
- `SetupWizard` — Web 模式下隐藏浏览器 cookies 选择器，支持 cookies 文件上传
- `App.tsx` — Web 模式下隐藏目录浏览按钮，改为纯文本输入
- 所有目录输入框在 Web 模式显示服务器端路径提示

### i18n 新增

- `action.download` / `action.upload` — 下载文件/上传按钮文本
- `outputDir.serverPathPlaceholder` — 服务器端路径占位符
- `settings.cookiesFileWebHint` — Web 模式 cookies 上传提示
- `settings.outputDirWebHint` — Web 模式目录输入提示

### 其他修复

- `main_web.go` — 移除已废弃的 `HideCommand` hook 和 `platform` import
- `desktop/app_bindings.go` — 新增 `InstallYtDlp` 桌面端绑定
