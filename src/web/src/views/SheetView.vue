<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, formatCount } from '../api'
import { auth } from '../auth'
import { loadMeta, sourceName } from '../meta'
import { player } from '../player'
import { isSheetFavorite, toggleSheetFavorite } from '../library'
import SongList from '../components/SongList.vue'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const error = ref('')
const detail = ref({ musicList: [], sheetItem: null })
const downloading = ref(false)
const downloadTotal = ref(0)
const downloadDone = ref(0)
const type = computed(() => String(route.query.type || 'playlist'))
const typeLabel = computed(() => (type.value === 'album' ? '专辑' : type.value === 'toplist' ? '排行榜' : '歌单'))

const metaQuery = computed(() => ({
  title: route.query.title || undefined,
  artist: route.query.artist || undefined,
  cover: route.query.cover || undefined,
  desc: route.query.desc || undefined,
  link: route.query.link || undefined,
  count: route.query.count || undefined,
  works: route.query.works || undefined,
}))

const sheet = computed(() => {
  const fetched = detail.value.sheetItem
  if (fetched && (fetched.title || fetched.artwork)) return fetched
  return {
    title: route.query.title || route.params.id,
    artwork: route.query.cover || '',
    artist: route.query.artist || '',
    description: route.query.desc || '',
    link: route.query.link || '',
    playCount: Number(route.query.count) || 0,
  }
})

const songs = computed(() => detail.value.musicList || [])

const favoriteItem = computed(() => ({
  id: String(route.params.id || ''),
  source: String(route.params.source || ''),
  type: type.value,
  title: sheet.value.title,
  artwork: sheet.value.artwork || route.query.cover || '',
  artist: sheet.value.artist || route.query.artist || '',
  description: sheet.value.description || route.query.desc || '',
  link: sheet.value.link || route.query.link || '',
  playCount: sheet.value.playCount || Number(route.query.count) || 0,
  trackCount: songs.value.length || Number(route.query.works) || 0,
}))

const favorited = computed(() => isSheetFavorite(favoriteItem.value))

function toggleFavoriteSheet() {
  toggleSheetFavorite(favoriteItem.value)
}

async function downloadAll() {
  if (!songs.value.length || downloading.value) return
  if (!auth.state.user) {
    router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  const total = songs.value.length
  if (total > 20 && !window.confirm(`将下载 ${total} 首歌曲到你的账号下载目录，是否继续？`)) return

  downloading.value = true
  downloadTotal.value = total
  downloadDone.value = 0
  let failed = 0
  const queue = songs.value.slice()

  async function worker() {
    while (queue.length) {
      const song = queue.shift()
      try {
        await api.download({ song, withCover: false, withLyrics: true })
      } catch {
        failed += 1
      } finally {
        downloadDone.value += 1
      }
    }
  }

  try {
    await Promise.all([worker(), worker()])
  } finally {
    downloading.value = false
  }
  window.alert(`下载完成：成功 ${total - failed} 首${failed ? `，失败 ${failed} 首` : ''}`)
}

async function load() {
  loading.value = true
  error.value = ''
  const params = {
    source: route.params.source,
    id: route.params.id,
    ...metaQuery.value,
    limit: 500,
  }
  try {
    if (type.value === 'toplist') {
      detail.value = await api.toplistDetail(params)
    } else if (type.value === 'album') {
      detail.value = await api.album(params)
    } else {
      detail.value = await api.playlist(params)
    }
  } catch (err) {
    error.value = err.message || '加载失败'
    detail.value = { musicList: [], sheetItem: null }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadMeta()
  load()
})

watch(() => route.fullPath, load)
</script>

<template>
  <div class="sheet-view">
    <header class="detail-head">
      <div class="detail-cover">
        <img v-if="sheet.artwork" :src="sheet.artwork" :alt="sheet.title" />
        <i v-else class="ri-music-2-line"></i>
      </div>
      <div class="detail-info">
        <span class="badge">{{ type === 'album' ? '专辑' : type === 'toplist' ? '排行榜' : '歌单' }} · {{ sourceName(route.params.source) }}</span>
        <h1>{{ sheet.title }}</h1>
        <p class="detail-artist" v-if="sheet.artist">{{ sheet.artist }}</p>
        <p class="detail-desc" v-if="sheet.description">{{ sheet.description }}</p>
        <div class="detail-stats">
          <span v-if="songs.length"><i class="ri-music-2-line"></i> {{ songs.length }} 首</span>
          <span v-if="sheet.playCount"><i class="ri-headphone-line"></i> {{ formatCount(sheet.playCount) }}</span>
        </div>
        <div class="detail-actions">
          <button class="primary-button" :disabled="!songs.length" @click="player.playSongs(songs, 0)">
            <i class="ri-play-fill"></i> 播放全部
          </button>
          <button class="ghost-button" :disabled="!songs.length" @click="player.addToQueue(songs)">
            <i class="ri-add-line"></i> 加入队列
          </button>
          <button
            class="ghost-button favorite-sheet-button"
            :class="{ active: favorited }"
            @click="toggleFavoriteSheet"
          >
            <i :class="favorited ? 'ri-bookmark-fill' : 'ri-bookmark-line'"></i>
            {{ favorited ? '已收藏' : `收藏${typeLabel}` }}
          </button>

          <button
            class="ghost-button download-sheet-button"
            :disabled="!songs.length || downloading"
            @click="downloadAll"
          >
            <i :class="downloading ? 'ri-loader-4-line spin' : 'ri-download-cloud-line'"></i>
            {{ downloading ? `下载中 ${downloadDone}/${downloadTotal}` : '下载歌单' }}
          </button>
        </div>
      </div>
    </header>

    <div v-if="error" class="notice error">
      <i class="ri-error-warning-line"></i>{{ error }}
      <button class="text-button" @click="load">重试</button>
    </div>

    <SongList
      :songs="songs"
      :loading="loading"
      show-album
      show-download
      :empty-text="error ? '歌曲加载失败' : '这个歌单还没有歌曲'"
      @play="player.playSongs(songs, $event)"
      @add="player.addToQueue([$event])"
    />
  </div>
</template>
