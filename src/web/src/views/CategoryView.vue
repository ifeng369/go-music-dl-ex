<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'
import { loadMeta, sourceName } from '../meta'
import SheetGrid from '../components/SheetGrid.vue'

const route = useRoute()
const items = ref([])
const page = ref(1)
const loading = ref(true)
const loadingMore = ref(false)
const isEnd = ref(false)
const error = ref('')

const categoryName = computed(() => String(route.query.name || route.query.id || '全部'))
const categoryID = computed(() => String(route.query.id || '全部'))

async function load(reset = true) {
  if (reset) {
    page.value = 1
    isEnd.value = false
    error.value = ''
    loading.value = true
  } else {
    loadingMore.value = true
  }
  try {
    const data = await api.categorySheets({
      source: route.params.source,
      category: categoryID.value,
      page: page.value,
      limit: 30,
    })
    const list = data.items || []
    items.value = reset ? list : [...items.value, ...list]
    isEnd.value = Boolean(data.isEnd)
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function more() {
  if (loadingMore.value || isEnd.value) return
  page.value += 1
  load(false)
}

onMounted(async () => {
  await loadMeta()
  load()
})

watch(() => route.fullPath, () => load())
</script>

<template>
  <div class="category-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">分类歌单 · {{ sourceName(route.params.source) }}</span>
        <h1>{{ categoryName }}</h1>
        <p>按分类浏览平台歌单</p>
      </div>
    </header>

    <div v-if="error" class="notice error"><i class="ri-error-warning-line"></i>{{ error }}</div>
    <SheetGrid :items="items" :loading="loading" empty-text="该分类暂无歌单" />
    <div v-if="items.length && !loading" class="load-more">
      <button v-if="!isEnd" class="ghost-button" :disabled="loadingMore" @click="more">
        <i :class="loadingMore ? 'ri-loader-4-line spin' : 'ri-add-line'"></i>
        {{ loadingMore ? '加载中…' : '加载更多' }}
      </button>
      <span v-else>已经到底啦</span>
    </div>
  </div>
</template>
