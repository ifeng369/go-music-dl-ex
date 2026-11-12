<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'
import { loadMeta } from '../meta'
import SheetGrid from '../components/SheetGrid.vue'

const route = useRoute()
const loading = ref(true)
const error = ref('')
const groups = ref([])
const categories = ref([])
const activeKey = ref('recommend')
const sheetLoading = ref(false)
const sheetError = ref('')
const categorySheets = ref([])
const cache = new Map()

const sourceFilter = computed(() => String(route.query.source || ''))
const RECOMMEND_KEY = 'recommend'
const ALL_KEY = 'all'

function keyOf(category) {
  return `${category.source}|${category.id}|${category.name}`
}

function collect(home) {
  const list = []
  const seen = new Set()
  const push = (item, fallbackSource) => {
    if (!item || !item.id) return
    const source = item.source || fallbackSource || ''
    const key = `${source}:${item.id}`
    if (seen.has(key)) return
    seen.add(key)
    list.push({ ...item, source })
  }
  for (const group of home.recommendGroups || []) {
    for (const item of group.data || []) push(item, group.source)
  }
  for (const item of home.recommend || []) push(item, item.source)
  return list
}

const allSheets = ref([])

// "全部" means all sheets of the current platform when a platform is set.
const scopedSheets = computed(() => {
  const source = sourceFilter.value
  if (!source) return allSheets.value
  return allSheets.value.filter((item) => item.source === source)
})

const recommendSheets = computed(() => {
  const source = sourceFilter.value
  const group = groups.value.find((item) => item.source === source)
  if (group) {
    return (group.data || []).map((item) => ({ ...item, source: group.source }))
  }
  // No source supplied: every platform's recommendation, merged.
  return scopedSheets.value
})

const visibleCats = computed(() => {
  const source = sourceFilter.value
  return source ? categories.value.filter((item) => item.source === source) : categories.value
})

const visibleSheets = computed(() => {
  if (activeKey.value === ALL_KEY) return scopedSheets.value
  if (activeKey.value === RECOMMEND_KEY) return recommendSheets.value
  return categorySheets.value
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [home] = await Promise.all([api.home(), loadMeta()])
    groups.value = home.recommendGroups || []
    allSheets.value = collect(home)
    categories.value = (home.categories || []).filter(
      (item) => item && item.name && item.name.trim() !== '全部',
    )
    activeKey.value = RECOMMEND_KEY
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function selectChip(category) {
  if (!category) {
    activeKey.value = RECOMMEND_KEY
    return
  }
  activeKey.value = keyOf(category)
  sheetError.value = ''
  const cached = cache.get(activeKey.value)
  if (cached) {
    categorySheets.value = cached
    return
  }
  sheetLoading.value = true
  categorySheets.value = []
  try {
    const result = await api.categorySheets({
      source: category.source,
      id: category.id || '',
      name: category.name,
    })
    const items = (result.items || []).map((item) => ({ ...item, source: item.source || category.source }))
    cache.set(activeKey.value, items)
    if (activeKey.value === keyOf(category)) categorySheets.value = items
  } catch (err) {
    sheetError.value = err.message || '该分类加载失败'
  } finally {
    sheetLoading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="recommend-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">发现音乐</span>
        <h1>推荐歌单</h1>
        <p>默认显示当前平台的推荐歌单，选择分类可查看该类目。</p>
      </div>
    </header>

    <div v-if="error" class="notice error">
      <i class="ri-error-warning-line"></i>
      {{ error }}
      <button class="text-button" @click="load">重试</button>
    </div>

    <section class="section recommd-cats">
      <div v-if="loading" class="chip-row">
        <span v-for="i in 16" :key="i" class="chip skeleton-chip"></span>
      </div>
      <div v-else class="chip-row">
        <button
          class="chip"
          :class="{ active: activeKey === RECOMMEND_KEY }"
          @click="activeKey = RECOMMEND_KEY"
        >
          推荐
        </button>
        <button class="chip" :class="{ active: activeKey === ALL_KEY }" @click="activeKey = ALL_KEY">
          全部
        </button>
        <button
          v-for="category in visibleCats.slice(0, 60)"
          :key="keyOf(category)"
          class="chip"
          :class="{ active: activeKey === keyOf(category) }"
          @click="selectChip(category)"
        >
          {{ category.name }}
        </button>
      </div>
    </section>

    <div v-if="sheetError" class="notice error compact">
      <i class="ri-error-warning-line"></i>
      {{ sheetError }}
    </div>

    <div class="recommend-grid">
      <SheetGrid
        :items="visibleSheets"
        :loading="loading || sheetLoading"
        :empty-text="activeKey === RECOMMEND_KEY ? '该平台暂无推荐歌单' : '该分类暂无歌单'"
      />
    </div>
  </div>
</template>
