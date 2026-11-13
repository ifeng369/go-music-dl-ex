# 安卓独立客户端（单用户，无服务端依赖）

目标：APK 内置完整能力，脱离 NAS 也能搜索、播放、管理歌单。
服务端只保留"可选同步"，不再是被依赖项。

## 一、总体结构

```
APK
├─ Android 壳（MainActivity / 播放服务 / 通知）
├─ Go 核心（gomobile bind 生成的 .aar）
│   ├─ source/          现有上游聚合核心（14 个平台）
│   ├─ api/             现有 HTTP 接口（本地监听 127.0.0.1:随机端口）
│   ├─ webassets/       内嵌的 Vue 前端产物
│   └─ store/           本地 SQLite（收藏/歌单/队列/换源记录）
└─ WebView 加载 http://127.0.0.1:<port>/
```

进程内起一个 HTTP 服务，前端仍用现有 Vue 页面，改动最小。

## 二、与现有代码的差异

| 模块 | 现状 | 独立版 |
|---|---|---|
| 鉴权 | 多用户 + Session | **单用户**，启动即注入本地用户，`requireUser` 直接放行 |
| Cookie | 每用户存库 | 存本机 SQLite（仅一份） |
| 用户数据 | `/api/me/*` 走服务端 | 同一套接口，落本地 SQLite |
| 媒体 | 服务端代理 / App 直连 | 本地核心直接解析，App 内直连（已实现 LocalProxy，可复用） |
| 生成歌单 | 服务端定时生成 | 本地生成（首次启动或每天 6:00） |
| 同步 | 无 | **可选**：配置服务器地址后，手动/定时同步用户数据 |
| 插件/共享 | - | 不需要 |

## 三、需要改造的点

### 1. 单用户模式（Go）
- `api.Options` 增加 `SingleUser *AppUser`
- `requireUser` 在 SingleUser 下直接返回该用户，不校验 Session
- 启动时确保库内存在该用户，并跳过登录页

### 2. 移动端入口（Go）
新增 `mobile/` 包：

```go
func Start(dataDir string) (string, error)
func Stop()
func Version() string
```

- 用 `gomobile bind -target=android` 生成 `mobile.aar`
- `Start` 内：打开 SQLite → 构造 api.Server → 监听 `127.0.0.1:0` → 返回 base URL
- NDK 不需要，`gomobile bind` 用 JDK + Android SDK 即可

### 3. Android 壳改造
- 移除「首次填写服务器地址」页面，直接启动本地服务
- `LocalProxy` 保留（用于媒体直连），但 base 改为 `http://127.0.0.1:<corePort>`
- 设置页「服务器地址」改为「可选同步服务器」
- 包体：Go 核心约 +20~30MB（arm64 + armv7）

### 4. CI
新增 job：
- `gomobile init`
- `gomobile bind -target=android/arm64,android/arm -o app/libs/mobile.aar ./mobile`
- `./gradlew assembleRelease`
- 与现有 FPK 流程互不影响

### 5. 数据迁移
首次启动时可从「已登录的服务器」拉一次 `/api/me/library`、歌单、换源记录，导入本地库。

## 四、分阶段

| 阶段 | 内容 | 预估 |
|---|---|---|
| P0 | 单用户模式 + `mobile.Start` + 本地起服务 | 2 天 |
| P1 | gomobile 产物接入 APK，去掉填地址页 | 1 天 |
| P2 | 本地数据落 SQLite，设置页改版 | 1 天 |
| P3 | 音乐库导入、初次迁移 | 1 天 |
| P4 | 可选同步（服务器地址 + 手动/定时） | 2 天 |
| P5 | 包体优化（ABI 分包、strip） | 0.5 天 |

## 五、风险

1. **包体增大**：Go 核心 + TLS，双 ABI 约 +30MB；可用 ABI 分包降到单包 18~20MB
2. **音源修复要发版**：核心内置后，平台改签名必须发新 APK；保留「可选服务器解析」作为热修复通道
3. **首次启动初始化**：SQLite 建库 + 首屏生成歌单，需要加载提示
4. **后台稳定性**：前台服务 + 本地 HTTP 服务需保活，避免被系统回收导致播放中断
5. **兼容性**：minSdk 21 下 gomobile 产物的 ABI 覆盖要确认

## 六、分支策略

- 分支：`feat/android-standalone`
- main 上的网页版与 FPK 流程完全不动
- 独立客户端单独打 tag（如 `v1.1.0-standalone`），与现有 release 并行
