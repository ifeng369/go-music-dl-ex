<script setup>
import SheetCard from './SheetCard.vue'

defineProps({
  items: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  emptyText: { type: String, default: '暂无内容' },
  round: { type: Boolean, default: false },
})
</script>

<template>
  <div v-if="loading" class="sheet-grid">
    <div v-for="i in 8" :key="i" class="sheet-card skeleton">
      <div class="sheet-cover"></div>
      <div class="skeleton-line"></div>
      <div class="skeleton-line short"></div>
    </div>
  </div>
  <div v-else-if="items.length" class="sheet-grid">
    <SheetCard v-for="item in items" :key="item.id + item.type" :item="item" :round="round" />
  </div>
  <div v-else class="empty">{{ emptyText }}</div>
</template>
