import { reactive } from 'vue'
import { api } from './api'
import { library } from './library'
import { loadMeta, meta } from './meta'

/**
 * Four generated song lists kept on the server:
 *  daily : 50 tracks, rebuilt once per 06:00 cycle
 *  liked : 20 tracks from the listener's taste, extended by 20 near the end
 *  random: 20 random tracks, extended by 20 near the end
 *  hot   : up to 100 tracks taken from every platform toplist, in platform order
 */
const DAILY_WINDOW = 30 * 24 * 60 * 60 * 1000
const RANDOM_WINDOW = 7 * 24 * 60 * 60 * 1000

export const recs = reactive({
  daily: { cycle: '', songs: [], generatedAt: 0 },
  liked: { songs: [], generatedAt: 0 },
  random: { cycle: '', songs: [], generatedAt: 0 },
  hot: { cycle: '', songs: [], generatedAt: 0 },
  loading: { daily: false, liked: false, random: false, hot: false },
  error: '',
})

export const REC_META = {
  daily: { title: '每日推荐', desc: '每天 6:00 更新，50 首，来自各平台推荐' },
  liked: { title: '你喜欢听', desc: '根据本地歌曲、我的歌单和收藏音乐的偏好生成，剩 2 首自动续 20 首' },
  random: { title: '随便听听', desc: '每天 6:00 换 20 首，剩 2 首自动追加 20 首' },
  hot: { title: '热门榜单', desc: '汇总各平台热门榜单 100 首，按平台热度顺序排列，每天 6:00 更新' },
}

export function songKey(song) {
  if (!song) return ''
  return `${song.source || ''}:${song.sourceId || song.id || ''}`
}

export function cycleKey(date = new Date()) {
  const copy = new Date(date)
  if (copy.getHours() < 6) copy.setDate(copy.getDate() - 1)
  return `${copy.getFullYear()}-${String(copy.getMonth() + 1).padStart(2, '0')}-${String(copy.getDate()).padStart(2, '0')}`
}

