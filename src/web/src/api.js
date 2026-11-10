const BASE = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')
const ABSOLUTE_BASE = /^https?:\/\//i.test(BASE)
const TOKEN_KEY = 'musicdl.token'

export function getAuthToken() {
  try {
    return localStorage.getItem(TOKEN_KEY) || ''
  } catch {
    return ''
  }
}

export function setAuthToken(token) {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token)
    else localStorage.removeItem(TOKEN_KEY)
  } catch {
    // Storage can be unavailable in private browsing; cookie auth still works.
  }
}

function buildUrl(path, params) {
  const url = new URL(BASE + path, window.location.origin)
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value === undefined || value === null || value === '') continue
      if (Array.isArray(value)) {
        for (const entry of value) if (entry) url.searchParams.append(key, entry)
      } else {
        url.searchParams.set(key, value)
      }
    }
  }
  return ABSOLUTE_BASE ? url.toString() : url.pathname + url.search
}

async function request(path, params, options = {}) {
  const token = getAuthToken()
  const headers = { Accept: 'application/json' }
  if (options.body) headers['Content-Type'] = 'application/json'
  if (token) headers.Authorization = `Bearer ${token}`
  const response = await fetch(buildUrl(path, params), {
    ...options,
    headers: { ...headers, ...(options.headers || {}) },
  })
  let data = null
  const text = await response.text()
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { error: text }
    }
  }
  if (!response.ok) {
    const message = (data && (data.error || data.message)) || `请求失败 (${response.status})`
    throw new Error(message)
  }
  return data
}

const get = (path, params) => request(path, params)
const post = (path, body) => request(path, null, { method: 'POST', body: JSON.stringify(body || {}) })
const put = (path, body) => request(path, null, { method: 'PUT', body: JSON.stringify(body || {}) })
const del = (path, params) => request(path, params, { method: 'DELETE' })

async function upload(path, file) {
  const token = getAuthToken()
  const form = new FormData()
  form.append('file', file)
  const headers = { Accept: 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  const response = await fetch(buildUrl(path), { method: 'POST', headers, body: form })
  const text = await response.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = { error: text }
  }
  if (!response.ok) throw new Error((data && (data.error || data.message)) || `上传失败 (${response.status})`)
  return data
}

export const api = {
  health: () => get('/api/healthz'),
  version: () => get('/api/version'),
  sources: () => get('/api/sources'),
  home: (params) => get('/api/home', params),
  search: (params) => get('/api/search', params),
  playlist: (params) => get('/api/playlist', params),
  album: (params) => get('/api/album', params),
  toplists: (params) => get('/api/toplists', params),
  toplistDetail: (params) => get('/api/toplist/detail', params),
  recommend: (params) => get('/api/recommend', params),
  categories: (params) => get('/api/categories', params),
  categorySheets: (params) => get('/api/category/sheets', params),
  artistWorks: (params) => get('/api/artist/works', params),
  songUrl: (params) => get('/api/song/url', params),
  alternatives: (params) => get('/api/song/alternatives', params),
  lyric: (params) => get('/api/lyric', params),
  importUrl: (body) => post('/api/import', body),
  cookies: () => get('/api/cookies'),
  userPlaylists: (params) => get('/api/user/playlists', params),
  saveCookie: (body) => post('/api/cookies', body),
  deleteCookie: (source) => del('/api/cookies', { source }),

  auth: {
    login: (username, password) => post('/api/auth/login', { username, password }),
    logout: () => post('/api/auth/logout', {}),
    me: () => get('/api/auth/me'),
  },
  users: {
    list: () => get('/api/users'),
    create: (body) => post('/api/users', body),
    remove: (id) => del(`/api/users/${id}`),
    updatePassword: (id, password) => put(`/api/users/${id}/password`, { password }),
  },
  library: {
    get: () => get('/api/me/library'),
    save: (data) => put('/api/me/library', data),
  },
  settings: {
    get: () => get('/api/settings'),
    save: (downloadDir) => put('/api/settings', { downloadDir }),
  },
  local: {
    list: () => get('/api/me/local'),
    upload: (file) => upload('/api/me/local/upload', file),
    remove: (name) => del('/api/me/local', { name }),
    streamUrl: (name) => buildUrl('/api/me/local/stream', { name }),
  },
  download: (body) => post('/api/me/download', body),
  downloads: {
    list: (params) => get('/api/me/downloads', params),
    remove: (id) => del(`/api/me/downloads/${id}`),
    clear: () => del('/api/me/downloads'),
  },
}

/**
 * Build a same-origin stream URL. Sending metadata lets upstream sources that
 * need a search fallback resolve the song correctly.
 */
export function streamUrl(song) {
  if (song.source === 'local' && song.url) return song.url
  const params = new URLSearchParams({ source: song.source, id: song.sourceId || song.id })
  if (song.extra && Object.keys(song.extra).length) {
    params.set('extra', JSON.stringify(song.extra))
  }
  if (song.title) params.set('title', song.title)
  if (song.artist) params.set('artist', song.artist)
  if (song.album) params.set('album', song.album)
  if (song.duration) params.set('duration', String(song.duration))
  if (song.ext) params.set('ext', song.ext)
  return `${BASE}/api/stream?${params.toString()}`
}

export function lyricParams(song) {
  const params = { source: song.source, id: song.sourceId || song.id }
  if (song.extra && Object.keys(song.extra).length) params.extra = JSON.stringify(song.extra)
  if (song.title) params.title = song.title
  if (song.artist) params.artist = song.artist
  if (song.album) params.album = song.album
  if (song.duration) params.duration = song.duration
  return params
}

export function formatDuration(seconds) {
  const total = Math.max(0, Math.round(Number(seconds) || 0))
  const m = Math.floor(total / 60)
  const s = total % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

export function formatSize(bytes) {
  const value = Number(bytes) || 0
  if (value <= 0) return '—'
  if (value >= 1024 * 1024 * 1024) return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`
  if (value >= 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`
  if (value >= 1024) return `${Math.round(value / 1024)} KB`
  return `${value} B`
}

export function formatBitrate(bitrate) {
  const value = Number(bitrate) || 0
  if (value <= 0) return '—'
  if (value >= 1000) return `${(value / 1000).toFixed(1)} Mbps`
  return `${value} kbps`
}

export function formatCount(count) {
  const value = Number(count) || 0
  if (value >= 100000000) return `${(value / 100000000).toFixed(1)}亿`
  if (value >= 10000) return `${(value / 10000).toFixed(1)}万`
  return String(value)
}

export function sourceLabel(source, sources) {
  const found = (sources || []).find((item) => item.id === source)
  return found && found.name ? found.name : source
}

export const sourceNames = {
  netease: '网易云',
  qq: 'QQ音乐',
  kugou: '酷狗',
  kuwo: '酷我',
  migu: '咪咕',
  fivesing: '5sing',
  jamendo: 'Jamendo',
  joox: 'JOOX',
  qianqian: '千千',
  soda: '汽水音乐',
  bilibili: 'B站',
  apple: 'Apple Music',
  spotify: 'Spotify',
  local: '本地',
}
