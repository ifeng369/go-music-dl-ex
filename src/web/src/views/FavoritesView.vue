<script setup>
import { computed } from 'vue'
import SongList from '../components/SongList.vue'
import { clearFavorites, library } from '../library'
import { player } from '../player'

const songs = computed(() => library.favorites)

function playAll() {
  if (songs.value.length) player.playSongs(songs.value, 0)
}

function clearAll() {
  if (!songs.value.length) return
  if (!window.confirm('确定清空所有收藏歌曲吗？')) return
  clearFavorites()
}
</script>

<template>
  <div class="library-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">我的音乐</span>
        <h1>收藏音乐</h1>
        <p>收藏的歌曲会保存在当前浏览器，点击歌曲右侧的爱心即可取消收藏。</p>
      </div>
      <div class="library-actions">
        <button class="ghost-button" :disabled="!songs.length" @click="clearAll">
          <i class="ri-delete-bin-6-line"></i> 清空收藏
        </button>
        <button class="primary-button" :disabled="!songs.length" @click="playAll">
          <i class="ri-play-fill"></i> 播放全部
        </button>
      </div>
    </header>

    <div class="library-summary">
      <span><i class="ri-heart-3-line"></i> {{ songs.length }} 首收藏</span>
      <span class="library-tip">在播放栏或歌曲列表点击爱心即可收藏</span>
    </div>

    <SongList
      :songs="songs"
      show-album
      empty-text="还没有收藏歌曲，去发现一首喜欢的音乐吧"
      @play="player.playSongs(songs, $event)"
      @add="player.addToQueue([$event])"
    />
  </div>
</template>