function seenMap() {
  if (!library.recs || typeof library.recs !== 'object') library.recs = { seen: {} }
  if (!library.recs.seen || typeof library.recs.seen !== 'object') library.recs.seen = {}
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

const THEMES = ['热歌', '新歌', '流行', '经典老歌', '华语', '轻音乐']

async function recommendationPool() {
  const pool = []
  const jobs = THEMES.map((keyword) =>
    api
      .search({ q: keyword, type: 'music', limit: 40 })
      .then((result) => result.items || [])
      .catch(() => []),
  )
  for (const items of await Promise.all(jobs)) pool.push(...items)
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

async function toplistPool() {
  const pool = []
  try {
    await loadMeta()
    const [data] = await Promise.all([api.toplists()])
    const groups = (data.groups || []).filter((group) => group.data && group.data.length)
    const order = (meta.sources || []).map((item) => item.id)
    groups.sort((a, b) => order.indexOf(a.source) - order.indexOf(b.source))
    for (const group of groups) {
      for (const board of group.data.slice(0, 2)) {
        try {
          const detail = await api.toplistDetail({ source: group.source, id: board.id, title: board.title })
          for (const song of detail.musicList || []) {
            pool.push({ ...song, source: song.source || group.source })
          }
        } catch {
          // Skip a board that fails; the rest still fill the list.
        }
      }
      if (pool.length >= 160) break
    }
  } catch {
    // Fall through to the search-based pool.
  }
  return pool
}

export async function buildDaily(force = false) {
  const cycle = cycleKey()
  if (!force && recs.daily.cycle === cycle && recs.daily.songs.length) return recs.daily.songs
  recs.loading.daily = true
  recs.error = ''
  try {
    let songs = takeFresh(await recommendationPool(), 150, DAILY_WINDOW)
    if (songs.length < 50) {
      const merged = dedupeSongs([...songs, ...(await likedPool())])
      songs = takeFresh(merged, 150, DAILY_WINDOW)
    }
    if (!songs.length) {
      songs = dedupeSongs([...(library.favorites || []), ...(library.history || [])]).slice(0, 50)
    }
    markSeen(songs)
    songs = topUp(await verifyList(songs, 50), 50)
    markSeen(songs)
    recs.daily = { cycle, songs, generatedAt: Date.now() }
    syncToLibrary()
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
    let songs = takeFresh(await likedPool(), 60, DAILY_WINDOW)
    if (songs.length < 20) {
      songs = takeFresh(dedupeSongs([...songs, ...(await recommendationPool())]), 60, DAILY_WINDOW)
    }
    songs = topUp(await verifyList(songs, 20), 20)
    recs.liked = { songs, generatedAt: Date.now() }
    syncToLibrary()
  } catch (err) {
    recs.error = err.message || '猜你喜欢生成失败'
  } finally {
    recs.loading.liked = false
  }
  return recs.liked.songs
}

export async function nextLikedBatch(count = 20) {
  const fresh = takeFresh(dedupeSongs([...(await likedPool()), ...(await recommendationPool())]), count, DAILY_WINDOW)
  markSeen(fresh)
  recs.liked.songs.push(...fresh)
  syncToLibrary()
  return fresh
}


/** Keeps a list full even when many tracks fail the playability check. */
function topUp(songs, target) {
  if (songs.length >= target) return songs.slice(0, target)
  const seen = new Set(songs.map(songKey))
  const pools = [recs.daily.songs, recs.hot.songs, recs.liked.songs, recs.random.songs, library.favorites || []]
  for (const pool of pools) {
    for (const song of pool) {
      if (songs.length >= target) break
      const key = songKey(song)
      if (!key || seen.has(key)) continue
      seen.add(key)
      songs.push(song)
    }
  }
  return songs.slice(0, target)
}

export async function buildRandom(force = false) {
  const cycle = cycleKey()
  if (!force && recs.random.cycle === cycle && recs.random.songs.length) return recs.random.songs
  recs.loading.random = true
  recs.error = ''
  try {
    const candidates = takeFresh(await recommendationPool(), 120, RANDOM_WINDOW)
    markSeen(songs)
    const verified = topUp(await verifyList(candidates, 20), 20)
    markSeen(verified)
    recs.random = { cycle, songs: verified, generatedAt: Date.now() }
    syncToLibrary()
  } catch (err) {
    recs.error = err.message || '随便听听生成失败'
  } finally {
    recs.loading.random = false
  }
  return recs.random.songs
}

export async function nextRandomBatch(count = 20) {
  const fresh = takeFresh(await recommendationPool(), count, RANDOM_WINDOW)
  markSeen(fresh)
  recs.random.songs.push(...fresh)
  syncToLibrary()
  return fresh
}

export async function buildHot(force = false) {
  const cycle = cycleKey()
  if (!force && recs.hot.cycle === cycle && recs.hot.songs.length) return recs.hot.songs
  recs.loading.hot = true
  recs.error = ''
  try {
    let songs = dedupeSongs(await toplistPool())
    if (songs.length < 100) {
      songs = dedupeSongs([...songs, ...(await recommendationPool())])
    }
    songs = songs.slice(0, 300)
    songs = topUp(await verifyList(songs, 100), 100)
    recs.hot = { cycle, songs, generatedAt: Date.now() }
    syncToLibrary()
  } catch (err) {
    recs.error = err.message || '热门榜单生成失败'
  } finally {
    recs.loading.hot = false
  }
  return recs.hot.songs
}

async function songPlayable(song) {
  try {
    const params = {
      source: song.source,
      id: song.sourceId || song.id,
      title: song.title,
      artist: song.artist,
      album: song.album,
      duration: song.duration,
    }
    if (song.extra && Object.keys(song.extra).length) params.extra = JSON.stringify(song.extra)
    const data = await api.songUrl(params)
    return Boolean(data && data.url)
  } catch {
    return false
  }
}

async function bestAlternative(song) {
  try {
    const data = await api.alternatives({
      title: song.title,
      artist: song.artist,
      duration: song.duration,
      source: song.source,
    })
    for (const item of (data.items || []).slice(0, 3)) {
      if (await songPlayable(item)) return item
    }
  } catch {
    // No alternatives: the track is dropped.
  }
  return null
}

/** Keeps only playable tracks, swapped to another source when possible. */
async function verifyList(candidates, target, concurrency = 5) {
  const verified = []
  let cursor = 0
  async function worker() {
    while (verified.length < target && cursor < candidates.length) {
      const song = candidates[cursor]
      cursor += 1
      if (await songPlayable(song)) {
        verified.push(song)
        continue
      }
      const alternative = await bestAlternative(song)
      if (alternative) {
        // Remember the working source so playback never re-checks this track.
        const { rememberSourceOverride } = await import('./player')
        rememberSourceOverride(song, alternative)
        verified.push(alternative)
      }
    }
  }
  await Promise.all(Array.from({ length: concurrency }, worker))
  return verified.slice(0, target)
}

export function hydrateRecs() {
  const saved = library.recs
  if (!saved || typeof saved !== 'object') return
  if (saved.daily && Array.isArray(saved.daily.songs)) recs.daily = { ...recs.daily, ...saved.daily }
  if (saved.liked && Array.isArray(saved.liked.songs)) recs.liked = { ...recs.liked, ...saved.liked }
  if (saved.random && Array.isArray(saved.random.songs)) recs.random = { ...recs.random, ...saved.random }
  if (saved.hot && Array.isArray(saved.hot.songs)) recs.hot = { ...recs.hot, ...saved.hot }
}

/** Builds anything that is missing for the current 06:00 cycle. */
export async function initRecs() {
  hydrateRecs()
  await Promise.all([buildDaily(), buildRandom(), buildHot()])
  if (!recs.liked.songs.length) await buildLiked()
}

/** Mirrors the generated lists into the synced library so a refresh (or
 * another device) shows them instantly instead of rebuilding. */
function syncToLibrary() {
  library.recs = snapshotRecs()
}

export function snapshotRecs() {
  return {
    daily: { cycle: recs.daily.cycle, songs: recs.daily.songs, generatedAt: recs.daily.generatedAt },
    liked: { songs: recs.liked.songs, generatedAt: recs.liked.generatedAt },
    random: { cycle: recs.random.cycle, songs: recs.random.songs, generatedAt: recs.random.generatedAt },
    hot: { cycle: recs.hot.cycle, songs: recs.hot.songs, generatedAt: recs.hot.generatedAt },
    seen: seenMap(),
  }
}
