<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { meta, loadMeta, sourceName } from '../meta'
import SongList from '../components/SongList.vue'
import SheetGrid from '../components/SheetGrid.vue'
import { player } from '../player'

const route = useRoute()
const router = useRouter()
const tabs = [
  { key: 'music', label: '歌曲', icon: 'ri-music-2-line' },
  { key: 'sheet', label: '歌单', icon: 'ri-play-list-line' },
  { key: 'album', label: '专辑', icon: 'ri-album-line' },
  { key: 'artist', label: '歌手', icon: 'ri-user-voice-line' },
]
const activeTab = ref(route.query.type || 'music')
const source = ref(route.query.source || '')
const loading = ref(false)
const error = ref('')
const result = ref({ items: [], errors: {} })

const keyword = computed(() => String(route.query.q || '').trim())
const count = computed(() => (result.value.items || []).length)
const sourceOptions = computed(() => {
  const capability = activeTab.value === 'sheet' ? 'sheet' : activeTab.value === 'album' ? 'album' : 'search'
  return meta.sources.filter((item) => (activeTab.value === 'artist' ? item.search : item[capability]))
})

async function load() {
  if (!keyword.value) {
    result.value = { items: [], errors: {} }
    return
  }
  loading.value = true
  error.value = ''
  try {
    result.value = await api.search({
      q: keyword.value,
      type: activeTab.value,
      source: source.value || undefined,
      limit: 30,
    })
  } catch (err) {
    error.value = err.message || '搜索失败'
    result.value = { items: [], errors: {} }
  } finally {
    loading.value = false
  }
}

function changeTab(key) {
  activeTab.value = key
  router.replace({ name: 'search', query: { ...route.query, type: key } })
  load()
}

function changeSource() {
  router.replace({ name: 'search', query: { ...route.query, source: source.value || undefined } })
  load()
}

function playSong(index) {
  player.playSongs(result.value.items, index)
}

onMounted(async () => {
  await loadMeta()
  load()
})

watch(
  () => route.query.q,
  () => load(),
)
</script>

<template>
  <div class="search-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">搜索</span>
        <h1>{{ keyword ? `“${keyword}”` : '搜索音乐' }}</h1>
        <p v-if="keyword && !loading">共找到 {{ count }} 条结果<span v-if="!result.isEnd"> · 已展示第一页</span></p>
      </div>
      <button v-if="activeTab === 'music' && count" class="primary-button" @click="player.playSongs(result.items, 0)">
        <i class="ri-play-fill"></i> 播放全部
      </button>
    </header>

    <div class="toolbar">
      <div class="tabs">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="tab"
          :class="{ active: activeTab === tab.key }"
          @click="changeTab(tab.key)"
        >
          <i :class="tab.icon"></i>{{ tab.label }}
        </button>
      </div>
      <div class="filters">
        <label class="select-label">
          <i class="ri-filter-3-line"></i>
          <select v-model="source" @change="changeSource">
            <option value="">全部音源</option>
            <option v-for="option in sourceOptions" :key="option.id" :value="option.id">
              {{ sourceName(option.id) }}
            </option>
          </select>
        </label>
      </div>
    </div>

    <div v-if="error" class="notice error"><i class="ri-error-warning-line"></i>{{ error }}</div>
    <div v-else-if="!keyword" class="empty large">
      <i class="ri-search-eye-line"></i>
      <p>在上方搜索框输入歌曲、歌单、专辑或歌手</p>
    </div>

    <template v-else>
      <SongList
        v-if="activeTab === 'music'"
        :songs="result.items || []"
        :loading="loading"
        show-album
        empty-text="没有找到歌曲，试试更换关键词或音源"
        @play="playSong"
        @add="player.addToQueue([$event])"
      />
      <SheetGrid
        v-else-if="activeTab === 'sheet' || activeTab === 'album'"
        :items="result.items || []"
        :loading="loading"
        :empty-text="activeTab === 'sheet' ? '没有找到歌单' : '没有找到专辑'"
      />
      <div v-else-if="activeTab === 'artist'" class="artist-grid">
        <router-link
          v-for="artist in result.items || []"
          :key="artist.id"
          :to="{ name: 'artist', params: { source: artist.source, name: artist.sourceId || artist.name } }"
          class="artist-card"
        >
          <div class="artist-avatar">
            <img v-if="artist.avatar" :src="artist.avatar" :alt="artist.name" loading="lazy" />
            <i v-else class="ri-user-voice-line"></i>
          </div>
          <strong>{{ artist.name }}</strong>
          <small>{{ sourceName(artist.source) }}<template v-if="artist.worksNum"> · {{ artist.worksNum }} 首</template></small>
        </router-link>
        <div v-if="!loading && !(result.items || []).length" class="empty">没有找到歌手</div>
      </div>
      <div v-if="Object.keys(result.errors || {}).length" class="warnings">
        <span v-for="(message, key) in result.errors" :key="key">{{ sourceName(key) }}: {{ message }}</span>
      </div>
    </template>
  </div>
</template>
