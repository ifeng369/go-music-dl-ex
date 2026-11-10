/* @name MusicDL
 * @description 通过 go-music-dl-ex 聚合接口搜索、播放与获取歌词
 * @version 1.0.0
 * @author go-music-dl-ex
 * @homepage https://github.com/ifeng369/go-music-dl-ex
 *
 * Mint-Music / 洛雪 LX 自定义音源插件。
 * 在 fnOS 应用中下载的版本会自动把 API 地址指向当前应用地址。
 */

const { EVENT_NAMES, request, on, send } = globalThis.lx

const DEFAULT_BASE = "http://127.0.0.1:8080";
const SOURCE = "musicdl"
const QUALITIES = ["128k", "320k", "flac"]

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
    request(url, { method: "GET", headers: { Accept: "application/json" } }, (err, resp) => {
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

function encodeInfo(item) {
  return encodeURIComponent(
    JSON.stringify({
      source: item.source,
      id: item.sourceId || item.id,
      extra: item.extra || {},
      title: item.title || item.name || "",
      artist: item.artist || "",
      album: item.album || "",
      duration: item.duration || 0,
      ext: item.ext || "",
      artwork: item.artwork || item.cover || "",
    }),
  )
}

function decodeInfo(musicInfo) {
  try {
    return JSON.parse(decodeURIComponent((musicInfo && musicInfo.songmid) || ""))
  } catch (_) {
    return null
  }
}

function toMusicItem(item) {
  return {
    songmid: encodeInfo(item),
    songId: String(item.sourceId || item.id || ""),
    hash: String(item.sourceId || item.id || ""),
    singer: item.artist || "",
    name: item.title || item.name || "",
    albumName: item.album || "",
    albumId: "",
    albumMid: "",
    strMediaMid: "",
    copyrightId: "",
    source: SOURCE,
    interval: formatDuration(item.duration),
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

function musicUrl(source, musicInfo, quality) {
  const info = decodeInfo(musicInfo)
  if (!info) return Promise.reject(new Error("无法解析歌曲信息"))
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
  return Promise.resolve(getBase() + "/api/stream?" + buildQuery(params))
}

function lyric(source, musicInfo) {
  const info = decodeInfo(musicInfo)
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
  const info = decodeInfo(musicInfo)
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
