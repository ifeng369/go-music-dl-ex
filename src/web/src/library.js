import { reactive, watch } from 'vue'
import { api } from './api'

const LEGACY_KEY = 'musicdl.library.v1'
const CACHE_PREFIX = 'musicdl.library.2.'
const HISTORY_LIMIT = 300

export const library = reactive({
  favorites: [],
  history: [],
  playlists: [],
  sheetFavorites: [],
  // The play queue lives on the server too so another device can continue.
  playerQueue: { songs: [], index: 0, currentTime: 0 },
  // Songs whose working source was discovered once, reused on later plays.
  sourceOverrides: {},
  // Generated lists plus the recent-seen index used for de-duplication.
  recs: {
    daily: { cycle: '', songs: [], generatedAt: 0 },
    liked: { songs: [], generatedAt: 0 },
    random: { cycle: '', songs: [], generatedAt: 0 },
    hot: { cycle: '', songs: [], generatedAt: 0 },
    seen: {},
  },
})

let currentUserID = null
let hydrated = false
let hydrating = false
let saveTimer = null

function readJSON(key) {
  try {
    const raw = localStorage.getItem(key)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}

function writeJSON(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // Storage quota is not fatal; the server still receives the next save.
  }
}

function uniqueSongs(songs) {
  const seen = new Set()
  return (Array.isArray(songs) ? songs : []).filter((song) => {
    if (!song || !song.id) return false
    if (seen.has(song.id)) return false
    seen.add(song.id)
    return true
  })
}

function sheetFavoriteKey(sheet) {
  if (!sheet) return ''
  return `${sheet.source || ''}|${sheet.type || 'playlist'}|${sheet.id || ''}`
}

function normalizeSheetFavorite(sheet) {
  const item = sheet && sheet.sheet ? sheet.sheet : sheet
  return {
    id: String(item.id || ''),
    source: item.source || '',
    type: item.type || 'playlist',
    title: item.title || item.name || '未命名歌单',
    artwork: item.artwork || item.cover || '',
    artist: item.artist || item.creator || '',
    description: item.description || '',
    playCount: Number(item.playCount) || 0,
    trackCount: Number(item.trackCount || item.worksNum) || 0,
    link: item.link || '',
  }
}

function uniqueSheets(sheets) {
  const seen = new Set()
  return (Array.isArray(sheets) ? sheets : []).filter((sheet) => {
    if (!sheet || !sheet.id) return false
    const item = normalizeSheetFavorite(sheet)
    const key = sheetFavoriteKey(item)
    if (seen.has(key)) return false
    seen.add(key)
    item.savedAt = sheet.savedAt || Date.now()
    Object.assign(sheet, item)
    return true
  })
}

function hasData(data) {
  if (!data || typeof data !== 'object') return false
  const queued = data.playerQueue && Array.isArray(data.playerQueue.songs) && data.playerQueue.songs.length > 0
  if (queued) return true
  return ['favorites', 'history', 'playlists', 'sheetFavorites'].some(
    (key) => Array.isArray(data[key]) && data[key].length > 0,
  )
}

function applyData(data) {
  const queue = data && data.playerQueue
  library.playerQueue = {
    songs: Array.isArray(queue && queue.songs)
      ? queue.songs.filter((song) => song && song.source && (song.sourceId || song.id))
      : [],
    index: Math.max(0, Number(queue && queue.index) || 0),
    currentTime: Math.max(0, Number(queue && queue.currentTime) || 0),
  }
  if (data && data.sourceOverrides && typeof data.sourceOverrides === 'object') {
    library.sourceOverrides = { ...data.sourceOverrides }
  }
  const savedRecs = data && data.recs
  if (savedRecs && typeof savedRecs === 'object') {
    library.recs = {
      daily: savedRecs.daily || { cycle: '', songs: [], generatedAt: 0 },
      liked: savedRecs.liked || { songs: [], generatedAt: 0 },
      random: savedRecs.random || { cycle: '', songs: [], generatedAt: 0 },
      hot: savedRecs.hot || { cycle: '', songs: [], generatedAt: 0 },
      seen: savedRecs.seen && typeof savedRecs.seen === 'object' ? savedRecs.seen : {},
    }
  }
  library.favorites = uniqueSongs(data && data.favorites)
  library.history = uniqueSongs(data && data.history)
  library.sheetFavorites = uniqueSheets(data && data.sheetFavorites)
  library.playlists = Array.isArray(data && data.playlists)    ? data.playlists
        .filter((item) => item && item.id && item.name)
        .map((item) => ({
          id: String(item.id),
          name: String(item.name),
          createdAt: Number(item.createdAt) || Date.now(),
          updatedAt: Number(item.updatedAt) || Date.now(),
          songs: uniqueSongs(item.songs),
        }))
    : []
}

