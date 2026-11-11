import { reactive } from 'vue'
import { api } from './api'
import { library } from './library'

/**
 * Three generated song lists:
 *  - daily  : rebuilt after 06:00, 50 tracks picked from platform recommendations
 *  - liked  : built from local songs / playlists / favourites, rebuilt on demand
 *  - random : 20 random tracks, extended by 20 whenever 2 are left
 * Tracks generated inside the freshness window are not reused.
 */
const DAILY_WINDOW = 30 * 24 * 60 * 60 * 1000
const RANDOM_WINDOW = 7 * 24 * 60 * 60 * 1000

export const recs = reactive({
  daily: { cycle: '', songs: [], generatedAt: 0 },
  liked: { songs: [], generatedAt: 0 },
  random: { songs: [], generatedAt: 0 },
  loading: { daily: false, liked: false, random: false },
  error: '',
})

export function songKey(song) {
  if (!song) return ''
  return `${song.source || ''}:${song.sourceId || song.id || ''}`
}

function cycleKey(date = new Date()) {
  const copy = new Date(date)
  if (copy.getHours() < 6) copy.setDate(copy.getDate() - 1)
  return `${copy.getFullYear()}-${String(copy.getMonth() + 1).padStart(2, '0')}-${String(copy.getDate()).padStart(2, '0')}`
}

function seenMap() {
  if (!library.recs || typeof library.recs !== 'object') {
    library.recs = { seen: {} }
  }
  if (!library.recs.seen || typeof library.recs.seen !== 'object') {
    library.recs.seen = {}
  }
  return library.recs.seen
}

function isFresh(song, windowMs) {
  const stamp = seenMap()[songKey(song)]
  return Boolean(stamp) && Date.now() - stamp < windowMs
}

function markSeen(songs) {
  const seen = seenMap()
  const now = Date.now()
  for (const song of songs) {
    const key = songKey(song)
    if (key) seen[key] = now
  }
}

function shuffle(list) {
  const array = [...list]
  for (let i = array.length - 1; i > 0; i -= 1) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[array[i], array[j]] = [array[j], array[i]]
  }
  return array
}

function dedupeSongs(list) {
  const seen = new Set()
  const out = []
  for (const song of list || []) {
    const key = songKey(song)
    if (!key || seen.has(key)) continue
    if (!song.source || !(song.sourceId || song.id)) continue
    seen.add(key)
    out.push(song)
  }
  return out
}

/** Round-robin across sources so one provider cannot dominate a list. */
function mixedPick(pool, count) {
  const buckets = new Map()
  for (const song of shuffle(pool)) {
    const source = song.source || ''
    if (!buckets.has(source)) buckets.set(source, [])
    buckets.get(source).push(song)
  }
  const queues = [...buckets.values()]
  const picked = []
  let empty = false
  while (picked.length < count && !empty) {
    empty = true
    for (const queue of queues) {
      if (!queue.length) continue
      empty = false
      picked.push(queue.shift())
      if (picked.length >= count) break
    }
  }
  return picked
}

async function recommendationPool(sheetLimit = 4) {
  const pool = []
  try {
    const home = await api.home()
    const groups = (home.recommendGroups || []).slice(0, sheetLimit)
    const jobs = []
    for (const group of groups) {
      for (const sheet of (group.data || []).slice(0, 3)) {
        jobs.push(
          api
            .playlist({ source: group.source, id: sheet.id, title: sheet.title, limit: 80 })
            .then((detail) => ({ group, songs: detail.musicList || [] }))
            .catch(() => ({ group, songs: [] })),
        )
      }
    }
    for (const result of await Promise.all(jobs)) {
      for (const song of result.songs) pool.push({ ...song, source: song.source || result.group.source })
    }
  } catch {
    // Ignore: the caller falls back to the local library.
  }
  if (pool.length < 30) {
    // Sheet endpoints can be rate limited; broad searches are far more stable
    // and still represent what each platform pushes right now.
    const themes = ['热歌', '新歌', '流行', '经典老歌', '华语', '轻音乐']
    const jobs = themes.map((keyword) =>
      api
        .search({ q: keyword, type: 'music', limit: 40 })
        .then((result) => result.items || [])
        .catch(() => []),
    )
    for (const items of await Promise.all(jobs)) pool.push(...items)
  }
  return pool
}

