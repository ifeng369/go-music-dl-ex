<script setup>
import { nextTick, ref, watch } from 'vue'
import { formatDuration } from '../api'
import { player } from '../player'

const state = player.state
const listRef = ref(null)

/** Centres the playing row inside the panel without scrolling the page. */
function scrollToCurrent() {
  const list = listRef.value
  if (!list) return
  const row = list.querySelector('.queue-row.active')
  if (!row) return
  const offset = row.offsetTop - list.clientHeight / 2 + row.clientHeight / 2
  list.scrollTop = Math.max(0, offset)
}

watch(
  () => state.showQueue,
  async (open) => {
    if (!open) return
    await nextTick()
    requestAnimationFrame(scrollToCurrent)
  },
)

watch(
  () => state.index,
  async () => {
    if (!state.showQueue) return
    await nextTick()
    requestAnimationFrame(scrollToCurrent)
  },
)

function closeQueue() {
  state.showQueue = false
}
</script>

<template>
  <transition name="queue-fade">
    <div v-if="state.showQueue" class="queue-layer" @click.self="closeQueue">
      <aside class="queue-panel" role="dialog" aria-label="播放列表" @click.stop>
        <header class="queue-panel-head">
          <div>
            <h3>播放列表</h3>
            <p>{{ state.queue.length }} 首歌曲</p>
          </div>
          <div class="queue-panel-actions">
            <button class="icon-button" title="清空播放列表" @click="player.clearQueue()">
              <i class="ri-delete-bin-6-line"></i>
            </button>
            <button class="icon-button" title="关闭播放列表" @click="closeQueue">
              <i class="ri-close-line"></i>
            </button>
          </div>
        </header>
        <div ref="listRef" class="queue-list">
          <div
            v-for="(song, index) in state.queue"
            :key="song.id + index"
            class="queue-row"
            :class="{ active: index === state.index }"
            @click="player.playAt(index)"
          >
            <span class="queue-index">
              <i v-if="index === state.index && state.playing" class="ri-volume-up-line"></i>
              <template v-else>{{ String(index + 1).padStart(2, '0') }}</template>
            </span>
            <img v-if="song.artwork || song.cover" class="queue-cover" :src="song.artwork || song.cover" alt="" loading="lazy" /><span v-else class="queue-cover queue-cover-fallback"><i class="ri-music-2-line"></i></span><div class="queue-meta">
              <div class="queue-title">{{ song.title }}</div>
              <div class="queue-artist">{{ song.artist || '未知歌手' }}</div>
            </div>
            <span class="queue-time">{{ formatDuration(song.duration) }}</span>
            <button class="queue-remove" title="移除" @click.stop="player.removeFromQueue(index)">
              <i class="ri-close-line"></i>
            </button>
          </div>
          <div v-if="!state.queue.length" class="queue-empty">
            <i class="ri-play-list-2-line"></i>
            <p>播放列表是空的</p>
          </div>
        </div>
      </aside>
    </div>
  </transition>
</template>
