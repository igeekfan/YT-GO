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

## 可借鉴的架构设计（来自 Tiny RDM）

> [tiny-craft/tiny-rdm](https://github.com/tiny-craft/tiny-rdm) 同为 Wails v2 + Go + Vue 项目，其非业务层的设计对 YT-GO 有直接参考价值。

### 1. Vite Alias 透明双模式切换

**Tiny RDM 做法：** 通过 `vite.config.js` 的 `resolve.alias`，在 Web 构建时将所有 `wailsjs/*` 导入重定向到 Web 适配层，业务代码零改动。

```js
// Tiny RDM vite.config.js
resolve: {
    alias: {
        ...(isWeb ? {
            'wailsjs/runtime/runtime.js': 'src/utils/wails_runtime.js',
            'wailsjs/go/services/connectionService.js': 'src/utils/api.js',
            // ... 所有 service 统一指向 api.js
        } : {}),
    }
}
```

**YT-GO 现状：** 每个 API 函数内部用 `getDesktop()` 运行时判断模式，导致 backend.ts 充满条件分支。

**借鉴方向：** 将 `backend.ts` 拆分为 `backend_desktop.ts` 和 `backend_web.ts`，通过 Vite alias 在构建时替换，消除运行时分支。前端其他文件无需感知模式。

---

### 2. WebSocket 事件适配层（替代 SSE）

**Tiny RDM 做法：** Web 模式用 WebSocket 替代 Wails Events，实现 `wails_runtime.js` 适配层，提供与 Wails 完全一致的 `EventsOn/EventsOff/EventsEmit` API。支持自动重连、断线恢复。

```js
// Tiny RDM wails_runtime.js (web stub)
export function EventsOn(event, callback) { onWsEvent(event, callback) }
export function EventsEmit(event, ...data) { sendWsMessage({ event, data }) }
```

**YT-GO 现状：** Web 模式用 SSE（单向），`runtime.ts` 对两种模式做了抽象但底层差异大。SSE 无法双向通信，无法 emit 事件给后端。

**借鉴方向：** 如未来需要前端→后端的实时通信（如取消下载确认、实时配置同步），可升级为 WebSocket 方案，参考 Tiny RDM 的适配层模式。

---

### 3. Web 模式安全中间件

**Tiny RDM 做法：** Gin 中间件统一注入安全头：

```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "SAMEORIGIN")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Content-Security-Policy", "default-src 'self'; ...")
        c.Next()
    }
}
```

**YT-GO 现状：** HTTP API 无任何安全头，无 CORS 配置，无 CSP。

**借鉴方向：** 在 `httpapi/server.go` 中添加类似中间件，配置 CSP、X-Frame-Options、CORS 等。**优先级高（PLAN P0）。**

---

### 4. Web 模式认证系统

**Tiny RDM 做法：** Web 模式启动时检查 `/api/auth/status`，未认证则显示 LoginPage。登录后重连 WebSocket 恢复状态。Desktop 模式跳过认证。使用 `defineAsyncComponent` 按需加载 LoginPage 避免影响 Desktop 包体积。

**YT-GO 现状：** Web 模式完全无认证，任何人可访问所有 API。

**借鉴方向：** 至少支持环境变量配置简单 token 认证或 basic auth。对敏感操作（下载、目录浏览、设置修改）做鉴权。**优先级高（PLAN P0）。**

---

### 5. 构建时平台分离（Build Tags + 文件后缀）

**Tiny RDM 做法：** Go 端用文件名区分平台：
- `platform_desktop.go` (`//go:build !web`) — Wails runtime 调用
- `platform_web.go` (`//go:build web`) — WebSocket/HTTP stub
- 两者导出相同函数签名，业务层无感知

**YT-GO 现状：** 已有类似模式（`main.go` / `main_web.go`，`cmd_windows.go` / `cmd_stub.go`），但 `httpapi/` 和 `core/` 的平台差异处理不够统一。

**借鉴方向：** 将 `platform/` 目录扩展，统一处理所有平台差异函数（如 `EventsEmit`、`OpenFileDialog`），避免散落在各处。

---

### 6. 状态管理（Pinia Stores）

**Tiny RDM 做法：** 用 Pinia store 按职责分离状态：
- `connections.js` — 连接列表
- `browser.js` — 键值浏览
- `preferences.js` — 偏好设置
- `tab.js` — 标签页管理
- `dialog.js` — 对话框状态

**YT-GO 现状：** 所有状态集中在 App.tsx 的 ~30 个 `useState` 中，组件间传递靠 props drilling。

**借鉴方向：** 考虑引入 Zustand 或 Jotai（比 Pinia 更轻量，适配 React），将设置、下载队列、视频信息等状态抽离为独立 store。App.tsx 可缩减 50%+。

---

### 7. 全局错误处理

**Tiny RDM 做法：** Vue 全局 errorHandler 捕获未处理异常，弹出通知：

```js
app.config.errorHandler = (err, instance, info) => {
    $notification.error(err.toString(), { title: 'Error' })
}
```

**YT-GO 现状：** 有 ErrorBoundary 组件但 i18n key 未定义（`errorBoundary.title/retry/reload`），错误信息可能显示空白。

**借鉴方向：** 补全 ErrorBoundary i18n key，增加 console 错误日志上报能力。

---

### 8. Vite 自动导入插件

**Tiny RDM 做法：** 使用 `unplugin-auto-import` 和 `unplugin-vue-components` 自动导入 Naive UI 组件和 API，减少手动 import。

**YT-GO 现状：** React 项目，手动 import 所有 shadcn/ui 组件。尚可接受，但如果组件数量增长，可考虑类似方案。

**借鉴方向：** 暂不需要。当前手动 import 清晰可控。

---

### 9. Docker 多阶段构建

**Tiny RDM 做法：** 前端构建和后端构建分阶段，用 `--platform=linux/amd64` 指定平台，`CGO_ENABLED=0` 静态编译。

**YT-GO 现状：** 已有 Dockerfile，但未指定 `--platform`，未传入版本号。

**借鉴方向：** 可参考 Tiny RDM 的 Dockerfile 结构，增加 `ARG APP_VERSION` 传入版本号，指定构建平台。

---

### 10. 窗口状态持久化

**Tiny RDM 做法：** 启动时读取上次窗口大小和最大化状态，关闭时保存。

```go
windowWidth, windowHeight, maximised := prefSvc.GetWindowSize()
```

**YT-GO 现状：** 未持久化窗口状态。

**借鉴方向：** 在 Settings 中增加 `windowWidth`/`windowHeight`/`maximized` 字段，Wails startup 时恢复。

---

### 优先级排序

| 优先级 | 项目 | 说明 |
|--------|------|------|
| 🔴 P0 | Web 安全中间件 | 安全头 + CORS + CSP |
| 🔴 P0 | Web 认证 | 至少 token/basic auth |
| 🟡 P1 | Vite alias 双模式 | 消除 backend.ts 条件分支 |
| 🟡 P1 | 状态管理抽离 | 减少 App.tsx 复杂度 |
| 🟢 P2 | ErrorBoundary i18n | 补全 key |
| 🟢 P2 | 窗口状态持久化 | 提升桌面体验 |
| 🟢 P2 | Dockerfile 优化 | 版本号 + 平台指定 |
| ⚪ P3 | WebSocket 事件 | 仅在需要双向通信时 |
| ⚪ P3 | 自动导入插件 | 组件多时再考虑 |

---

## 架构改进实现路线图

### Phase 1 — Web 安全加固（P0，预计 1 天）

#### 1.1 安全中间件

**改动文件：** `internal/httpapi/server.go`

**具体步骤：**
1. 在 `Handler()` 方法中，在 CORS 处理之后、`s.mux.ServeHTTP` 之前，注入安全头中间件
2. 添加以下响应头：
   ```
   X-Content-Type-Options: nosniff
   X-Frame-Options: SAMEORIGIN
   Referrer-Policy: strict-origin-when-cross-origin
   ```
3. CSP 暂不启用（当前内联脚本较多，需要先审计），留作后续

**改动量：** ~15 行

#### 1.2 路径遍历防护

**改动文件：** `internal/httpapi/server.go`（`handleBrowseDir`、`serveDownloadFile`）

**具体步骤：**
1. `handleBrowseDir`：限制浏览路径必须在 `outputDir` 子目录内，拒绝 `..` 和绝对路径逃逸
2. `serveDownloadFile`：校验最终文件路径在下载目录内，`filepath.Clean` 后做前缀检查
3. Settings 保存：验证 `OutputDir` 不为系统敏感路径（`/etc`、`C:\Windows` 等），`MaxConcurrent` 限制 1-10 范围

**改动量：** ~40 行

#### 1.3 URL 协议校验

**改动文件：** `internal/httpapi/server.go`（`handleVideoInfo`、`handleDownloads`）

**具体步骤：**
1. 在解析 URL 前检查 scheme 必须是 `http` 或 `https`
2. 拒绝 `file://`、`ftp://`、`data:` 等协议

**改动量：** ~10 行

#### 1.4 CORS 增强

**改动文件：** `internal/httpapi/server.go`

**具体步骤：**
1. 现有 `YTGO_CORS_ORIGIN` 环境变量已支持，但缺少 `Access-Control-Allow-Credentials` 头
2. 添加 `Vary: Origin` 头避免缓存问题
3. 在 AGENTS.md 中补充 CORS 配置文档

**改动量：** ~10 行

**验证方式：**
- `go vet ./...` 通过
- 手动测试：用 curl 发送 `file:///etc/passwd` URL，确认被拒绝
- 手动测试：目录浏览试图逃逸下载目录，确认被拒绝

---

### Phase 2 — Web 认证（P0，预计 0.5 天）

#### 2.1 Token 认证

**改动文件：** `internal/httpapi/server.go`、`main_web.go`

**具体步骤：**
1. 读取环境变量 `YTGO_AUTH_TOKEN`，非空时启用 token 认证
2. 认证方式：请求头 `Authorization: Bearer <token>` 或查询参数 `?token=<token>`
3. 白名单路径无需认证：`/api/health`、`/api/config`、`/api/events`（SSE 连接后可选）
4. 认证失败返回 `401 Unauthorized`
5. 在 `GET /api/config` 响应中增加 `authRequired: true` 字段，让前端知道需要登录

**改动量：** ~50 行 Go + ~30 行前端

#### 2.2 前端登录页

**改动文件：** 新建 `frontend/src/components/LoginPage.tsx`、`frontend/src/lib/backend.ts`

**具体步骤：**
1. `fetchWebConfig` 检查 `authRequired`，为 true 时显示 LoginPage
2. LoginPage：简单表单，输入 token，存入 `sessionStorage`（非 localStorage，关闭标签即清除）
3. `apiFetch` 自动附加 `Authorization` 头
4. 401 响应时清除 token 并跳回登录页
5. Desktop 模式完全不加载 LoginPage（`fetchWebConfig` 返回 null）

**验证方式：**
- 启动时设置 `YTGO_AUTH_TOKEN=test123`，确认无 token 时返回 401
- 前端输入 token 后正常访问 API
- Desktop 模式不受影响

---

### Phase 3 — 前端架构改进（P1，预计 2 天）

#### 3.1 Vite Alias 双模式切换

**改动文件：** `frontend/vite.config.ts`、`frontend/src/lib/backend.ts`、新建 `frontend/src/lib/backend_web.ts`

**具体步骤：**

1. **新建 `backend_web.ts`：** 从 `backend.ts` 提取所有 web 分支逻辑，导出与 desktop 完全相同的函数签名
   ```ts
   // backend_web.ts — 所有函数纯 HTTP 调用，无 Wails 依赖
   import { apiFetch } from './api_client'
   export function CheckYtDlp() { return apiFetch('/api/ytdlp/status') }
   export function GetVideoInfo(url: string) { return apiFetch('/api/video/info', ...) }
   // ... 其余 ~25 个函数
   ```

2. **新建 `api_client.ts`：** 提取 `apiFetch`、`apiURL`、`WebConfig` 等 web 模式公共逻辑

3. **`backend.ts` 简化为 desktop-only：** 只保留 Wails 调用，移除所有 `getDesktop()` 分支
   ```ts
   import * as DesktopApp from '../../wailsjs/go/desktop/App'
   export function CheckYtDlp() { return DesktopApp.CheckYtDlp() }
   // ...
   ```

4. **`vite.config.ts` 添加 alias：**
   ```ts
   const isWeb = process.env.VITE_WEB === 'true' || process.env.NODE_ENV === 'development'
   resolve: {
       alias: {
           ...(isWeb ? { './backend': './backend_web' } : {}),
       }
   }
   ```
   需要确认 alias 路径映射正确，可能需要用 `@` 别名

5. **更新所有 import：** `App.tsx`、`SettingsDialog.tsx` 等文件的 `import { ... } from '../lib/backend'` 保持不变（alias 自动替换）

6. **`runtime.ts` 同步处理：** web 模式的 SSE 逻辑也提取到 `runtime_web.ts`，alias 替换

**改动量：** ~300 行重构（净增 ~0，主要是拆分）

**验证方式：**
- `npm run build` 通过（web 模式）
- `wails build` 通过（desktop 模式）
- 两种模式功能正常

#### 3.2 状态管理抽离（Zustand）

**改动文件：** 新建 `frontend/src/stores/` 目录，重构 `App.tsx`

**具体步骤：**

1. **安装依赖：** `npm install zustand`（~1KB gzipped，比 Redux/Pinia 轻量得多）

2. **新建 stores：**
   - `stores/settings.ts` — 设置状态 + 持久化逻辑
   - `stores/downloads.ts` — 下载队列 + SSE 事件监听
   - `stores/video.ts` — 视频信息、格式、播放列表状态
   - `stores/ui.ts` — 主题、语言、对话框开关、控制台日志

3. **迁移 App.tsx 中的 useState：** 将 ~30 个 `useState` 按职责分配到对应 store

4. **组件直接从 store 读取：** 消除 props drilling，SettingsDialog 直接读 `useSettingsStore()`

**改动量：** ~400 行（新增 ~150 行 store，App.tsx 减少 ~250 行）

**依赖：** 3.1 完成后进行（backend 拆分后 store 更清晰）

**验证方式：**
- `npm run build` 通过
- 手动测试：设置修改、下载队列、主题切换等功能正常

---

### Phase 4 — 收尾完善（P2，预计 1 天）

#### 4.1 ErrorBoundary i18n 补全

**改动文件：** `frontend/src/i18n/zh-CN.ts`、`frontend/src/i18n/en-US.ts`、`frontend/src/components/ErrorBoundary.tsx`

**具体步骤：**
1. 添加 3 个 i18n key：
   ```ts
   'errorBoundary.title': '应用出错了' / 'Something went wrong'
   'errorBoundary.retry': '重试' / 'Retry'
   'errorBoundary.reload': '重新加载' / 'Reload'
   ```
2. ErrorBoundary 中使用 `t()` 替代硬编码

**改动量：** ~10 行

#### 4.2 窗口状态持久化

**改动文件：** `internal/core/settings.go`、`internal/core/types.go`、`desktop/app.go`

**具体步骤：**
1. Settings 结构体增加 `WindowWidth`、`WindowHeight`、`WindowMaximized` 字段
2. `desktop/app.go` 的 `startup` 中读取并恢复窗口大小
3. 使用 Wails 的 `runtime.WindowSetSize` 和 `runtime.WindowSetMaximised`

**改动量：** ~30 行

#### 4.3 Dockerfile 优化

**改动文件：** `Dockerfile`

**具体步骤：**
1. 添加 `ARG APP_VERSION=0.0.0`
2. 后端构建时传入 `-ldflags "-X main.version=${APP_VERSION}"`
3. 指定 `--platform=linux/amd64`（当前已有，确认一致性）

**改动量：** ~5 行

#### 4.4 代码清理

**改动文件：** 多个

**具体步骤：**
1. 删除 `App.tsx` 中未使用的 `QUALITY_OPTIONS`
2. `formatDuration` 提取到 `lib/formatUtils.ts`（已有该文件，确认是否已提取）
3. 清理废弃 i18n key（`quickSettings.*`、`playlist.detected` 等）
4. 删除未使用的 shadcn/ui 组件（需先确认哪些确实未使用）

**改动量：** ~50 行删除

---

### 实施顺序与依赖

```
Phase 1 (安全加固) ──┐
                     ├── 可并行，无依赖
Phase 2 (认证系统) ──┘
         │
         ▼
Phase 3.1 (Vite alias) ──┐
                          ├── 3.2 依赖 3.1
Phase 3.2 (Zustand)   ───┘
         │
         ▼
Phase 4 (收尾) ── 无依赖，可随时穿插
```

**总预计工时：** ~4.5 天

### 不实施的项目

| 项目 | 原因 |
|------|------|
| WebSocket 事件 | 当前 SSE 满足需求，双向通信无场景 |
| Vite 自动导入 | React 手动 import 清晰可控，无需引入 |
| Pinia（改用 Zustand） | React 生态用 Zustand 更合适 |

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
