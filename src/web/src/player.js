import { computed, reactive, watch } from 'vue'
import { api, lyricParams, streamUrl } from './api'
import { addHistory, getPlayerQueue, isFavorite, onLibraryLoaded, savePlayerQueue, toggleFavorite } from './library'
import { hydrateLocalSongs } from './localdb'
import { sourceName } from './meta'

const rawVolume = localStorage.getItem('musicdl.volume')
// Number(null) is 0, which silently muted every first-time visitor.
const savedVolume = rawVolume === null || rawVolume === '' ? NaN : Number(rawVolume)
const savedMode = localStorage.getItem('musicdl.mode')

const state = reactive({
  queue: [],
  index: -1,
  playing: false,
  loading: false,
  currentTime: 0,
  duration: 0,
  volume: Number.isFinite(savedVolume) ? Math.min(1, Math.max(0, savedVolume)) : 0.8,
  muted: false,
  mode: savedMode || 'list',
  error: '',
  lyrics: [],
  lyricIndex: -1,
  showQueue: false,
  showNowPlaying: false,
  alternatives: [],
  alternativesLoading: false,
  switching: false,
})

const audio = new Audio()
// Buffer ahead instead of only fetching metadata: short preload windows are the
// main cause of mid-song stalls on phones with a slow link.
audio.preload = 'auto'
audio.setAttribute('playsinline', '')
audio.volume = state.volume
// Safari treats a media element that is not in the document as a detached
// stream and can throttle it while the page renders; mounting it hidden keeps
// decoding on the normal playback path.
audio.style.display = 'none'
if (document.body) {
  document.body.appendChild(audio)
} else {
  document.addEventListener('DOMContentLoaded', () => document.body.appendChild(audio))
}
// A media element created without a source can inherit a stale rate on iOS,
// which is audible as slowed-down playback. Pin both rates to 1x.
function normalizePlaybackRate() {
  if (audio.playbackRate !== 1) audio.playbackRate = 1
  if (audio.defaultPlaybackRate !== 1) audio.defaultPlaybackRate = 1
}
audio.addEventListener('play', normalizePlaybackRate)
audio.addEventListener('playing', normalizePlaybackRate)
let playToken = 0
let altToken = 0
const altCache = new Map()
let fallbackState = null

const SOURCE_OVERRIDE_KEY = 'musicdl.sourceOverrides.v1'