function serialize() {
  return {
    favorites: library.favorites,
    history: library.history,
    playlists: library.playlists,
    sheetFavorites: library.sheetFavorites,
    playerQueue: library.playerQueue,
    recs: library.recs,
    sourceOverrides: library.sourceOverrides,
  }
}

function cacheKey(id) {
  return `${CACHE_PREFIX}${id}`
}

function cacheLocally() {
  if (!currentUserID) return
  writeJSON(cacheKey(currentUserID), serialize())
}

async function saveNow() {
  if (!currentUserID) return
  try {
    await api.library.save(serialize())
  } catch {
    // Keep the per-user cache; a later change retries the save.
  }
}

function scheduleSave() {
  if (!currentUserID) return
  cacheLocally()
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    saveTimer = null
    saveNow()
  }, 600)
}

watch(
  library,
  () => {
    if (hydrating) return
    scheduleSave()
  },
  { deep: true },
)

export async function loadForUser(user) {
  if (!user) {
    resetLibrary()
    return
  }
  const id = String(user.id)
  if (currentUserID === id && hydrated) return

  hydrating = true
  currentUserID = id
  hydrated = false
  applyData({})

  const cached = readJSON(cacheKey(id))
  if (hasData(cached)) applyData(cached)

  try {
    const result = await api.library.get()
    const data = (result && result.data) || {}
    if (hasData(data)) {
      applyData(data)
      cacheLocally()
    } else if (user.role === 'admin') {
      const legacy = readJSON(LEGACY_KEY)
      if (hasData(legacy)) {
        applyData(legacy)
        await saveNow()
      }
    } else if (!hasData(cached)) {
      applyData({})
    }
  } catch {
    // Offline or not yet authenticated: keep the per-user cache.
  } finally {
    hydrating = false
    hydrated = true
    cacheLocally()
    for (const listener of libraryLoadedListeners) {
      try {
        listener()
      } catch {
        // A listener must never break hydration.
      }
    }
  }
}

const libraryLoadedListeners = []

export function onLibraryLoaded(listener) {
  if (typeof listener === 'function') libraryLoadedListeners.push(listener)
}

export function isLibraryHydrated() {
  return hydrated
}

export function getPlayerQueue() {
  const queue = library.playerQueue || { songs: [], index: 0, currentTime: 0 }
  return { songs: queue.songs || [], index: queue.index || 0, currentTime: queue.currentTime || 0 }
}

export function savePlayerQueue(data, force = false) {
  if (!currentUserID) return
  const songs = Array.isArray(data && data.songs) ? data.songs : []
  const index = Math.max(0, Number(data && data.index) || 0)
  const currentTime = Math.max(0, Number(data && data.currentTime) || 0)
  const prev = library.playerQueue || { songs: [], index: 0, currentTime: 0 }
  const unchangedQueue = prev.index === index && (prev.songs || []).length === songs.length
  if (!force && unchangedQueue && Math.abs((prev.currentTime || 0) - currentTime) < 15) {
    return
  }
  library.playerQueue = { songs, index, currentTime }
}

export function resetLibrary() {
  currentUserID = null
  hydrated = false
  hydrating = true
  applyData({})
  hydrating = false
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
}

function persistSoon() {
  scheduleSave()
}

export function isFavorite(songOrID) {
  const id = typeof songOrID === 'string' ? songOrID : songOrID && songOrID.id
  return Boolean(id) && library.favorites.some((song) => song.id === id)
}

