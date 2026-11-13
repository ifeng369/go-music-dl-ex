# 双向同步设计（独立版 ⇄ 服务端）

目标：独立版（手机内置核心）与服务器保持同一份用户数据，任一侧修改都能同步到另一侧。

## 一、同步范围

| 数据 | 方向 | 冲突策略 |
|---|---|---|
| 平台 Cookie | 双向 | 按 source 粒度，后写覆盖 |
| 收藏歌曲 | 双向 | 按 song id 合并，删除用墓碑 |
| 自建歌单 | 双向 | 按 playlist id + 字段时间戳 |
| 收藏歌单 | 双向 | 按 sheet key 合并，删除用墓碑 |
| 播放历史 | 双向 | 按时间合并，保留最近 300 |
| 播放队列 | 双向 | 整份覆盖，取较新 |
| 换源记录 | 双向 | 按 song id 合并，取较新 |
| 生成歌单（每日/喜欢/随便/榜单） | 服务端 → 本地 | 服务端权威，本地只读 |
| 音源解析结果 / 音频字节 | 不同步 | 本机实时获取 |

## 二、数据模型

每条记录带 `updatedAt`（毫秒）与可选 `deletedAt`（墓碑）：

```json
{
  "favorites": { "netease:123": { "song": {...}, "updatedAt": 1712..., "deletedAt": 0 } },
  "playlists": { "<id>": { "name": "...", "songs": [...], "updatedAt": 1712..., "deletedAt": 0 } },
  "overrides": { "netease:123": { "song": {...}, "updatedAt": 1712... } },
  "cookies":   { "netease": { "value": "MUSIC_U=...", "updatedAt": 1712... } }
}
```

墓碑保留 90 天，之后可物理清理。

## 三、接口

```
GET  /api/sync?since=<ms>     拉取自 since 之后变更（含墓碑）
POST /api/sync                推送本地变更，服务端合并后返回合并结果
```

请求体：

```json
{ "since": 1712345678000, "changes": { "favorites": {...}, "cookies": {...} } }
```

响应：

```json
{ "now": 1712345999000, "merged": { ... }, "serverChanges": { ... } }
```

合并规则（服务端实现，两侧一致）：

1. 同 key 比较 `updatedAt`，大的胜出
2. `deletedAt > updatedAt` 视为删除
3. 集合类（收藏、歌单内歌曲）按成员取并集，删除靠墓碑

## 四、触发时机

| 时机 | 行为 |
|---|---|
| App 启动 | 拉一次（`since` 取本地记录） |
| 本地改动 | 500ms 防抖后推送该条 |
| 手动「立即同步」 | 全量拉 + 推送 |
| 每隔 10 分钟（前台） | 增量拉 |
| 服务端生成歌单完成 | 本地下次拉取时带上 |

## 五、实现拆分

### 服务端
1. `sync_store.go`：`app_sync_items(user_id, kind, key, payload, updated_at, deleted_at)` 表
2. `sync.go`：`GET/POST /api/sync`，实现上面的合并规则
3. 现有 `library` / `cookies` 写接口改为写入 sync 表（保持兼容旧接口）

### 独立版
1. 本地核心同样有 `app_sync_items` 表
2. `SettingsView` 增加「服务器地址 / 账号 / 密码 / 立即同步 / 自动同步」配置
3. `NativeBridge.syncNow(url, user, pass)`：登录 → 双向合并 → 写回本地
4. 本地每次变更打 `updatedAt`，同步时只上传增量

## 六、安全

- 同步接口要求登录，只操作调用者自己的数据
- Cookie 明文只在传输与内存中出现，服务端与本地都只存自己那份
- 建议 HTTPS（Cloudflare Tunnel 已满足）
- 可选：为独立版生成专用同步 Token，避免保存账号密码

## 七、阶段

| 阶段 | 内容 |
|---|---|
| S1 | 服务端 sync 表 + 合并接口 |
| S2 | 独立版本地表 + 增量收集 |
| S3 | 桥接方法 + 设置页表单 |
| S4 | 自动同步与冲突可视化（可选） |

## 八、已实现方案

- 数据仍以 `app_user_libraries` 整份 JSON 为准，`app_sync_versions` 只保存每条记录的 `updatedAt` / `deletedAt`
- `PUT /api/me/library` 与 Cookie 写接口会在保存时对比前后差异，自动为变化的记录打时间戳
- `GET /api/sync?since=<ms>` 拉取变更（含墓碑），`POST /api/sync` 按 last-write-wins 合并并返回完整状态
- 独立版设置页「与服务器同步」由 `MusicDLNative.syncNow()` 完成：拉本地 → 推服务器 → 拉服务器 → 写回本地
- 生成歌单 `recs` 不参与上传，服务器下发的副本始终覆盖本地
- 同步接口不进入 Cookie 作用域中间件，避免与服务端 Cookie 缓存失效逻辑互相等待

## 九、使用方式

独立版 APK → 设置 → 与服务器同步：填写服务器地址、账号、密码后点「立即同步」。
同步完成后页面会自动刷新，收藏、歌单、队列、历史、换源记录与 Cookie 全部生效。

首次成功后账号密码保存在应用私有目录（SharedPreferences），之后每次启动都会在页面加载前
自动双向同步，无需再输入；设置页可点「清除保存的同步账号」移除。