function loadSourceOverrides() {
  try {
    const raw = localStorage.getItem(SOURCE_OVERRIDE_KEY)
    const parsed = raw ? JSON.parse(raw) : {}
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

let sourceOverrides = loadSourceOverrides()

function persistSourceOverrides() {
  try {
    localStorage.setItem(SOURCE_OVERRIDE_KEY, JSON.stringify(sourceOverrides))
  } catch {
    // Ignore storage quota errors.
  }
}

function applySourceOverride(song) {
  if (!song || !song.id) return song
  let current = song
  const seen = new Set()
  while (current && current.id && sourceOverrides[current.id] && !seen.has(current.id)) {
    seen.add(current.id)
    current = { ...sourceOverrides[current.id] }
  }
  return current
}

function rememberSourceOverride(original, replacement) {
  if (!original || !replacement || !original.id || !replacement.id) return
  if (original.id === replacement.id) return
  sourceOverrides[original.id] = { ...replacement }
  persistSourceOverrides()
}
let fallbackBusy = false

const current = computed(() => (state.index >= 0 ? state.queue[state.index] : null))
const progress = computed(() => (state.duration ? state.currentTime / state.duration : 0))

function parseLrc(raw) {
  if (!raw) return []
  const lines = []
  const timeTag = /\[(\d{1,3}):(\d{2})(?:[.:](\d{1,3}))?\]/g
  for (const line of raw.split(/\r?\n/)) {
    const text = line.replace(timeTag, '').trim()
    if (!text) continue
    let match
    timeTag.lastIndex = 0
    while ((match = timeTag.exec(line))) {
      const minutes = Number(match[1])
      const seconds = Number(match[2])
      const fraction = match[3] ? Number(`0.${match[3]}`) : 0
      lines.push({ time: minutes * 60 + seconds + fraction, text })
    }
  }
  lines.sort((a, b) => a.time - b.time)
  // Some providers return each line twice (merged lyric sources), which showed
  // up as every lyric repeated back to back.
  const deduped = []
  for (const entry of lines) {
    const prev = deduped[deduped.length - 1]
    if (prev && prev.text === entry.text) continue
    deduped.push(entry)
  }
  return deduped
}

function syncLyricIndex() {
  const lyrics = state.lyrics
  if (!lyrics.length) {
    state.lyricIndex = -1
    return
  }
  let index = -1
  for (let i = 0; i < lyrics.length; i += 1) {
    if (lyrics[i].time <= state.currentTime + 0.35) index = i
    else break
  }
  state.lyricIndex = index
}

async function loadLyric(song) {
  state.lyrics = []
  state.lyricIndex = -1
  if (!song) return
  try {
    const data = await api.lyric(lyricParams(song))
    state.lyrics = parseLrc(data.rawLrc || data.lrc || '')
    syncLyricIndex()
  } catch {
    state.lyrics = []
  }
}

function updateMediaSession(song) {
  if (!('mediaSession' in navigator)) return
  navigator.mediaSession.metadata = new MediaMetadata({
    title: song.title || '未知歌曲',
    artist: song.artist || '',
    album: song.album || '',
    artwork: song.artwork ? [{ src: song.artwork, sizes: '512x512', type: 'image/jpeg' }] : [],
  })
}

function altKey(song) {
  if (!song) return ''
  return `${song.source}|${song.sourceId || song.id}|${song.title || ''}|${song.artist || ''}`
}

async function fetchAlternatives(song) {
  if (!song || song.source === 'local' || !song.title) return []
  const key = altKey(song)
  if (altCache.has(key)) return altCache.get(key)
  try {
    const data = await api.alternatives({
      title: song.title,
      artist: song.artist,
      duration: song.duration,
      source: song.source,
    })
    const items = (data.items || []).filter(
      (item) => item && item.source && item.source !== 'local' && item.source !== song.source,
    )
    altCache.set(key, items)
    return items
  } catch {
    altCache.set(key, [])
    return []
  }
}

async function loadAlternatives(song = current.value) {
  if (!song || song.source === 'local') {
    state.alternatives = []
    return []
  }
  const token = ++altToken
  state.alternativesLoading = true
  const items = await fetchAlternatives(song)
  if (token === altToken && current.value && altKey(current.value) === altKey(song)) {
    state.alternatives = items
  }
  if (token === altToken) state.alternativesLoading = false
  return items
}

function applySource(song) {
  if (!song || state.index < 0) return
  state.queue.splice(state.index, 1, song)
  playAt(state.index, { keepFallback: true })
}

async function tryNextFallback() {
  if (fallbackBusy) return
  const song = current.value
  if (!song) return
  if (song.source === 'local') {
    state.error = '播放失败，本地文件可能已被移动或删除'
    return
  }

  fallbackBusy = true
  if (!fallbackState) {
    state.switching = true
    state.error = '播放失败，正在自动换源…'
    const candidates = await fetchAlternatives(song)
    if (!current.value || altKey(current.value) !== altKey(song)) {
      state.switching = false
      fallbackBusy = false
      return
    }
    fallbackState = {
      candidates: candidates.slice(0, 10),
      next: 0,
      tried: new Set([song.source]),
      original: song,
    }
    state.alternatives = candidates
  }

  while (fallbackState && fallbackState.next < fallbackState.candidates.length) {
    const candidate = fallbackState.candidates[fallbackState.next]
    fallbackState.next += 1
    if (!candidate || fallbackState.tried.has(candidate.source)) continue
    fallbackState.tried.add(candidate.source)
    state.switching = true
    state.error = `播放失败，正在自动切换到${sourceName(candidate.source)}…`
    applySource(candidate)
    fallbackBusy = false
    return
  }

  state.switching = false
  state.error = '播放失败，已尝试其他可用音源'
  fallbackState = null
  fallbackBusy = false
}

function switchSource(song) {
  const currentSong = current.value
  if (!song || !currentSong || song.id === currentSong.id) return
  const candidates = state.alternatives.slice()
  const selected = candidates.findIndex((item) => item.id === song.id)
  fallbackState = {
    candidates,
    next: selected >= 0 ? selected + 1 : 0,
    tried: new Set([currentSong.source, song.source]),
    original: currentSong,
  }
  state.error = ''
  applySource(song)
}

function playAt(index, options = {}) {
  let song = state.queue[index]
  if (!song) return
  if (!options.keepFallback) {
    const resolved = applySourceOverride(song)
    if (resolved !== song) {
      song = resolved
      state.queue.splice(index, 1, song)
    }
  }
  playToken += 1
  if (!options.keepFallback) {
    fallbackState = null
    fallbackBusy = false
    state.switching = false
    state.alternatives = []
    altToken += 1
  }
  state.index = index
  addHistory(song)
  state.loading = true
  state.error = ''
  state.currentTime = 0
  state.duration = song.duration || 0
  audio.src = streamUrl(song)
  updateMediaSession(song)
  loadLyric(song)
  audio.play().catch((error) => {
    state.loading = false
    state.playing = false
    if (error && error.name === 'NotAllowedError') {
      state.error = '点击播放按钮开始播放'
      return
    }
    tryNextFallback()
  })
}

// ---------------------------------------------------------------- precheck
// Resolving a playable URL is the slowest part of starting a track and the
// upstream providers fail intermittently. Warm the queue in the background so
// a broken item is swapped for a working source before the user reaches it.

const precheckCache = new Set()
let precheckToken = 0

function precheckKey(song) {
  return song && song.id ? String(song.id) : ''
}

async function probePlayable(song) {
  const params = { source: song.source, id: song.sourceId || song.id }
  if (song.extra && Object.keys(song.extra).length) params.extra = JSON.stringify(song.extra)
  if (song.title) params.title = song.title
  if (song.artist) params.artist = song.artist
  if (song.album) params.album = song.album
  if (song.duration) params.duration = song.duration
  try {
    const data = await api.songUrl(params)
    return Boolean(data && data.url)
  } catch {
    return false
  }
}

async function precheckQueue(songs) {
  const list = (songs || []).filter(
    (song) => song && song.source && song.source !== 'local' && !precheckCache.has(precheckKey(song)),
  )
  if (!list.length) return
  const token = ++precheckToken
  const batch = list.slice(0, 40)
  let cursor = 0

  const worker = async () => {
    while (cursor < batch.length) {
      const song = batch[cursor]
      cursor += 1
      if (token !== precheckToken) return
      const key = precheckKey(song)
      if (!key || precheckCache.has(key)) continue
      const playable = await probePlayable(song)
      if (token !== precheckToken) return
      if (playable) {
        precheckCache.add(key)
        continue
      }
      // Not playable: look for a working replacement and rewire the queue entry
      // while it is still upcoming.
      const candidates = await fetchAlternatives(song)
      if (token !== precheckToken) return
      const replacement = candidates.find((item) => item && item.source && item.source !== song.source)
      const index = state.queue.findIndex((item) => item.id === song.id)
      precheckCache.add(key)
      if (!replacement || index < 0 || index === state.index) continue
      state.queue.splice(index, 1, replacement)
      rememberSourceOverride(song, replacement)
      precheckCache.add(precheckKey(replacement))
    }
  }

  await Promise.all([worker(), worker(), worker()])
}

function playSongs(songs, start = 0) {
  const list = (songs || []).map(applySourceOverride).filter((song) => song && song.source && (song.sourceId || song.id))
  if (!list.length) return
  const token = ++playToken
  hydrateLocalSongs(list).then((resolved) => {
    if (token !== playToken) return
    state.queue = resolved
    playAt(Math.min(Math.max(0, start), resolved.length - 1))
    precheckQueue(resolved)
  })
}

function playSong(song) {
  playSongs([song], 0)
}

function toggle() {
  if (!current.value) {
    if (state.queue.length) playAt(0)
    return
  }
  if (audio.paused) {
    audio.play().catch(() => {})
  } else {
    audio.pause()
  }
}

function next(fromEnded = false) {
  if (!state.queue.length) return
  if (fromEnded && state.mode === 'one') {
    audio.currentTime = 0
    audio.play().catch(() => {})
    return
  }
  if (state.mode === 'shuffle' && state.queue.length > 1) {
    let index = state.index
    while (index === state.index) index = Math.floor(Math.random() * state.queue.length)
    playAt(index)
    return
  }
  playAt((state.index + 1) % state.queue.length)
}

function prev() {
  if (!state.queue.length) return
  if (audio.currentTime > 3) {
    seek(0)
    return
  }
  playAt((state.index - 1 + state.queue.length) % state.queue.length)
}

function seek(time) {
  if (!Number.isFinite(time)) return
  const target = Math.min(Math.max(0, time), state.duration || time)
  // Sliders and lyrics also report the position they were rendered with, which
  // arrives a fraction behind the playhead. Writing currentTime for those echo
  // values restarts the decoder several times a second, so playback ran slow
  // and choppy. Ignore seeks that barely move the playhead.
  if (Math.abs(target - audio.currentTime) < 0.5) {
    state.currentTime = audio.currentTime
    syncLyricIndex()
    return
  }
  audio.currentTime = target
  state.currentTime = target
  syncLyricIndex()
}

function setVolume(value) {
  const volume = Math.min(1, Math.max(0, Number(value) || 0))
  state.volume = volume
  state.muted = volume === 0
  audio.volume = volume
  audio.muted = false
  localStorage.setItem('musicdl.volume', String(volume))
}

function toggleMute() {
  state.muted = !state.muted
  audio.muted = state.muted
}

function cycleMode() {
  state.mode = state.mode === 'list' ? 'one' : state.mode === 'one' ? 'shuffle' : 'list'
  localStorage.setItem('musicdl.mode', state.mode)
}

function addToQueue(songs) {
  const added = []
  for (const song of songs || []) {
    if (!song || !song.source) continue
    if (!state.queue.some((item) => item.id === song.id)) {
      state.queue.push(song)
      added.push(song)
    }
  }
  if (added.length) precheckQueue(added)
}

function removeFromQueue(index) {
  if (index < 0 || index >= state.queue.length) return
  state.queue.splice(index, 1)
  if (index === state.index) {
    if (state.queue.length) playAt(Math.min(index, state.queue.length - 1))
    else clearQueue()
  } else if (index < state.index) {
    state.index -= 1
  }
}

function clearQueue() {
  audio.pause()
  audio.removeAttribute('src')
  state.queue = []
  state.index = -1
  state.playing = false
  state.currentTime = 0
  state.duration = 0
  state.lyrics = []
  state.showNowPlaying = false
  state.alternatives = []
  state.alternativesLoading = false
  state.switching = false
  fallbackState = null
  fallbackBusy = false
  altToken += 1
  syncNativeState(true)
}

audio.addEventListener('loadedmetadata', () => {
  state.duration = Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration : state.duration
})
audio.addEventListener('timeupdate', () => {
  state.currentTime = audio.currentTime
  syncLyricIndex()
})
audio.addEventListener('play', () => {
  state.playing = true
  state.loading = false
})
audio.addEventListener('pause', () => {
  state.playing = false
})
audio.addEventListener('waiting', () => {
  state.loading = true
})
audio.addEventListener('playing', () => {
  if (fallbackState && fallbackState.original && current.value && current.value.id !== fallbackState.original.id) {
    rememberSourceOverride(fallbackState.original, current.value)
  }
  state.loading = false
  state.switching = false
  fallbackState = null
  fallbackBusy = false
  state.error = ''
})
audio.addEventListener('canplay', () => {
  state.loading = false
})
audio.addEventListener('ended', () => next(true))
audio.addEventListener('error', () => {
  state.loading = false
  state.playing = false
  tryNextFallback()
})

if ('mediaSession' in navigator) {
  navigator.mediaSession.setActionHandler('play', () => toggle())
  navigator.mediaSession.setActionHandler('pause', () => toggle())
  navigator.mediaSession.setActionHandler('previoustrack', () => prev())
  navigator.mediaSession.setActionHandler('nexttrack', () => next())
  navigator.mediaSession.setActionHandler('seekto', (details) => seek(details.seekTime))
}

export const player = {
  state,
  current,
  progress,
  playSongs,
  playSong,
  playAt,
  toggle,
  next,
  prev,
  seek,
  setVolume,
  toggleMute,
  cycleMode,
  addToQueue,
  removeFromQueue,
  clearQueue,
  loadAlternatives,
  switchSource,
}

/* ---------------- Playback queue persistence ---------------- */
const QUEUE_STORAGE_KEY = 'musicdl.player.queue.v1'
let lastQueueSaveAt = 0

function loadPersistedQueue() {
  try {
    const raw = localStorage.getItem(QUEUE_STORAGE_KEY)
    if (!raw) return null
    const data = JSON.parse(raw)
    if (!data || !Array.isArray(data.queue)) return null
    return data
  } catch {
    return null
  }
}

function persistQueue(force = false) {
  const now = Date.now()
  if (!force && now - lastQueueSaveAt < 1000) return
  lastQueueSaveAt = now
  try {
    localStorage.setItem(QUEUE_STORAGE_KEY, JSON.stringify({
      queue: state.queue,
      index: state.index,
      currentTime: state.currentTime,
      duration: state.duration,
    }))
  } catch {
    // Ignore storage quota errors.
  }
  savePlayerQueue(
    { songs: state.queue, index: state.index, currentTime: state.currentTime },
    force,
  )
}

function persistQueueProgress() {
  if (Date.now() - lastQueueSaveAt < 5000) return
  persistQueue(true)
}

watch(() => state.queue, () => persistQueue(true), { deep: true })
watch(() => state.index, () => persistQueue(true))
audio.addEventListener('timeupdate', persistQueueProgress)
audio.addEventListener('pause', () => persistQueue(true))
window.addEventListener('pagehide', () => persistQueue(true))

// ---------------------------------------------------------- Android shell
// The WebView shell mirrors the current track into a system MediaSession so the
// lock screen, notification shade and Bluetooth controls can drive playback.
let lastNativeSync = 0
let lastNativeKey = ''

function publishNativeState() {
  if (typeof window === 'undefined' || !window.MusicDLNative) return
  const song = current.value
  if (!song) {
    try {
      window.MusicDLNative.clearMediaState()
    } catch {
      // Browser build: nothing to do.
    }
    return
  }
  try {
    window.MusicDLNative.updateMediaState(JSON.stringify({
      title: song.title || '',
      artist: song.artist || '',
      album: song.album || '',
      artwork: song.artwork || song.cover || '',
      playing: state.playing,
      liked: Boolean(current.value && isFavorite(current.value)),
      position: state.currentTime || 0,
      duration: state.duration || song.duration || 0,
      hasQueue: state.queue.length > 1,
    }))
  } catch {
    // The bridge only exists inside the Android shell.
  }
}

function syncNativeState(force = false) {
  if (typeof window === 'undefined' || !window.MusicDLNative) return
  const key = `${current.value ? current.value.id : ''}|${state.playing}`
  const now = Date.now()
  if (!force && key === lastNativeKey && now - lastNativeSync < 4000) return
  lastNativeKey = key
  lastNativeSync = now
  publishNativeState()
}

audio.addEventListener('play', () => syncNativeState(true))
audio.addEventListener('pause', () => syncNativeState(true))
audio.addEventListener('timeupdate', () => syncNativeState(false))

if (typeof window !== 'undefined') {
  window.__musicdlCommand = (command) => {
    const cmd = String(command || '')
    if (cmd === 'play') {
      if (audio.paused) audio.play().catch(() => {})
    } else if (cmd === 'pause') {
      audio.pause()
    } else if (cmd === 'toggle') {
      toggle()
    } else if (cmd === 'next') {
      next()
    } else if (cmd === 'prev') {
      prev()
    } else if (cmd === 'like') {
      if (current.value) toggleFavorite(current.value)
    } else if (cmd === 'stop') {
      audio.pause()
      state.showNowPlaying = false
    } else if (cmd.startsWith('seek:')) {
      seek(Number(cmd.slice(5)))
    }
    syncNativeState(true)
  }
}

async function restorePersistedQueue() {
  const saved = loadPersistedQueue()
  if (!saved) return
  const queue = saved.queue
    .map(applySourceOverride)
    .filter((song) => song && song.source && (song.sourceId || song.id))
  if (!queue.length) return

  const token = playToken
  const index = Math.min(Math.max(0, Number(saved.index) || 0), queue.length - 1)
  const resume = Math.max(0, Number(saved.currentTime) || 0)
  const duration = Number(saved.duration) || queue[index].duration || 0

  state.queue = queue
  state.index = index
  state.currentTime = resume
  state.duration = duration

  const hydrated = await hydrateLocalSongs(state.queue)
  if (token !== playToken) return
  state.queue = hydrated

  const song = current.value
  if (!song) return
  audio.src = streamUrl(song)
  state.duration = duration || song.duration || 0
  if (resume > 0) {
    const seekOnMetadata = () => {
      try {
        audio.currentTime = Math.min(resume, audio.duration || resume)
      } catch {
        // The source may not support seeking; ignore.
      }
      audio.removeEventListener('loadedmetadata', seekOnMetadata)
    }
    audio.addEventListener('loadedmetadata', seekOnMetadata)
  }
  updateMediaSession(song)
  loadLyric(song)
}

restorePersistedQueue()
// Adopt the queue stored on the server when this device has none yet.
onLibraryLoaded(() => {
  if (state.queue.length) return
  const saved = getPlayerQueue()
  const songs = (saved.songs || []).filter(
    (song) => song && song.source && song.source !== 'local' && (song.sourceId || song.id),
  )
  if (!songs.length) return
  const index = Math.min(Math.max(0, Number(saved.index) || 0), songs.length - 1)
  state.queue = songs
  state.index = index
  state.currentTime = Math.max(0, Number(saved.currentTime) || 0)
  state.duration = songs[index].duration || 0
})