export function toggleFavorite(song) {
  if (!song || !song.id) return false
  const index = library.favorites.findIndex((item) => item.id === song.id)
  if (index >= 0) {
    library.favorites.splice(index, 1)
    persistSoon()
    return false
  }
  library.favorites.unshift(song)
  persistSoon()
  return true
}

export function removeFavorite(songOrID) {
  const id = typeof songOrID === 'string' ? songOrID : songOrID && songOrID.id
  const index = library.favorites.findIndex((song) => song.id === id)
  if (index >= 0) {
    library.favorites.splice(index, 1)
    persistSoon()
  }
}

export function clearFavorites() {
  library.favorites.splice(0, library.favorites.length)
  persistSoon()
}

export function addHistory(song) {
  if (!song || !song.id) return
  const index = library.history.findIndex((item) => item.id === song.id)
  if (index >= 0) library.history.splice(index, 1)
  library.history.unshift({ ...song, playedAt: Date.now() })
  if (library.history.length > HISTORY_LIMIT) library.history.splice(HISTORY_LIMIT)
  persistSoon()
}

export function clearHistory() {
  library.history.splice(0, library.history.length)
  persistSoon()
}

export function createPlaylist(name) {
  const title = String(name || '').trim() || '新建歌单'
  const playlist = {
    id: `pl_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`,
    name: title,
    createdAt: Date.now(),
    updatedAt: Date.now(),
    songs: [],
  }
  library.playlists.unshift(playlist)
  persistSoon()
  return playlist
}

export function renamePlaylist(id, name) {
  const playlist = library.playlists.find((item) => item.id === id)
  const title = String(name || '').trim()
  if (!playlist || !title) return false
  playlist.name = title
  playlist.updatedAt = Date.now()
  persistSoon()
  return true
}

export function deletePlaylist(id) {
  const index = library.playlists.findIndex((item) => item.id === id)
  if (index >= 0) library.playlists.splice(index, 1)
  persistSoon()
}

export function addSongToPlaylist(id, song) {
  const playlist = library.playlists.find((item) => item.id === id)
  if (!playlist || !song || !song.id) return false
  if (playlist.songs.some((item) => item.id === song.id)) return false
  playlist.songs.push(song)
  playlist.updatedAt = Date.now()
  persistSoon()
  return true
}

export function removeSongFromPlaylist(id, songOrIndex) {
  const playlist = library.playlists.find((item) => item.id === id)
  if (!playlist) return
  const index = typeof songOrIndex === 'number'
    ? songOrIndex
    : playlist.songs.findIndex((song) => song.id === (songOrIndex && songOrIndex.id))
  if (index >= 0) {
    playlist.songs.splice(index, 1)
    playlist.updatedAt = Date.now()
    persistSoon()
  }
}

export function playlistByID(id) {
  return library.playlists.find((item) => item.id === id) || null
}

export function isSheetFavorite(sheetOrKey) {
  const key = typeof sheetOrKey === 'string' ? sheetOrKey : sheetFavoriteKey(sheetOrKey)
  return Boolean(key) && library.sheetFavorites.some((sheet) => sheetFavoriteKey(sheet) === key)
}

export function toggleSheetFavorite(sheet) {
  const item = normalizeSheetFavorite(sheet)
  const key = sheetFavoriteKey(item)
  if (!key) return false
  const index = library.sheetFavorites.findIndex((entry) => sheetFavoriteKey(entry) === key)
  if (index >= 0) {
    library.sheetFavorites.splice(index, 1)
    persistSoon()
    return false
  }
  library.sheetFavorites.unshift({ ...item, savedAt: Date.now() })
  persistSoon()
  return true
}

export function removeSheetFavorite(sheetOrKey) {
  const key = typeof sheetOrKey === 'string' ? sheetOrKey : sheetFavoriteKey(sheetOrKey)
  const index = library.sheetFavorites.findIndex((entry) => sheetFavoriteKey(entry) === key)
  if (index >= 0) {
    library.sheetFavorites.splice(index, 1)
    persistSoon()
  }
}

export function clearSheetFavorites() {
  library.sheetFavorites.splice(0, library.sheetFavorites.length)
  persistSoon()
}
