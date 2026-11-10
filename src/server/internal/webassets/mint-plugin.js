/* @name MusicDL
 * @description 通过 go-music-dl-ex 聚合接口搜索、播放与获取歌词
 * @version 1.0.1
 * @author go-music-dl-ex
 * @homepage https://github.com/ifeng369/go-music-dl-ex
 *
 * Mint-Music / 洛雪 LX 自定义音源插件。
 * 请从 fnOS 应用的设置页下载本插件，下载时会自动写入应用地址和当前账号 Token。
 */

const { EVENT_NAMES, request, on, send } = globalThis.lx

const DEFAULT_BASE = "http://127.0.0.1:8080";
const DEFAULT_TOKEN = "";
const SOURCE = "musicdl"
const QUALITIES = ["128k", "320k", "flac"]

// 搜索结果索引。部分宿主会截断/重建 songmid，导致播放阶段拿不到完整信息，
// 这里按多种键缓存，播放时逐个回退查找。
const infoCache = new Map()
const CACHE_LIMIT = 800

function getBase() {
  return String(DEFAULT_BASE).replace(/\/+$/, "")
}

function buildQuery(params) {
  return Object.keys(params || {})
    .filter((key) => params[key] !== undefined && params[key] !== null && params[key] !== "")
    .map((key) => encodeURIComponent(key) + "=" + encodeURIComponent(params[key]))
    .join("&")
}

function httpGet(path, params) {
  const query = buildQuery(params)
  const url = getBase() + path + (query ? "?" + query : "")
  return new Promise((resolve, reject) => {
    request(url, { method: "GET", headers: { Accept: "application/json", ...(DEFAULT_TOKEN ? { Authorization: "Bearer " + DEFAULT_TOKEN } : {}) } }, (err, resp) => {
      if (err) return reject(err)
      const status = resp && resp.statusCode
      let body = resp && resp.body
      if (typeof body === "string") {
        try {
          body = JSON.parse(body)
        } catch (_) {
          // keep plain text
        }
      }
      if (status && status >= 400) {
        return reject(new Error("HTTP " + status))
      }
      resolve(body)
    })
  })
}

function formatDuration(seconds) {
  const total = Math.max(0, Math.round(Number(seconds) || 0))
  return String(Math.floor(total / 60)).padStart(2, "0") + ":" + String(total % 60).padStart(2, "0")
}

function cacheKey(...parts) {
  return parts
    .map((part) => String(part === undefined || part === null ? "" : part))
    .join("\u0001")
}

function rememberInfo(payload, extraKeys) {
  const keys = [cacheKey("songmid", payload.songmid)]
  for (const key of extraKeys || []) {
    if (key) keys.push(key)
  }
  for (const key of keys) infoCache.set(key, payload)
  if (infoCache.size > CACHE_LIMIT) {
    const overflow = infoCache.size - CACHE_LIMIT
    let index = 0
    for (const key of infoCache.keys()) {
      if (index++ >= overflow) break
      infoCache.delete(key)
    }
  }
}

function normalizeItem(item) {
  return {
    source: item.source,
    id: item.sourceId || item.id,
    extra: item.extra || {},
    title: item.title || item.name || "",
    artist: item.artist || "",
    album: item.album || "",
    duration: item.duration || 0,
    ext: item.ext || "",
    artwork: item.artwork || item.cover || "",
  }
}

function encodeInfo(item) {
  return encodeURIComponent(JSON.stringify(normalizeItem(item)))
}

function parseSongmid(value) {
  if (!value || typeof value !== "string") return null
  const candidates = [value]
  try {
    const decoded = decodeURIComponent(value)
    if (decoded !== value) candidates.push(decoded)
  } catch (_) {
    // ignore malformed escapes
  }
  for (const raw of candidates) {
    try {
      const parsed = JSON.parse(raw)
      if (parsed && typeof parsed === "object" && parsed.id) return parsed
    } catch (_) {
      // try next
    }
  }
  return null
}

function lookupCachedInfo(musicInfo) {
  if (!musicInfo) return null
  if (typeof musicInfo === "string") {
    return infoCache.get(cacheKey("songmid", musicInfo)) || null
  }
  const keys = []
  if (musicInfo.songmid) keys.push(cacheKey("songmid", musicInfo.songmid))
  if (musicInfo.songId) keys.push(cacheKey("id", musicInfo.songId), cacheKey("hash", musicInfo.songId))
  if (musicInfo.hash) keys.push(cacheKey("hash", musicInfo.hash), cacheKey("id", musicInfo.hash))
  if (musicInfo.songmid && musicInfo.songId) keys.push(cacheKey("songmid", musicInfo.songId))
  for (const key of keys) {
    const hit = infoCache.get(key)
    if (hit) return hit
  }
  return null
}

