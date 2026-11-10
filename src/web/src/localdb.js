const DB_NAME = 'musicdl-local'
const DB_VERSION = 1
const STORE = 'tracks'

let dbPromise = null

function openDB() {
  if (!dbPromise) {
    dbPromise = new Promise((resolve, reject) => {
      if (!('indexedDB' in window)) {
        reject(new Error('当前浏览器不支持本地音乐存储'))
        return
      }
      const request = indexedDB.open(DB_NAME, DB_VERSION)
      request.onupgradeneeded = () => {
        const db = request.result
        if (!db.objectStoreNames.contains(STORE)) {
          db.createObjectStore(STORE, { keyPath: 'id' })
        }
      }
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error || new Error('本地音乐数据库打开失败'))
    })
  }
  return dbPromise
}

function transaction(mode) {
  return openDB().then((db) => db.transaction(STORE, mode).objectStore(STORE))
}

function wrap(request) {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

function parseFilename(filename) {
  const base = String(filename || '').replace(/\.[^.]+$/, '').replace(/^\d+[\s._-]+/, '').trim()
  const parts = base.split(/\s+-\s+/)
  if (parts.length >= 2) {
    return { artist: parts[0].trim(), title: parts.slice(1).join(' - ').trim() }
  }
  return { artist: '', title: base || '未命名音频' }
}

function extensionOf(filename) {
  const match = /\.([a-z0-9]+)$/i.exec(String(filename || ''))
  return match ? match[1].toLowerCase() : 'mp3'
}

function readDuration(file) {
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file)
    const audio = new Audio()
    audio.preload = 'metadata'
    const finish = (value) => {
      URL.revokeObjectURL(url)
      resolve(value)
    }
    audio.addEventListener('loadedmetadata', () => {
      finish(Number.isFinite(audio.duration) ? Math.round(audio.duration) : 0)
    })
    audio.addEventListener('error', () => finish(0))
    audio.src = url
  })
}

function toSong(track) {
  return {
    platform: 'go-music-dl',
    id: `local:${track.id}`,
    source: 'local',
    sourceId: track.id,
    title: track.title,
    artist: track.artist,
    album: track.album || '',
    duration: track.duration || 0,
    artwork: '',
    ext: track.ext || 'mp3',
    size: track.size || 0,
    url: URL.createObjectURL(track.blob),
    isLocal: true,
  }
}

export async function listLocalTracks() {
  const store = await transaction('readonly')
  const tracks = await wrap(store.getAll())
  return tracks
    .sort((a, b) => (b.addedAt || 0) - (a.addedAt || 0))
    .map(toSong)
}

export async function addLocalFiles(files) {
  const list = Array.from(files || []).filter((file) => file && String(file.type || '').startsWith('audio/'))
  if (!list.length) return 0
  const db = await openDB()
  let added = 0
  for (const file of list) {
    const duration = await readDuration(file)
    const parsed = parseFilename(file.name)
    const record = {
      id: `local_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`,
      title: parsed.title,
      artist: parsed.artist,
      album: '',
      duration,
      size: file.size,
      ext: extensionOf(file.name),
      mime: file.type || 'audio/mpeg',
      addedAt: Date.now(),
      blob: file,
    }
    const store = db.transaction(STORE, 'readwrite').objectStore(STORE)
    await wrap(store.put(record))
    added += 1
  }
  return added
}

export async function deleteLocalTrack(id) {
  const store = await transaction('readwrite')
  await wrap(store.delete(id))
}

export async function clearLocalTracks() {
  const store = await transaction('readwrite')
  await wrap(store.clear())
}

export async function getLocalSong(id) {
  if (!id) return null
  const store = await transaction('readonly')
  const track = await wrap(store.get(id))
  return track ? toSong(track) : null
}

export async function hydrateLocalSong(song) {
  if (!song || song.source !== 'local' || !song.sourceId) return song
  const fresh = await getLocalSong(song.sourceId)
  if (!fresh) return song
  return { ...song, ...fresh, id: song.id, artwork: song.artwork || fresh.artwork }
}

export async function hydrateLocalSongs(songs) {
  return Promise.all((songs || []).map((song) => hydrateLocalSong(song)))
}
