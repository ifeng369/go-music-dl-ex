<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'
import { loadMeta, sourceName } from '../meta'
import { player } from '../player'
import SongList from '../components/SongList.vue'

const route = useRoute()
const loading = ref(true)
const error = ref('')
const songs = ref([])
const artistName = computed(() => decodeURIComponent(String(route.params.name || '')))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.artistWorks({
      name: artistName.value,
      source: route.params.source,
      limit: 100,
    })
    songs.value = data.items || []
  } catch (err) {
    error.value = err.message || '加载失败'
    songs.value = []
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
  <div class="artist-view">
    <header class="detail-head artist-head">
      <div class="detail-cover round">
        <img v-if="songs[0] && songs[0].artwork" :src="songs[0].artwork" :alt="artistName" />
        <i v-else class="ri-user-voice-line"></i>
      </div>
      <div class="detail-info">
        <span class="badge">歌手 · {{ sourceName(route.params.source) }}</span>
        <h1>{{ artistName }}</h1>
        <p class="detail-desc">该平台下与歌手相关的热门作品</p>
        <div class="detail-stats">
          <span v-if="songs.length"><i class="ri-music-2-line"></i> {{ songs.length }} 首相关歌曲</span>
        </div>
        <div class="detail-actions">
          <button class="primary-button" :disabled="!songs.length" @click="player.playSongs(songs, 0)">
            <i class="ri-play-fill"></i> 播放全部
          </button>
          <button class="ghost-button" :disabled="!songs.length" @click="player.addToQueue(songs)">
            <i class="ri-add-line"></i> 加入队列
          </button>
        </div>
      </div>
    </header>

    <div v-if="error" class="notice error"><i class="ri-error-warning-line"></i>{{ error }}</div>
    <SongList
      :songs="songs"
      :loading="loading"
      show-album
      empty-text="没有找到该歌手的作品"
      @play="player.playSongs(songs, $event)"
      @add="player.addToQueue([$event])"
    />
  </div>
</template>
