<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { loadMeta } from '../meta'
import SheetCard from '../components/SheetCard.vue'

const loading = ref(true)
const error = ref('')
const groups = ref([])

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [data] = await Promise.all([api.toplists(), loadMeta()])
    groups.value = data.groups || []
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="toplist-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">排行榜</span>
        <h1>热门榜单</h1>
        <p>各平台实时更新的音乐排行榜</p>
      </div>
      <button class="ghost-button" @click="load"><i class="ri-refresh-line"></i> 刷新</button>
    </header>

    <div v-if="error" class="notice error"><i class="ri-error-warning-line"></i>{{ error }}</div>

    <template v-if="loading">
      <section v-for="i in 2" :key="i" class="section">
        <div class="skeleton-line title-skeleton"></div>
        <div class="sheet-grid">
          <div v-for="j in 6" :key="j" class="sheet-card skeleton">
            <div class="sheet-cover"></div>
            <div class="skeleton-line"></div>
          </div>
        </div>
      </section>
    </template>

    <section v-for="group in groups" v-else :key="group.source" class="section">
      <div class="section-header">
        <div class="section-title">
          <h2>{{ group.title }}</h2>
          <span class="section-subtitle">{{ group.data.length }} 个榜单</span>
        </div>
      </div>
      <div class="sheet-grid">
        <SheetCard v-for="item in group.data" :key="item.id" :item="item" />
      </div>
    </section>

    <div v-if="!loading && !groups.length" class="empty">暂无排行榜数据</div>
  </div>
</template>
