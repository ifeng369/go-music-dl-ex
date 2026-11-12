<script setup>
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { player } from '../player'
import { REC_META, recs, buildDaily, buildLiked, buildRandom, buildHot } from '../recs'
import SongList from '../components/SongList.vue'

const route = useRoute()
const router = useRouter()
const kind = computed(() => {
  const value = String(route.params.kind || 'daily')
  return REC_META[value] ? value : 'daily'
})
const meta = computed(() => REC_META[kind.value])
const list = computed(() => recs[kind.value]?.songs || [])

const builders = {
  daily: buildDaily,
  liked: buildLiked,
  random: buildRandom,
  hot: buildHot,
}

onMounted(async () => {
  const build = builders[kind.value]
  if (build) await build()
})

function playAll() {
  if (!list.value.length) return
  player.playSongs(list.value, 0, kind.value === 'random' || kind.value === 'liked' ? kind.value : '')
}
</script>

<template>
  <div class="rec-view">
    <header class="page-head rec-head">
      <div>
        <span class="eyebrow">为你生成</span>
        <h1>{{ meta.title }}</h1>
        <p class="rec-desc">{{ meta.desc }}</p>
      </div>
      <div class="rec-head-actions">
        <button class="primary-button" :disabled="!list.length" @click="playAll">
          <i class="ri-play-fill"></i> 播放全部
        </button>
      </div>
    </header>

    <div class="library-summary">
      <span><i class="ri-music-2-line"></i> {{ list.length }} 首</span>
      <span class="library-tip">最近生成过的歌曲会自动避开</span>
    </div>

    <SongList
      :songs="list"
      show-album
      show-source
      show-download
      :loading="recs.loading[kind] && !list.length"
      empty-text="正在生成…"
      @play="playAll"
    />
  </div>
</template>
