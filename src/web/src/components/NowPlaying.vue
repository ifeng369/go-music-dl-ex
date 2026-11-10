<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { formatBitrate, formatDuration, formatSize } from '../api'
import { player } from '../player'
import { isFavorite, toggleFavorite } from '../library'
import { sourceName } from '../meta'

const route = useRoute()
const state = player.state
const current = player.current
const progress = player.progress
const sourceMenuOpen = ref(false)
const switcherRef = ref(null)
const showLyricsView = ref(false)
const lyricScroll = ref(null)

const liked = computed(() => Boolean(current.value && isFavorite(current.value)))
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
const currentLyric = computed(() => {
  const line = state.lyrics[state.lyricIndex]
  return line ? line.text : ''
})
watch(
  [() => state.lyricIndex, showLyricsView],
  async () => {
    if (!showLyricsView.value) return
    await nextTick()
    lyricScroll.value?.querySelector('.now-lyrics-line.active')?.scrollIntoView({
      block: 'center',
      behavior: 'smooth',
    })
  },
)

const backdropStyle = computed(() => (
  current.value && current.value.artwork
    ? { '--now-cover': `url(${current.value.artwork})` }
    : {}
))

function close() {
  state.showNowPlaying = false
}

function seek(event) {
  player.seek(Number(event.target.value))
}

function toggleLike() {
  if (current.value) toggleFavorite(current.value)
}

function removeAt(index, event) {
  event.stopPropagation()
  player.removeFromQueue(index)
}

async function toggleSourceMenu() {
  if (sourceMenuOpen.value) {
    sourceMenuOpen.value = false
    return
  }
  if (!current.value) return
  sourceMenuOpen.value = true
  await player.loadAlternatives(current.value)
}

function switchSource(song) {
  sourceMenuOpen.value = false
  player.switchSource(song)
}

function onDocumentClick(event) {
  if (!sourceMenuOpen.value) return
  if (switcherRef.value && !switcherRef.value.contains(event.target)) {
    sourceMenuOpen.value = false
  }
}

function onKeydown(event) {
  if (event.key !== 'Escape') return
  if (sourceMenuOpen.value) {
    sourceMenuOpen.value = false
    return
  }
  if (state.showNowPlaying) close()
}

watch(
  () => state.showNowPlaying,
  (open) => {
    document.body.classList.toggle('now-playing-open', open)
    if (!open) sourceMenuOpen.value = false
  },
  { immediate: true },
)

watch(
  [current, () => state.showNowPlaying],
  () => {
    sourceMenuOpen.value = false
  },
  { immediate: true },
)

watch(
  () => route.fullPath,
  () => {
    if (state.showNowPlaying) close()
  },
)

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  document.addEventListener('click', onDocumentClick)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  document.removeEventListener('click', onDocumentClick)
  document.body.classList.remove('now-playing-open')
})
</script>