function tasteSeeds() {
  const songs = [
    ...(library.favorites || []),
    ...(library.playlists || []).flatMap((item) => item.songs || []),
    ...(library.history || []).slice(0, 60),
  ]
  const artists = new Map()
  for (const song of songs) {
    const name = (song.artist || '').split(/[、,/&]/)[0].trim()
    if (!name || name === '未知歌手') continue
    artists.set(name, (artists.get(name) || 0) + 1)
  }
  return {
    artists: [...artists.entries()].sort((a, b) => b[1] - a[1]).slice(0, 4).map(([name]) => name),
    songs: dedupeSongs(songs),
  }
}

async function likedPool() {
  const { artists, songs } = tasteSeeds()
  const pool = [...songs]
  const jobs = artists.map((artist) =>
    api
      .search({ q: artist, type: 'music', limit: 30 })
      .then((result) => result.items || [])
      .catch(() => []),
  )
  for (const items of await Promise.all(jobs)) pool.push(...items)
  return pool
}

function takeFresh(pool, count, windowMs) {
  const fresh = dedupeSongs(pool).filter((song) => !isFresh(song, windowMs))
  return mixedPick(fresh, count)
}

export async function buildDaily(force = false) {
  const cycle = cycleKey()
  if (!force && recs.daily.cycle === cycle && recs.daily.songs.length) return recs.daily.songs
  recs.loading.daily = true
  recs.error = ''
  try {
    let pool = await recommendationPool(5)
    let songs = takeFresh(pool, 50, DAILY_WINDOW)
    if (songs.length < 50) {
      // Recommendations can come back thin when a platform is rate limited;
      // top up with the listener's own taste instead of showing an empty list.
      const fallback = await likedPool()
      const merged = dedupeSongs([...songs, ...pool, ...fallback])
      songs = takeFresh(merged, 50, DAILY_WINDOW)
    }
    if (!songs.length) {
      songs = dedupeSongs([...(library.favorites || []), ...(library.history || [])]).slice(0, 50)
    }
    recs.daily = { cycle, songs, generatedAt: Date.now() }
  } catch (err) {
    recs.error = err.message || '每日推荐生成失败'
  } finally {
    recs.loading.daily = false
  }
  return recs.daily.songs
}

export async function buildLiked(force = false) {
  if (!force && recs.liked.songs.length) return recs.liked.songs
  recs.loading.liked = true
  recs.error = ''
  try {
    const pool = await likedPool()
    const songs = takeFresh(pool, 50, DAILY_WINDOW)
    recs.liked = { songs, generatedAt: Date.now() }
  } catch (err) {
    recs.error = err.message || '猜你喜欢生成失败'
  } finally {
    recs.loading.liked = false
  }
  return recs.liked.songs
}

export async function buildRandom(force = false) {
  const cycle = cycleKey()
  if (!force && recs.random.songs.length && recs.random.cycle === cycle) return recs.random.songs
  recs.loading.random = true
  recs.error = ''
  try {
    const pool = await recommendationPool(4)
    const songs = takeFresh(pool, 20, RANDOM_WINDOW)
    recs.random = { cycle, songs, generatedAt: Date.now() }
    markSeen(songs)
  } catch (err) {
    recs.error = err.message || '随便听听生成失败'
  } finally {
    recs.loading.random = false
  }
  return recs.random.songs
}

/** Extra batch appended when the random queue is almost finished. */
export async function nextRandomBatch(count = 20) {
  const pool = await recommendationPool(4)
  const fresh = takeFresh(pool, count, RANDOM_WINDOW)
  markSeen(fresh)
  recs.random.songs.push(...fresh)
  return fresh
}

export function markDailySeen(songs) {
  markSeen(songs)
}

export function hydrateRecs() {
  const saved = library.recs
  if (!saved || typeof saved !== 'object') return
  if (saved.daily && Array.isArray(saved.daily.songs)) recs.daily = { ...recs.daily, ...saved.daily }
  if (saved.liked && Array.isArray(saved.liked.songs)) recs.liked = { ...recs.liked, ...saved.liked }
  if (saved.random && Array.isArray(saved.random.songs)) recs.random = { ...recs.random, ...saved.random }
}

export function snapshotRecs() {
  return {
    daily: { cycle: recs.daily.cycle, songs: recs.daily.songs, generatedAt: recs.daily.generatedAt },
    liked: { songs: recs.liked.songs, generatedAt: recs.liked.generatedAt },
    random: { cycle: recs.random.cycle, songs: recs.random.songs, generatedAt: recs.random.generatedAt },
    seen: seenMap(),
  }
}