function resolveInfo(musicInfo) {
  if (typeof musicInfo === "string") {
    const trimmed = musicInfo.trim()
    if (trimmed.startsWith("%7B") || trimmed.startsWith("{")) {
      const parsed = parseSongmid(trimmed)
      if (parsed) return parsed
      if (trimmed.startsWith("{")) {
        try {
          const obj = JSON.parse(trimmed)
          if (obj && typeof obj === "object") return resolveInfo(obj)
        } catch (_) {
          // fall through to cache lookup
        }
      }
    }
    return lookupCachedInfo(trimmed)
  }
  const fromSongmid = parseSongmid(musicInfo && musicInfo.songmid)
  if (fromSongmid) return fromSongmid
  return lookupCachedInfo(musicInfo)
}

function toMusicItem(item) {
  const payload = normalizeItem(item)
  const encoded = encodeURIComponent(JSON.stringify(payload))
  const id = String(item.sourceId || item.id || "")
  rememberInfo(
    { ...payload, songmid: encoded },
    [cacheKey("songmid", encoded), cacheKey("id", id), cacheKey("hash", id), cacheKey("songmid", id)],
  )
  return {
    songmid: encoded,
    songId: id,
    hash: id,
    singer: payload.artist,
    name: payload.title,
    albumName: payload.album,
    albumId: "",
    albumMid: "",
    strMediaMid: "",
    copyrightId: "",
    source: SOURCE,
    interval: formatDuration(payload.duration),
    mrcUrl: "",
    trcUrl: "",
    types: QUALITIES,
    typeMap: {},
    typeUrl: {},
  }
}

function search(source, keyword, page, limit) {
  const pageSize = limit || 30
  return httpGet("/api/search", {
    q: keyword,
    type: "music",
    page: page || 1,
    limit: pageSize,
  }).then((data) => {
    const items = (data && data.items) || []
    const isEnd = Boolean(data && data.isEnd) || items.length < pageSize
    return {
      list: items.map(toMusicItem),
      total: Number(data && data.total) || items.length,
      isEnd,
      allPage: isEnd ? page || 1 : (page || 1) + 1,
    }
  })
}

function streamParams(info) {
  const params = {
    source: info.source,
    id: info.id,
    title: info.title,
    artist: info.artist,
    album: info.album,
    duration: info.duration,
    ext: info.ext,
  }
  if (info.extra && Object.keys(info.extra).length) {
    params.extra = JSON.stringify(info.extra)
  }
  return params
}

function musicUrl(source, musicInfo, quality) {
  const info = resolveInfo(musicInfo)
  if (!info || !info.source || !info.id) {
    return Promise.reject(new Error("无法解析歌曲信息，请重新搜索后再试"))
  }
  const accessToken = DEFAULT_TOKEN ? "&access_token=" + encodeURIComponent(DEFAULT_TOKEN) : ""
  return Promise.resolve(getBase() + "/api/stream?" + buildQuery(streamParams(info)) + accessToken)
}

function lyric(source, musicInfo) {
  const info = resolveInfo(musicInfo)
  if (!info) return Promise.resolve({ lyric: "", tlyric: "" })
  const params = {
    source: info.source,
    id: info.id,
    title: info.title,
    artist: info.artist,
    album: info.album,
    duration: info.duration,
  }
  if (info.extra && Object.keys(info.extra).length) {
    params.extra = JSON.stringify(info.extra)
  }
  return httpGet("/api/lyric", params)
    .then((data) => ({
      lyric: (data && (data.rawLrc || data.lrc)) || "",
      tlyric: "",
    }))
    .catch(() => ({ lyric: "", tlyric: "" }))
}

function pic(source, musicInfo) {
  const info = resolveInfo(musicInfo)
  return Promise.resolve((info && info.artwork) || "")
}

const sources = {
  [SOURCE]: {
    name: "MusicDL",
    type: "music",
    qualitys: QUALITIES,
    qualities: QUALITIES,
    actions: ["musicSearch", "musicUrl", "lyric", "pic"],
  },
}

send(EVENT_NAMES.inited, { sources })

on(EVENT_NAMES.request, ({ source, action, info }) => {
  switch (String(action).toLowerCase()) {
    case "musicsearch":
      return search(source, info.keyword, info.page, info.limit || info.pagesize)
    case "musicurl":
      return musicUrl(source, info.musicInfo, info.type)
    case "lyric":
    case "musiclyric":
      return lyric(source, info.musicInfo)
    case "pic":
    case "musicpic":
      return pic(source, info.musicInfo)
    default:
      throw new Error("Unsupported action: " + action)
  }
})
