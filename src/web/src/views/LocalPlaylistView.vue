<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SongList from '../components/SongList.vue'
import { deletePlaylist, playlistByID, removeSongFromPlaylist, renamePlaylist } from '../library'
import { player } from '../player'

const route = useRoute()
const router = useRouter()
const playlist = computed(() => playlistByID(String(route.params.id || '')))
const songs = computed(() => (playlist.value ? playlist.value.songs : []))
const cover = computed(() => {
  const song = songs.value.find((item) => item.artwork)
  return song ? song.artwork : ''
})

function playAll() {
  if (songs.value.length) player.playSongs(songs.value, 0)
}

function rename() {
  if (!playlist.value) return
  const name = window.prompt('修改歌单名称', playlist.value.name)
  if (name && String(name).trim()) renamePlaylist(playlist.value.id, name)
}

function removePlaylist() {
  if (!playlist.value) return
  if (!window.confirm(`确定删除歌单「${playlist.value.name}」吗？`)) return
  deletePlaylist(playlist.value.id)
  router.push({ name: 'my-playlists' })
}

function removeSong(index) {
  if (playlist.value) removeSongFromPlaylist(playlist.value.id, index)
}
</script>

<template>
  <div v-if="playlist" class="library-view">
    <header class="detail-head">
      <div class="detail-cover">
        <img v-if="cover" :src="cover" :alt="playlist.name" />
        <i v-else class="ri-play-list-2-line"></i>
      </div>
      <div class="detail-info">
        <span class="badge">我的歌单</span>
        <h1>{{ playlist.name }}</h1>
        <div class="detail-stats">
          <span><i class="ri-music-2-line"></i> {{ songs.length }} 首歌曲</span>
          <span><i class="ri-time-line"></i> 更新于 {{ new Date(playlist.updatedAt).toLocaleDateString('zh-CN') }}</span>
        </div>
        <div class="detail-actions">
          <button class="primary-button" :disabled="!songs.length" @click="playAll">
            <i class="ri-play-fill"></i> 播放全部
          </button>
          <button class="ghost-button" :disabled="!songs.length" @click="player.addToQueue(songs)">
            <i class="ri-add-line"></i> 加入队列
          </button>
          <button class="ghost-button" @click="rename">
            <i class="ri-edit-line"></i> 重命名
          </button>
          <button class="ghost-button danger-text" @click="removePlaylist">
            <i class="ri-delete-bin-6-line"></i> 删除歌单
          </button>
        </div>
      </div>
    </header>

    <SongList
      :songs="songs"
      show-album
      show-remove
      empty-text="歌单还是空的，去搜索歌曲并点击列表中的歌单图标添加"
      @play="player.playSongFrom(songs, $event)"
      @add="player.addToQueue([$event])"
      @remove="removeSong"
    />
  </div>

  <div v-else class="empty large">
    <i class="ri-play-list-2-line"></i>
    <p>歌单不存在或已被删除</p>
    <router-link to="/my" class="text-button">返回我的歌单</router-link>
  </div>
</template>
