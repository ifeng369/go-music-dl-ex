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
const sourceMenuOpen = ref(false)
const switcherRef = ref(null)
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

watch(
  [() => state.lyricIndex, () => state.showNowPlaying, () => current.value?.id],
  async () => {
    if (!state.showNowPlaying) return
    await nextTick()
    lyricScroll.value?.querySelector('.now-lyrics-line.active')?.scrollIntoView({
      block: 'center',
      behavior: 'smooth',
    })
  },
  { immediate: true },
)

const backdropStyle = computed(() => (
  current.value && current.value.artwork
    ? { '--now-cover': `url(${current.value.artwork})` }
    : {}
))

function close() {
  state.showNowPlaying = false
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
          <i class="ri-arrow-down-s-line"></i>
        </button>
      </header>

      <div class="now-layout">
        <section class="now-stage">
          <template v-if="current">
            <div class="now-art" :class="{ spinning: state.playing }">
              <img v-if="current.artwork" :src="current.artwork" :alt="current.title" />
              <div v-else class="now-art-fallback"><i class="ri-music-2-line"></i></div>
              <span class="now-art-ring"></span>
            </div>
          </template>
          <div v-else class="now-empty">
            <i class="ri-music-2-line"></i>
            <p>还没有播放音乐</p>
          </div>
        </section>

        <section v-if="current" class="now-lyrics-view">
          <header class="now-lyrics-head">
            <h1 class="now-title" :title="current.title">{{ current.title }}</h1>
            <p class="now-sub">
              <span v-if="current.album">专辑：{{ current.album }}</span>
              <span>歌手：{{ current.artist || '未知歌手' }}</span>
              <span>来源：{{ sourceName(current.source) }}</span>
            </p>

            <div class="now-actions">
              <button
                class="now-like"
                :class="{ active: liked }"
                :title="liked ? '取消收藏' : '收藏'"
                @click="toggleLike"
              >
                <i :class="liked ? 'ri-heart-3-fill' : 'ri-heart-3-line'"></i>
              </button>

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
            </div>

            <p v-if="state.error" class="now-error">{{ state.error }}</p>
          </header>

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
        </section>
      </div>
    </div>
  </transition>
</template>
