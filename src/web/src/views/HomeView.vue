<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'
import { loadMeta, meta, sourceName } from '../meta'
import SectionHeader from '../components/SectionHeader.vue'
import SheetGrid from '../components/SheetGrid.vue'
import SongList from '../components/SongList.vue'
import { player } from '../player'
import { buildDaily, buildLiked, buildRandom, recs } from '../recs'
import { library } from '../library'

const loading = ref(true)
const error = ref('')
const data = ref({ recommend: [], recommendGroups: [], categories: [] })
const categorySource = ref('netease')
const categoryItems = ref([])
const categoryLoading = ref(false)
const categoryError = ref('')
const categoryCache = new Map()
const cookieState = ref({})
let categoryToken = 0

const categorySources = computed(() => meta.sources.filter((source) => source.categories))
const sourceStates = computed(() => {
  const result = {}
  for (const source of meta.sources) {
    const cookie = cookieState.value[source.id]
    if (cookie && cookie.configured) {
      result[source.id] = { text: '已登录', cls: 'logged' }
    } else {
      result[source.id] = {
        text: source.requiresLogin ? '需登录' : '免登录',
        cls: source.requiresLogin ? 'required' : '',
      }
    }
  }
  return result
})
const heroCover = computed(() => {
  const first = (data.value.recommend || [])[0]
  return first ? first.artwork : ''
})

const allSheets = computed(() => {
  const list = []
  const seen = new Set()
  const push = (item, fallbackSource) => {
    if (!item || !item.id) return
    const source = item.source || fallbackSource || ''
    const key = `${source}:${item.id}`
    if (seen.has(key)) return
    seen.add(key)
    list.push({ ...item, source })
  }
  for (const group of data.value.recommendGroups || []) {
    for (const item of group.data || []) push(item, group.source)
  }
  for (const item of data.value.recommend || []) push(item, item.source)
  return list
})

function daySeed() {
  const now = new Date()
  return Number(`${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}`)
}

function pickSheets(pool, count, seed) {
  if (!pool.length) return []
  const total = pool.length
  let state = seed % 2147483647
  if (state <= 0) state += 2147483646
  const picked = []
  const used = new Set()
  while (picked.length < Math.min(count, total) && used.size < total) {
    state = (state * 16807) % 2147483647
    const index = state % total
    if (used.has(index)) continue
    used.add(index)
    picked.push(pool[index])
  }
  return picked
}

const dailySheets = computed(() => pickSheets(allSheets.value, 8, daySeed()))
const randomSheets = computed(() => pickSheets(allSheets.value, 8, daySeed() * 7 + 17))
const likedSheets = computed(() => (library.sheetFavorites || []).slice(0, 12))

function cacheCategories(items) {
  for (const item of items || []) {
    if (!item.source) continue
    if (!categoryCache.has(item.source)) categoryCache.set(item.source, [])
    categoryCache.get(item.source).push(item)
  }
}

async function selectCategory(source) {
  if (!source) return
  categorySource.value = source
  categoryError.value = ''
  const token = ++categoryToken
  if (categoryCache.has(source)) {
    categoryItems.value = categoryCache.get(source)
    categoryLoading.value = false
    return
  }
  categoryLoading.value = true
  categoryItems.value = []
  try {
    const result = await api.categories({ source })
    const items = result.items || []
    categoryCache.set(source, items)
    if (token === categoryToken) categoryItems.value = items
  } catch (err) {
    if (token === categoryToken) {
      categoryItems.value = []
      categoryError.value = err.message || '该音源分类加载失败，可能需要在设置中登录'
    }
  } finally {
    if (token === categoryToken) categoryLoading.value = false
  }
}

