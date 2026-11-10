# go-music-dl-ex

在 [guohuiyuan/go-music-dl](https://github.com/guohuiyuan/go-music-dl) 的聚合音乐能力之上，增加：

- **Web 播放器**：Vue 3 + Vite，暗色主题，搜索、按音源分组的推荐、分类歌单、排行榜、歌单/专辑、歌手、歌词、播放队列、全屏当前播放。
- **多用户**：管理员可新增/删除用户；收藏、自建歌单、收藏歌单、播放记录、本地音乐按账号隔离并保存在服务端。
- **下载目录**：管理员可在设置中配置全局下载目录，下载文件与本地音乐位于 `<下载目录>/<用户名>/`。
- **MusicFree 插件**：单文件 `src/plugin/plugin.js`，把本项目的 JSON API 适配成 MusicFree 插件协议。
- **飞牛 fnOS 应用**：GitHub Actions 在推送 `v*` 标签时自动编译 `.fpk`，一个服务端口同时提供 Web 和 MusicFree API。

> 本项目只提供聚合与播放能力，不存储、不分发音乐文件。请遵守各音乐平台的服务条款和当地法律，仅将 Cookie/账号用于你拥有权限的内容。

## 目录结构

```
source/                 # 上游 go-music-dl 的浅克隆（保持不改动，便于升级）
src/
  server/               # Go API + 内嵌 Web 静态资源
    main.go
    internal/api/       # JSON API（MusicFree 友好）
    internal/webassets/ # go:embed 的 Vite 构建产物
  web/                  # Vue 3 Web 客户端
  plugin/plugin.js      # MusicFree 插件（无需构建）
packaging/fnos/         # fnOS 原生应用模板与打包脚本
.github/workflows/      # tag 触发的 Release 工作流
scripts/update-source.sh
```

`source/` 是上游仓库的独立 git 克隆。根仓库的 `.gitignore` 忽略它（嵌套 `.git` 无法作为普通目录提交）；CI 在 `source/` 缺失时会自动浅克隆上游。这样本地可以随时 `git -C source pull` 无缝升级，发布构建又保持可复现。

## 环境要求

- Go **1.25.1+**
- Node.js **20+**（推荐 22）
- 仅打包 fnOS 时需要 Linux + [fnpack](https://static2.fnnas.com/fnpack/)（CI 已处理）

## 本地开发

一次性拉取/更新上游：

```bash
bash scripts/update-source.sh          # 默认 main
bash scripts/update-source.sh v1.2.3   # 或指定 tag/分支/commit
```

启动 API（默认 `:8080`，数据目录 `src/server/data`）：

```bash
cd src/server
go run . -addr :8080 -data ./data
```

另开终端启动 Web 开发服务器（`/api` 自动代理到 `:8080`）：

```bash
cd src/web
npm install
npm run dev
```

访问 http://127.0.0.1:5173 。

### 生产构建

Vite 直接把产物写入 `src/server/internal/webassets/dist`，Go 再用 `go:embed` 打进二进制：

```bash
cd src/web && npm run build
cd ../server && go build -trimpath -o musicdl-server .
./musicdl-server -addr :8080 -data ./data
```

访问 http://127.0.0.1:8080 ，同一个端口同时提供：

- Web UI：`/`
- MusicFree API：`/api/*`
- 健康检查：`/api/healthz`

启动参数：

| 参数 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-addr` | `MUSIC_DL_ADDR` | `:8080` | 监听地址 |
| `-data` | `MUSIC_DL_DATA_DIR` | `data` | 持久化目录（设置、Cookie、下载） |
| `-web` | - | 空 | 用外部目录覆盖内嵌 Web 资源（调试用） |
| `-version` | - | - | 打印版本 |

## MusicFree 插件

1. 打开 MusicFree → 插件管理 → 从本地安装，选择 `src/plugin/plugin.js`（也可以直接放到插件目录）。
2. 安装后进入插件设置，把 **API 地址** 改成你的服务地址：
   - 本机：`http://127.0.0.1:8080`
   - 局域网/飞牛：`http://<NAS-IP>:18080`
3. 支持搜索音乐/歌单/专辑/歌手、歌单与专辑详情、排行榜、推荐分类、歌词、播放地址、链接导入。

插件通过 `env.getUserVariables()` 读取 `apiBase`，默认 `http://127.0.0.1:8080`。插件只使用 MusicFree 注入的 HTTP 客户端（axios / fetch / needle），无 npm 依赖、无需构建。

插件目录参考：

- Android：`Android/data/fun.upup.musicfree/files/plugins`
- Windows：`%APPDATA%/MusicFree/musicfree-plugins`

## API 一览

所有接口返回 JSON（`/api/stream` 与 `/api/cover` 返回二进制），允许跨域。`source` 取值见 `GET /api/sources`（netease、qq、kugou、kuwo、migu、bilibili、fivesing、jamendo、joox、qianqian、soda、apple、spotify 等）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/healthz`、`/api/version` | 健康检查、版本 |
| GET | `/api/sources` | 平台列表与能力（搜索/歌单/专辑/排行榜/登录等） |
| GET | `/api/detect?url=` | 识别链接来源 |
| GET | `/api/search?q=&type=music\|sheet\|album\|artist&source=&limit=&page=` | 聚合搜索 |
| GET | `/api/artist/works?source=&name=&limit=` | 歌手作品 |
| GET | `/api/song?source=&id=&extra=` | 歌曲详情（`resolve=1` 解析直链） |
| GET | `/api/song/url?source=&id=&extra=` | 解析播放直链 + 代理地址 |
| GET | `/api/stream?source=&id=&extra=&title=&artist=&album=&duration=&ext=` | 音频代理/流（支持 Range） |
| GET | `/api/cover?url=` | 封面代理（绕过 Referer/UA 限制） |
| GET | `/api/lyric?source=&id=&extra=` | 歌词 |
| GET | `/api/playlist?source=&id=` | 歌单详情 |
| GET | `/api/album?source=&id=` | 专辑详情 |
| GET | `/api/toplists?source=` | 排行榜分组 |
| GET | `/api/toplist/detail?source=&id=` | 排行榜歌曲 |
| GET | `/api/recommend?source=` | 推荐歌单 |
| GET | `/api/categories?source=` | 歌单分类 |
| GET | `/api/category/sheets?source=&category=&page=&limit=` | 分类歌单 |
| GET | `/api/user/playlists?source=&page=&limit=` | 用户歌单（需 Cookie） |
| GET | `/api/home?source=` | Web 首页聚合数据 |
| GET/POST | `/api/import` | 通过链接导入歌曲/歌单/专辑（`url`、`type`） |
| GET/POST/DELETE | `/api/cookies` | 查看（脱敏）/保存/清除各平台 Cookie |

MusicFree 插件使用的核心形状：

```jsonc
// 搜索 / 详情
{ "items": [ /* 歌曲、歌单、专辑或歌手 */ ], "page": 1, "isEnd": true, "errors": {} }
// 歌单 / 专辑 / 排行榜详情
{ "isEnd": true, "page": 1, "total": 100, "musicList": [ /* 歌曲 */ ], "sheetItem": { /* 歌单 */ } }
// 排行榜 / 推荐 / 分类
{ "groups": [], "items": [], "errors": {} }
```

歌曲对象中包含 `id`（`source:原始ID`）、`source`、`title`、`artist`、`album`、`duration`、`artwork`、`url`、`ext`、`extra` 等字段，插件会再映射成 MusicFree 的 `IMusicItem`。

## 飞牛 fnOS 应用

### 安装

从 GitHub Release 下载对应架构的包：

- x86_64：`musicdl-ex-<version>-fnos-amd64.fpk`
- ARM64：`musicdl-ex-<version>-fnos-arm64.fpk`

在 fnOS 应用中心手动安装。安装后：

- 桌面入口打开内嵌 Web 播放器；
- 服务监听 **18080**，同一端口暴露 `/api/*` 给 MusicFree；
- 数据保存在 fnOS 包数据目录（`${TRIM_PKGVAR}`），升级不丢失；
- 卸载时向导可选择是否删除数据。

> 端口转发后，同一局域网内可直接把 MusicFree 插件的 API 地址设为 `http://<NAS-IP>:18080`。该 API 默认无鉴权，请勿直接暴露到公网。

### 打包原理

`packaging/fnos/` 是标准的 fnOS 原生应用模板：

```
manifest              # appname/version/platform/service_port 等
ICON.PNG / ICON_256.PNG
app/
  musicdl-server      # 构建时复制进来的 Linux 二进制（内嵌 Web）
  musicdl-ex.sc       # 端口转发声明
  ui/config           # 桌面入口（iframe → http://NAS:18080/）
  ui/images/*.png
cmd/                  # start/stop/status 与安装/升级/卸载生命周期脚本
config/privilege      # 使用专用包用户 musicdlex
config/resource       # 端口配置
wizard/uninstall      # 卸载时是否删除数据
```

`packaging/fnos/build.sh` 会复制模板、写入版本/平台、`chmod 755`、调用官方 `fnpack` 生成 FPK。`fnpack` 固定为 **1.2.3** 并校验 SHA-256；仅支持在 Linux amd64 上运行（CI 使用 `ubuntu-24.04`）。

## 发布

推送 `v*` 标签即可触发 `.github/workflows/release.yml`：

```bash
git tag v1.0.0
git push origin v1.0.0
```

工作流会：

1. 如 `source/` 不存在则浅克隆上游；
2. `npm ci && npm run build` 构建 Web；
3. `go test ./...`；
4. 交叉编译 `linux/amd64`、`linux/arm64` 二进制（版本号通过 ldflags 注入）；
5. 用 `fnpack` 生成 x86 与 ARM 两个 FPK，并生成 SHA-256；
6. 创建 GitHub Release，上传两个 `.fpk`、校验文件和 `plugin.js`。

## 升级上游

```bash
bash scripts/update-source.sh            # 跟随 main
bash scripts/update-source.sh <commit>   # 固定到某个提交
cd src/server && go mod tidy && go build .
```

升级后建议重新构建 Web（`npm run build`）再编译，确保内嵌资源是最新的。

## 常见问题

- **播放无声/403**：部分平台需要 Cookie。在 Web 设置页或 `POST /api/cookies` 配置对应平台 Cookie。播放优先走 `/api/stream` 代理，已处理 Referer/UA/Range。
- **搜索为空**：确认 `source` 拼写正确，并查看响应里的 `errors`（不同平台可用性不同）。
- **fnOS 安装后打不开**：确认 18080 未被占用，查看包数据目录下 `musicdl-ex.log`；`cmd/main status` 应返回运行中。
- **MusicFree 提示网络错误**：插件 API 地址必须能被手机访问；不要填 `127.0.0.1`（除非 MusicFree 与服务同机）。

## 致谢

- [guohuiyuan/go-music-dl](https://github.com/guohuiyuan/go-music-dl) 与 `music-lib`
- [maotoumao/MusicFree](https://github.com/maotoumao/MusicFree) 插件协议
- [algerkong/AlgerMusicPlayer](https://github.com/algerkong/AlgerMusicPlayer) Web UI 参考
