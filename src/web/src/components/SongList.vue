<script setup>
import { reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, formatBitrate, formatDuration, formatSize } from '../api'
import { sourceName } from '../meta'
import { auth } from '../auth'
import { library, addSongToPlaylist, createPlaylist, isFavorite, toggleFavorite } from '../library'

defineProps({
  songs: { type: Array, default: () => [] },
  showAlbum: { type: Boolean, default: false },
  showSource: { type: Boolean, default: true },
  showFileMeta: { type: Boolean, default: false },
  showFavorite: { type: Boolean, default: true },
  showPlaylist: { type: Boolean, default: true },
  showRemove: { type: Boolean, default: false },
  showDownload: { type: Boolean, default: false },
  selectable: { type: Boolean, default: false },
  selected: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  compact: { type: Boolean, default: false },
  emptyText: { type: String, default: '暂无歌曲' },
})

const emit = defineEmits(['play', 'add', 'remove', 'select'])

const route = useRoute()
const router = useRouter()
const menu = reactive({ open: false, x: 0, y: 0, song: null })
const downloadState = reactive({})

function favorite(song) {
  toggleFavorite(song)
}

async function downloadSong(song) {
  if (!auth.state.user) {
    router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  if (downloadState[song.id] === 'loading') return
  downloadState[song.id] = 'loading'
  try {
    await api.download({ song, withCover: false, withLyrics: true })
    downloadState[song.id] = 'done'
    window.setTimeout(() => { delete downloadState[song.id] }, 2500)
  } catch (err) {
    downloadState[song.id] = 'error'
    window.alert(`下载失败：${err.message || '请稍后重试'}`)
    window.setTimeout(() => { delete downloadState[song.id] }, 3000)
  }
}

function openMenu(song, event) {
  const rect = event.currentTarget.getBoundingClientRect()
  const width = 224
  const height = 288
  menu.x = Math.max(8, Math.min(rect.right - width, window.innerWidth - width - 8))
  menu.y = rect.bottom + 6 + height > window.innerHeight
    ? Math.max(8, rect.top - height - 6)
    : rect.bottom + 6
  menu.song = song
  menu.open = true
}

function closeMenu() {
  menu.open = false
  menu.song = null
}

function addToPlaylist(playlist) {
  if (menu.song) addSongToPlaylist(playlist.id, menu.song)
  closeMenu()
}

function createAndAdd() {
  const name = window.prompt('新建歌单名称')
  if (!name || !String(name).trim()) return
  const playlist = createPlaylist(name)
  if (menu.song) addSongToPlaylist(playlist.id, menu.song)
  closeMenu()
}
</script>

<template>
  <div v-if="loading" class="song-list">
    <div v-for="i in 10" :key="i" class="song-row skeleton-row">
      <span class="skeleton-line"></span>
    </div>
  </div>
  <div v-else-if="songs.length" class="song-list">
    <div
      v-for="(song, index) in songs"
      :key="song.id || index"
      class="song-row"
      @click="selectable ? emit('select', song) : emit('play', index)"
    >
      <span class="song-index" :class="{ selecting: selectable }">
        <label v-if="selectable" class="song-check" @click.stop>
          <input type="checkbox" :checked="selected.includes(song.id)" @change="emit('select', song)" />
          <span class="song-check-box"><i class="ri-check-line"></i></span>
        </label>
        <template v-else>
          <span class="index-num">{{ String(index + 1).padStart(2, '0') }}</span>
          <button class="row-play" title="播放" @click.stop="emit('play', index)">
            <i class="ri-play-fill"></i>
          </button>
        </template>
      </span>
      <span class="song-main">
        <img
          v-if="song.artwork || song.cover"
          class="song-thumb"
          :src="song.artwork || song.cover"
          :alt="song.title"
          loading="lazy"
        />
        <span v-else class="song-thumb song-thumb-fallback">
          <i :class="song.source === 'local' ? 'ri-file-music-line' : 'ri-music-2-line'"></i>
        </span>
        <span class="song-text">
          <span class="song-title" :title="song.title">{{ song.title }}</span>
          <span class="song-artist" :title="song.artist">{{ song.artist || '未知歌手' }}</span>
          <span v-if="song.source === 'local' && !showFileMeta" class="song-local-meta">{{ formatBitrate(song.bitrate) }} · {{ formatSize(song.size) }} · {{ formatDuration(song.duration) }}</span>
        </span>
      </span>
      <span v-if="showAlbum" class="song-album" :title="song.album">{{ song.album || '—' }}</span>
      <span v-if="showSource" class="song-source">{{ sourceName(song.source) }}</span>
      <span v-if="song.isVip" class="vip-badge">VIP</span>
      <span v-if="showFileMeta" class="song-file-meta song-bitrate">{{ formatBitrate(song.bitrate) }}</span>
      <span v-if="showFileMeta" class="song-file-meta song-size">{{ formatSize(song.size) }}</span>
      <span class="song-duration">{{ formatDuration(song.duration) }}</span>
      <div class="row-actions">
        <button
          v-if="showFavorite"
          class="row-action"
          :class="{ active: isFavorite(song) }"
          :title="isFavorite(song) ? '取消收藏' : '收藏'"
          @click.stop="favorite(song)"
        >
          <i :class="isFavorite(song) ? 'ri-heart-3-fill' : 'ri-heart-3-line'"></i>
        </button>
        <button
          v-if="showPlaylist"
          class="row-action"
          title="加入歌单"
          @click.stop="openMenu(song, $event)"
        >
          <i class="ri-play-list-add-line"></i>
        </button>
        <button
          v-if="showRemove"
          class="row-action danger"
          title="移除"
          @click.stop="emit('remove', index)"
        >
          <i class="ri-delete-bin-6-line"></i>
        </button>
        <button class="row-action" title="加入播放队列" @click.stop="emit('add', song)">
          <i class="ri-add-line"></i>
        </button>
        <button
          v-if="showDownload"
          class="row-action"
          :class="{ active: downloadState[song.id] === 'done', danger: downloadState[song.id] === 'error' }"
          :disabled="downloadState[song.id] === 'loading'"
          :title="downloadState[song.id] === 'loading' ? '下载中…' : downloadState[song.id] === 'done' ? '已下载' : downloadState[song.id] === 'error' ? '下载失败' : '下载'"
          @click.stop="downloadSong(song)"
        >
          <i :class="downloadState[song.id] === 'loading' ? 'ri-loader-4-line spin' : downloadState[song.id] === 'done' ? 'ri-check-line' : downloadState[song.id] === 'error' ? 'ri-error-warning-line' : 'ri-download-line'"></i>
        </button>
      </div>
    </div>
  </div>
  <div v-else class="empty">{{ emptyText }}</div>

  <Teleport to="body">
    <div v-if="menu.open" class="song-menu-backdrop" @click="closeMenu"></div>
    <div v-if="menu.open" class="song-menu" :style="{ left: `${menu.x}px`, top: `${menu.y}px` }">
      <div class="song-menu-title">加入歌单</div>
      <div class="song-menu-list">
        <button
          v-for="playlist in library.playlists"
          :key="playlist.id"
          class="song-menu-item"
          @click="addToPlaylist(playlist)"
        >
          <i class="ri-play-list-2-line"></i>
          <span>{{ playlist.name }}</span>
          <small>{{ playlist.songs.length }}</small>
        </button>
        <div v-if="!library.playlists.length" class="song-menu-empty">还没有自建歌单</div>
      </div>
      <button class="song-menu-item create" @click="createAndAdd">
        <i class="ri-add-line"></i>
        <span>新建歌单</span>
      </button>
    </div>
  </Teleport>
</template>