<template>
  <transition name="now-fade">
    <div v-if="state.showNowPlaying" class="now-playing">
      <div class="now-backdrop" :style="backdropStyle"></div>
      <div class="now-shade"></div>

      <header class="now-topbar">
        <button class="icon-button" title="收起 (Esc)" @click="close">
          <i class="ri-arrow-down-line"></i>
        </button>
      </header>

      <div class="now-layout">
        <section class="now-main" :class="{ 'lyrics-mode': showLyricsView }">
          <template v-if="current">
            <template v-if="!showLyricsView">
              <div
                class="now-art"
                :class="{ spinning: state.playing }"
                title="点击查看歌词"
                role="button"
                tabindex="0"
                @click="showLyricsView = true"
                @keydown.enter="showLyricsView = true"
                @keydown.space.prevent="showLyricsView = true"
              >
                <img v-if="current.artwork" :src="current.artwork" :alt="current.title" />
                <div v-else class="now-art-fallback"><i class="ri-music-2-line"></i></div>
                <span class="now-art-ring"></span>
              </div>

              <div class="now-meta">
                <h1>{{ current.title }}</h1>
                <p>{{ current.artist || '未知歌手' }}<span v-if="current.album"> · {{ current.album }}</span></p>
              </div>
            </template>

            <div v-else class="now-lyrics-view">
              <div class="now-lyrics-head">
                <span><i class="ri-file-music-line"></i> 歌词</span>
                <button class="now-lyrics-back" title="显示封面" @click="showLyricsView = false">
                  <i class="ri-image-line"></i> 显示封面
                </button>
              </div>
              <div ref="lyricScroll" class="now-lyrics-scroll">
                <p
                  v-for="(line, index) in state.lyrics"
                  :key="index"
                  class="now-lyrics-line"
                  :class="{ active: index === state.lyricIndex }"
                  @click="player.seek(line.time)"
                >
                  {{ line.text }}
                </p>
                <div v-if="!state.lyrics.length" class="now-lyrics-empty">当前歌曲暂无歌词</div>
              </div>
            </div>

            <div ref="switcherRef" class="source-switcher">
              <button class="source-trigger" :class="{ open: sourceMenuOpen }" @click.stop="toggleSourceMenu">
                <span class="source-trigger-text">换源</span>
                <i
                  class="source-trigger-arrow"
                  :class="state.alternativesLoading ? 'ri-loader-4-line spin' : (sourceMenuOpen ? 'ri-arrow-up-s-line' : 'ri-arrow-down-s-line')"
                ></i>
              </button>

              <div v-if="sourceMenuOpen" class="source-dropdown" @click.stop>
                <div class="source-dropdown-head">
                  <span>选择其他平台播放</span>
                  <span v-if="state.switching" class="source-switching"><i class="ri-loader-4-line spin"></i> 换源中</span>
                  <span v-else class="source-dropdown-current">当前 {{ sourceName(current.source) }}</span>
                </div>
                <button
                  v-for="item in state.alternatives"
                  :key="item.id"
                  class="source-option"
                  @click="switchSource(item)"
                >
                  <span class="source-option-main">
                    <strong>{{ sourceName(item.source) }}</strong>
                    <span class="source-option-song">{{ item.title }}<template v-if="item.artist"> · {{ item.artist }}</template></span>
                  </span>
                  <span class="source-option-stats">
                    <span>码率 {{ formatBitrate(item.bitrate) }}</span>
                    <span>大小 {{ formatSize(item.size) }}</span>
                    <span>时长 {{ formatDuration(item.duration) }}</span>
                  </span>
                  <i class="ri-play-circle-line source-option-play"></i>
                </button>
                <div v-if="state.alternativesLoading" class="source-dropdown-state">
                  <i class="ri-loader-4-line spin"></i> 正在搜索其他音源…
                </div>
                <div v-else-if="!state.alternatives.length" class="source-dropdown-state">
                  没有找到其他平台的可用版本
                </div>
              </div>
            </div>

            <div class="now-progress">
              <span class="time">{{ formatDuration(state.currentTime) }}</span>
              <input
                class="range"
                type="range"
                min="0"
                :max="state.duration || 0"
                step="0.5"
                :value="state.currentTime"
                :style="{ '--fill': `${progress * 100}%` }"
                @input="seek"
              />
              <span class="time">{{ formatDuration(state.duration) }}</span>
            </div>

            <div class="now-controls">
              <button class="icon-button" :title="modeTitle" @click="player.cycleMode()">
                <i :class="modeIcon"></i>
              </button>
              <button class="icon-button" title="上一首" @click="player.prev()">
                <i class="ri-skip-back-fill"></i>
              </button>
              <button class="play-button now-play" :title="state.playing ? '暂停' : '播放'" @click="player.toggle()">
                <i :class="state.playing ? 'ri-pause-fill' : 'ri-play-fill'"></i>
              </button>
              <button class="icon-button" title="下一首" @click="player.next()">
                <i class="ri-skip-forward-fill"></i>
              </button>
              <button
                class="icon-button"
                :class="{ active: liked }"
                :title="liked ? '取消收藏' : '收藏'"
                @click="toggleLike"
              >
                <i :class="liked ? 'ri-heart-3-fill' : 'ri-heart-3-line'"></i>
              </button>
            </div>

            <div v-if="state.error || !showLyricsView" class="now-lyric">
              <span v-if="state.error" class="player-error">{{ state.error }}</span>
              <template v-else-if="currentLyric">{{ currentLyric }}</template>
              <template v-else>当前歌曲暂无歌词</template>
            </div>
          </template>
          <div v-else class="now-empty">
            <i class="ri-music-2-line"></i>
            <p>还没有播放音乐</p>
          </div>
        </section>

        <aside class="now-queue">
          <header class="now-queue-head">
            <div>
              <h3>播放列表</h3>
              <p>{{ state.queue.length }} 首歌曲</p>
            </div>
            <button class="icon-button" title="清空播放列表" @click="player.clearQueue()">
              <i class="ri-delete-bin-6-line"></i>
            </button>
          </header>
          <div class="now-queue-list">
            <div
              v-for="(song, index) in state.queue"
              :key="song.id + index"
              class="now-queue-row"
              :class="{ active: index === state.index }"
              @click="player.playAt(index)"
            >
              <span class="now-queue-index">
                <i v-if="index === state.index && state.playing" class="ri-volume-up-line"></i>
                <template v-else>{{ String(index + 1).padStart(2, '0') }}</template>
              </span>
              <img v-if="song.artwork || song.cover" class="now-queue-cover" :src="song.artwork || song.cover" alt="" loading="lazy" /><span v-else class="now-queue-cover now-queue-cover-fallback"><i class="ri-music-2-line"></i></span><div class="now-queue-meta">
                <div class="now-queue-title">{{ song.title }}</div>
                <div class="now-queue-artist">{{ song.artist || '未知歌手' }}</div>
              </div>
              <span class="now-queue-time">{{ formatDuration(song.duration) }}</span>
              <button class="now-queue-remove" title="移除" @click="removeAt(index, $event)">
                <i class="ri-close-line"></i>
              </button>
            </div>
            <div v-if="!state.queue.length" class="now-queue-empty">
              <i class="ri-play-list-2-line"></i>
              <p>播放列表是空的</p>
            </div>
          </div>
        </aside>
      </div>
    </div>
  </transition>
</template>
