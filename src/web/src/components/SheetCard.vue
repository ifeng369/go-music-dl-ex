<script setup>
import { computed } from 'vue'
import { formatCount } from '../api'
import { sourceName } from '../meta'

const props = defineProps({
  item: { type: Object, required: true },
  round: { type: Boolean, default: false },
})

const link = computed(() => {
  const item = props.item
  return {
    name: 'sheet',
    params: { source: item.source, id: item.sourceId || item.id },
    query: {
      type: item.type || 'playlist',
      title: item.title,
      cover: item.artwork || '',
      desc: item.description || '',
      artist: item.artist || '',
      count: item.playCount || '',
      works: item.worksNum || '',
    },
  }
})
</script>

<template>
  <router-link :to="link" class="sheet-card" :class="{ round }">
    <div class="sheet-cover">
      <img v-if="item.artwork" :src="item.artwork" :alt="item.title" loading="lazy" />
      <div v-else class="cover-fallback"><i class="ri-music-2-line"></i></div>
      <span class="play-badge"><i class="ri-play-fill"></i></span>
      <span v-if="item.playCount" class="count-badge">
        <i class="ri-headphone-line"></i>{{ formatCount(item.playCount) }}
      </span>
    </div>
    <div class="sheet-meta">
      <div class="sheet-title" :title="item.title">{{ item.title }}</div>
      <div class="sheet-sub">
        <span v-if="item.artist">{{ item.artist }}</span>
        <span v-else>{{ sourceName(item.source) }}</span>
        <span v-if="item.worksNum"> · {{ item.worksNum }} 首</span>
      </div>
    </div>
  </router-link>
</template>
