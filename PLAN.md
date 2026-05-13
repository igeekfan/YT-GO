# YT-GO 开发计划

## 项目现状

YT-GO 是基于 yt-dlp 的视频下载工具，支持 Desktop（Wails）和 Web（HTTP API）双模式运行，共享 `internal/core` 业务逻辑。

### 已具备的能力

- 视频/播放列表解析与格式选择
- 多画质预设 + 手动格式选择（单格式/组合/纯音/纯视频）
- 下载队列（并发控制、进度、取消/重试/删除）
- Cookies 配置（浏览器导入 / 文件上传）
- 代理、字幕、SponsorBlock、缩略图等下载选项
- 抖音专用解析器
- yt-dlp / FFmpeg / Deno / Node.js 依赖检测与更新
- Web 模式：目录浏览、文件下载、Cookies 上传、SSE 事件
- 中英文 i18n、暗色/亮色主题

---

## 待解决问题

按严重程度排列，只列出需要修复/改进的现实问题。

### P0 — Web 模式安全

| 问题 | 说明 |
|------|------|
| 目录浏览无路径限制 | `handleBrowseDir` 可遍历服务器全文件系统，无白名单或沙箱 |
| 文件下载无认证 + 路径遍历风险 | `serveDownloadFile` 仅靠 taskID（UUID）提供文件，无授权检查；`filepath.Clean()` 不能防路径遍历，应验证文件在下载目录内 |
| Settings 保存无服务端验证 | `OutputDir` 可设为系统敏感路径，`Proxy` 有 SSRF 风险，`MaxConcurrent` 无范围检查 |
| StartDownload 无 URL 验证 | 可提交 `file:///etc/passwd` 等非 HTTP 协议 URL |
| 无 CORS 配置 | 前后端分离部署时 API 请求被浏览器拦截 |

### P1 — i18n 缺失

| 问题 | 说明 |
|------|------|
| `douyin.go` ~20 处硬编码中文 | 未走 i18n 系统，en-US 用户看到中文错误消息 |
| `SettingsDialog` 多处硬编码英文 | placeholder（rate limit、subtitle langs、proxy）和诊断标签（Output/Current/Latest）未翻译 |
| `ErrorBoundary` 使用 3 个未定义 i18n key | `errorBoundary.title/retry/reload` 在 zh-CN.ts / en-US.ts 中不存在 |
| 后端 I18n 默认语言硬编码 zh-CN | Service 创建时未读取 Settings 中的语言偏好 |
| `shouldTryPlaylistFallback` 依赖中文匹配 | App.tsx 中用硬编码中文字符串判断错误类型，后端语言不同步时 fallback 逻辑失效 |

### P2 — 功能缺陷

| 问题 | 说明 |
|------|------|
| Web 模式完全跳过 SetupWizard | 首次使用无任何引导，输出目录为空时下载按钮 disabled 但无提示 |
| 抖音解析器不使用用户配置的 cookies | `douyin.go` 的 HTTP 请求未携带用户 cookies，配置了也没用 |
| 抖音 WAF solver defer 泄漏 | `defer resp.Body.Close()` 在 for 循环内，多次迭代会累积未关闭的连接 |

### P3 — 代码质量

| 问题 | 说明 |
|------|------|
| 19 处 `as any` 类型绕过 | `StartDownload` 参数等核心 API 缺乏类型安全 |
| 6 个 Shadcn UI 组件 + radix-ui 未使用 | `badge/card/checkbox/dialog/scroll-area/switch` 和 `@radix-ui` 依赖增加包体积 |
| `formatDuration` 在两处重复 | App.tsx 和 DownloadItem.tsx 各有一份相同实现 |
| App.tsx 中 `QUALITY_OPTIONS` 未使用 | 第 13 行声明但从未引用 |
| 9+ 个废弃 i18n key | `quickSettings.*`、`playlist.detected`、`format.usePreset` 等已弃用但仍保留 |
| EventHub 静默丢弃消息 | channel 缓冲区满时进度更新丢失，前端进度条可能卡住 |
| `CheckForUpdate` / `CheckYtDlpVersion` 无 HTTP 超时 | GitHub 不可达时长时间阻塞 |

---

## 推进优先级

1. **Web 模式安全加固** — 目录浏览加白名单、文件下载加路径校验、Settings 加验证、URL 加协议检查、CORS 支持
2. **i18n 补全** — douyin.go 硬编码中文改造、SettingsDialog placeholder 翻译、ErrorBoundary key 补充、后端 I18n 从 Settings 初始化
3. **功能缺陷修复** — Web 首次使用引导、抖音 cookies 传递、WAF solver 连接泄漏
4. **代码清理** — 删除未使用的 UI 组件和依赖、消除 `as any`、提取公共 utils、清理废弃 i18n key
