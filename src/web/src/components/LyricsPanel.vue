<script setup>
import { nextTick, ref, watch } from 'vue'
import { player } from '../player'

const state = player.state
const current = player.current
const scroller = ref(null)

watch(
  () => state.lyricIndex,
  async () => {
    await nextTick()
    const container = scroller.value
    const active = container && container.querySelector('.lyric-line.active')
    if (container && active) {
      container.scrollTo({ top: active.offsetTop - container.clientHeight / 2 + active.clientHeight / 2, behavior: 'smooth' })
    }
  },
)
</script>

<template>
  <transition name="slide-right">
    <aside v-if="state.showLyrics" class="lyrics-panel">
      <button class="icon-button lyrics-close" title="关闭" @click="state.showLyrics = false">
        <i class="ri-close-line"></i>
      </button>
      <div class="lyrics-cover">
        <img v-if="current && current.artwork" :src="current.artwork" :alt="current.title" />
        <i v-else class="ri-music-2-line"></i>
      </div>
      <div class="lyrics-heading">
        <h3>{{ current ? current.title : '暂无播放' }}</h3>
        <p>{{ current ? current.artist : '' }}</p>
      </div>
      <div ref="scroller" class="lyrics-scroll">
        <div v-if="!state.lyrics.length" class="lyrics-empty">
          {{ current ? '暂无歌词' : '播放歌曲后显示歌词' }}
        </div>
        <p
          v-for="(line, index) in state.lyrics"
          :key="index"
          class="lyric-line"
          :class="{ active: index === state.lyricIndex }"
          @click="player.seek(line.time)"
        >
          {{ line.text }}
        </p>
      </div>
    </aside>
  </transition>
</template>
