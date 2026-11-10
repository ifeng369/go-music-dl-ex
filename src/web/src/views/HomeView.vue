<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '../api'
import { loadMeta, meta, sourceName } from '../meta'
import SectionHeader from '../components/SectionHeader.vue'
import SheetGrid from '../components/SheetGrid.vue'

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

onMounted(load)
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
          <a href="#categories" class="primary-button"><i class="ri-play-circle-line"></i> 开始探索</a>
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

    <section id="categories" class="section home-categories">
      <SectionHeader title="分类歌单" subtitle="按心情与场景发现音乐，可切换不同音源" />
      <div v-if="categorySources.length" class="category-source-tabs">
        <button
          v-for="source in categorySources"
          :key="source.id"
          class="category-source-tab"
          :class="{ active: categorySource === source.id }"
          @click="selectCategory(source.id)"
        >
          {{ sourceName(source.id) }}
        </button>
      </div>
      <div v-if="loading || categoryLoading" class="chip-row">
        <span v-for="i in 18" :key="i" class="chip skeleton-chip"></span>
      </div>
      <div v-else-if="categoryError" class="notice error compact">
        <i class="ri-error-warning-line"></i>
        {{ categoryError }}
        <button class="text-button" @click="selectCategory(categorySource)">重试</button>
      </div>
      <div v-else-if="categoryItems.length" class="chip-row">
        <router-link
          v-for="category in categoryItems.slice(0, 48)"
          :key="category.source + category.id + category.name"
          :to="{ name: 'category', params: { source: category.source }, query: { id: category.id || '', name: category.name } }"
          class="chip"
          :title="sourceName(category.source)"
        >
          <i class="ri-play-list-2-line"></i>
          {{ category.name }}
        </router-link>
      </div>
      <div v-else class="empty-inline">该音源暂无可用分类</div>
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
          :more-text="`查看全部 ${group.data.length} 个`"
          :to="{ name: 'search', query: { q: sourceName(group.source), type: 'sheet', source: group.source } }"
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
