<script setup>
import { computed } from 'vue'
import SongList from '../components/SongList.vue'
import { clearHistory, library } from '../library'
import { player } from '../player'

const songs = computed(() => library.history)

function playAll() {
  if (songs.value.length) player.playSongs(songs.value, 0)
}

function clearAll() {
  if (!songs.value.length) return
  if (!window.confirm('确定清空播放记录吗？')) return
  clearHistory()
}
</script>

<template>
  <div class="library-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">我的音乐</span>
        <h1>播放记录</h1>
        <p>最近播放过的歌曲会按时间倒序保存在这里。</p>
      </div>
      <div class="library-actions">
        <button class="ghost-button" :disabled="!songs.length" @click="clearAll">
          <i class="ri-delete-bin-6-line"></i> 清空记录
        </button>
        <button class="primary-button" :disabled="!songs.length" @click="playAll">
          <i class="ri-play-fill"></i> 播放全部
        </button>
      </div>
    </header>

    <div class="library-summary">
      <span><i class="ri-history-line"></i> {{ songs.length }} 条记录</span>
      <span class="library-tip">最多保留最近 300 首播放记录</span>
    </div>

    <SongList
      :songs="songs"
      show-album
      empty-text="还没有播放记录，播放一首歌曲后会自动记录"
      @play="player.playSongFrom(songs, $event)"
      @add="player.addToQueue([$event])"
    />
  </div>
</template>
