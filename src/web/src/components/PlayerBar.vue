<script setup>
import { computed } from 'vue'
import { Slider as TSlider } from 'tdesign-vue-next'
import 'tdesign-vue-next/es/slider/style/index.css'
import { formatDuration } from '../api'
import { player } from '../player'
import { isFavorite, toggleFavorite } from '../library'

const state = player.state
const current = player.current
const progress = player.progress

const modeIcon = computed(() => {
  if (state.mode === 'one') return 'ri-repeat-one-line'
  if (state.mode === 'shuffle') return 'ri-shuffle-line'
  return 'ri-repeat-line'
})

const modeTitle = computed(() => {
  if (state.mode === 'one') return '单曲循环'
  if (state.mode === 'shuffle') return '随机播放'
  return '列表循环'
})

const liked = computed(() => Boolean(current.value && isFavorite(current.value)))

const volume = computed(() => (state.muted ? 0 : state.volume))

function onSeek(value) {
  player.seek(Number(value))
}

function onVolume(value) {
  player.setVolume(Number(value))
}

function openNowPlaying(event) {
  if (!current.value) return
  if (event && event.target && event.target.closest('button, input, a, .range, .t-slider')) return
  state.showNowPlaying = true
}

function openNowPlayingForce() {
  if (current.value) state.showNowPlaying = true
}

function toggleLike() {
  if (current.value) toggleFavorite(current.value)
}
</script>

<template>
  <footer class="player-bar" :class="{ clickable: Boolean(current) }" @click="openNowPlaying">
    <div class="player-track">
      <button class="player-cover" :class="{ spinning: state.playing }" title="打开当前播放" @click="openNowPlayingForce">
        <img v-if="current && current.artwork" :src="current.artwork" :alt="current.title" />
        <i v-else class="ri-music-2-line"></i>
        <span v-if="state.loading" class="cover-loading"><i class="ri-loader-4-line"></i></span>
      </button>
      <div class="player-info">
        <div class="player-title" :title="current ? current.title : ''">
          {{ current ? current.title : '还没有播放音乐' }}
        </div>
        <div class="player-artist">
          <span v-if="state.error" class="player-error">{{ state.error }}</span>
          <template v-else>{{ current ? current.artist || '未知歌手' : '从发现页挑一首歌开始' }}</template>
        </div>
      </div>
      <button
        v-if="current"
        class="icon-button like-button"
        :class="{ active: liked }"
        :title="liked ? '取消收藏' : '收藏'"
        @click.stop="toggleLike"
      >
        <i :class="liked ? 'ri-heart-3-fill' : 'ri-heart-3-line'"></i>
      </button>
    </div>

    <div class="player-center">
      <div class="player-controls">
        <button
          class="icon-button mode-button"
          :title="modeTitle"
          :disabled="!current"
          @click.stop="player.cycleMode()"
        >
          <i :class="modeIcon"></i>
        </button>
        <button class="icon-button prev-button" title="上一首" :disabled="!current" @click.stop="player.prev()">
          <i class="ri-skip-back-fill"></i>
        </button>
        <button class="play-button" :title="state.playing ? '暂停' : '播放'" @click.stop="player.toggle()">
          <i :class="state.playing ? 'ri-pause-fill' : 'ri-play-fill'"></i>
        </button>
        <button class="icon-button next-button" title="下一首" :disabled="!current" @click.stop="player.next()">
          <i class="ri-skip-forward-fill"></i>
        </button>
        <button
          class="icon-button queue-button"
          title="播放列表"
          :class="{ active: state.showQueue }"
          @click.stop="state.showQueue = !state.showQueue"
        >
          <i class="ri-play-list-2-line"></i>
        </button>
      </div>
      <div class="player-progress">
        <span class="time">{{ formatDuration(state.currentTime) }}</span>
        <t-slider
          class="progress-range"
          :value="state.currentTime"
          :max="state.duration || 1"
          :step="0.5"
          :disabled="!current"
          :tooltip-props="{ showArrow: false }"
          @change="onSeek"
        />
        <span class="time">{{ formatDuration(state.duration) }}</span>
      </div>
    </div>

    <div class="player-extra">
      <button class="icon-button" :title="state.muted ? '取消静音' : '静音'" @click.stop="player.toggleMute()">
        <i :class="volume ? 'ri-volume-up-line' : 'ri-volume-mute-line'"></i>
      </button>
      <t-slider
        class="volume-range"
        :value="volume"
        :min="0"
        :max="1"
        :step="0.01"
        :tooltip-props="{ showArrow: false }"
        @change="onVolume"
      />
    </div>
  </footer>
</template>