async function load() {
  loading.value = true
  error.value = ''
  categoryCache.clear()
  try {
    const [home, , cookieData] = await Promise.all([
      api.home(),
      loadMeta(),
      api.cookies().catch(() => ({ items: {} })),
    ])
    cookieState.value = (cookieData && cookieData.items) || {}
    data.value = {
      recommend: home.recommend || [],
      recommendGroups: home.recommendGroups || [],
      categories: home.categories || [],
    }
    cacheCategories(home.categories)
    const available = categorySources.value.map((source) => source.id)
    const initial = available.includes(categorySource.value)
      ? categorySource.value
      : (available[0] || 'netease')
    await selectCategory(initial)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

const dailySubtitle = computed(() => {
  const stamp = recs.daily.generatedAt
  if (!stamp) return '每天 6:00 根据各平台推荐生成 50 首'
  const time = new Date(stamp).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  return `每天 6:00 更新 · 上次 ${time}`
})

function playRec(songs, mode, index = 0) {
  if (!songs.length) return
  player.playSongs(songs, index)
  // Set after playSongs, which resets the flag for normal lists.
  player.state.randomMode = mode === 'random'
}

async function refreshLiked() {
  await buildLiked(true)
}

onMounted(load)
onMounted(() => {
  buildDaily()
  buildLiked()
  buildRandom()
})
</script>

<template>
  <div class="home-view">
    <section class="hero" :style="heroCover ? { '--hero-cover': `url(${heroCover})` } : {}">
      <div class="hero-glow"></div>
      <div class="hero-copy">
        <span class="hero-eyebrow"><i class="ri-sparkling-2-line"></i> 多平台音乐聚合</span>
        <h1>一个入口，听见所有平台</h1>
        <p>网易云、QQ音乐、酷狗、酷我、咪咕、汽水、B站……<br />搜索、歌单、排行榜与歌词，一处搞定。</p>
        <div class="hero-actions">
          <router-link :to="{ name: 'recommend' }" class="primary-button"><i class="ri-play-circle-line"></i> 开始探索</router-link>
          <span class="hero-stat">
            <strong>{{ meta.sources.length || '—' }}</strong>
            <span>个音乐源在线</span>
          </span>
        </div>
      </div>
      <div class="hero-art">
        <div class="vinyl"><div class="vinyl-center"><i class="ri-music-2-fill"></i></div></div>
      </div>
    </section>

    <div v-if="error" class="notice error">
      <i class="ri-error-warning-line"></i>
      {{ error }}
      <button class="text-button" @click="load">重试</button>
    </div>

        <section class="section home-daily">
      <SectionHeader title="每日推荐" :subtitle="dailySubtitle" />
      <div class="rec-actions">
        <button class="primary-button" :disabled="!recs.daily.songs.length" @click="playRec(recs.daily.songs, 'daily', 0)">
          <i class="ri-play-fill"></i> 播放全部
        </button>
        <span class="rec-count">{{ recs.daily.songs.length }} 首 · 每天 6:00 更新</span>
      </div>
      <SongList
        :songs="recs.daily.songs.slice(0, 8)"
        compact
        :loading="recs.loading.daily && !recs.daily.songs.length"
        empty-text="正在为你生成每日推荐…"
        @play="playRec(recs.daily.songs, 'daily', $event)"
      />
    </section>

    <section class="section">
      <SectionHeader title="你喜欢听" subtitle="根据本地歌曲、我的歌单和收藏音乐的偏好推荐">
        <template #actions>
          <button class="text-button" :disabled="recs.loading.liked" @click="refreshLiked">
            <i class="ri-refresh-line"></i> 换一批
          </button>
        </template>
      </SectionHeader>
      <div class="rec-actions">
        <button class="primary-button" :disabled="!recs.liked.songs.length" @click="playRec(recs.liked.songs, 'liked', 0)">
          <i class="ri-play-fill"></i> 播放全部
        </button>
        <span class="rec-count">{{ recs.liked.songs.length }} 首 · 每次点击重新生成</span>
      </div>
      <SongList
        :songs="recs.liked.songs.slice(0, 8)"
        compact
        :loading="recs.loading.liked && !recs.liked.songs.length"
        empty-text="多听几首歌，这里就会更懂你"
        @play="playRec(recs.liked.songs, 'liked', $event)"
      />
    </section>

    <section class="section">
      <SectionHeader title="随便听听" subtitle="随机 20 首，快播完时自动补充" />
      <div class="rec-actions">
        <button class="primary-button" :disabled="!recs.random.songs.length" @click="playRec(recs.random.songs, 'random', 0)">
          <i class="ri-play-fill"></i> 播放全部
        </button>
        <span class="rec-count">{{ recs.random.songs.length }} 首 · 剩余 2 首自动续 20 首</span>
      </div>
      <SongList
        :songs="recs.random.songs.slice(0, 8)"
        compact
        :loading="recs.loading.random && !recs.random.songs.length"
        empty-text="正在随机挑选…"
        @play="playRec(recs.random.songs, 'random', $event)"
      />
    </section>

    <template v-if="loading">
      <section class="section">
        <div class="section-header">
          <div class="section-title"><h2>推荐歌单</h2></div>
        </div>
        <SheetGrid :items="[]" loading />
      </section>
    </template>
    <template v-else>
      <section
        v-for="group in data.recommendGroups"
        :key="group.source"
        class="section recommend-group"
      >
        <SectionHeader
          :title="`${sourceName(group.source)} 推荐`"
          :subtitle="`来自${sourceName(group.source)}的热门歌单`"
          :more-text="'全部 ' + allSheets.length + ' 个'"
          :to="{ name: 'recommend' }"
        />
        <SheetGrid :items="group.data.slice(0, 12)" empty-text="暂无推荐歌单" />
      </section>

      <div v-if="!data.recommendGroups.length" class="empty large">
        <i class="ri-play-list-2-line"></i>
        <p>暂无推荐歌单，请稍后重试</p>
      </div>
    </template>

    <section class="section">
      <SectionHeader title="已接入音源" subtitle="在设置中可配置各平台 Cookie" to="/settings" more-text="管理音源" />
      <div class="source-grid">
        <div v-for="source in meta.sources" :key="source.id" class="source-card">
          <span class="source-avatar">{{ sourceName(source.id).slice(0, 1) }}</span>
          <div class="source-info">
            <strong>{{ sourceName(source.id) }}</strong>
            <small>{{ [source.search && '搜索', source.sheet && '歌单', source.toplist && '榜单', source.album && '专辑'].filter(Boolean).join(' · ') || '浏览' }}</small>
          </div>
          <span class="source-state" :class="sourceStates[source.id] ? sourceStates[source.id].cls : ''">
            {{ sourceStates[source.id] ? sourceStates[source.id].text : (source.requiresLogin ? '需登录' : '免登录') }}
          </span>
        </div>
      </div>
    </section>
  </div>
</template>
