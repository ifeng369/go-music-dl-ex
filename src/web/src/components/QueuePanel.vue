<script setup>
import { formatDuration } from '../api'
import { player } from '../player'

const state = player.state
</script>

<template>
  <transition name="slide-right">
    <aside v-if="state.showQueue" class="queue-panel">
      <header class="panel-header">
        <div>
          <h3>播放队列</h3>
          <p>{{ state.queue.length }} 首歌曲</p>
        </div>
        <div class="panel-actions">
          <button class="icon-button" title="清空队列" @click="player.clearQueue()">
            <i class="ri-delete-bin-6-line"></i>
          </button>
          <button class="icon-button" title="关闭" @click="state.showQueue = false">
            <i class="ri-close-line"></i>
          </button>
        </div>
      </header>
      <div class="queue-list">
        <div
          v-for="(song, index) in state.queue"
          :key="song.id + index"
          class="queue-row"
          :class="{ active: index === state.index }"
          @dblclick="player.playAt(index)"
        >
          <span class="queue-index">
            <i v-if="index === state.index && state.playing" class="ri-volume-up-line"></i>
            <template v-else>{{ index + 1 }}</template>
          </span>
          <img v-if="song.artwork || song.cover" class="queue-cover" :src="song.artwork || song.cover" alt="" loading="lazy" /><span v-else class="queue-cover queue-cover-fallback"><i class="ri-music-2-line"></i></span><div class="queue-meta">
            <div class="queue-title">{{ song.title }}</div>
            <div class="queue-artist">{{ song.artist }}</div>
          </div>
          <span class="queue-time">{{ formatDuration(song.duration) }}</span>
          <button class="queue-remove" title="移除" @click.stop="player.removeFromQueue(index)">
            <i class="ri-close-line"></i>
          </button>
        </div>
        <div v-if="!state.queue.length" class="empty">队列是空的</div>
      </div>
    </aside>
  </transition>
</template>
