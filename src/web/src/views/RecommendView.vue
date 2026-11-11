<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../api'
import { loadMeta, sourceName } from '../meta'
import SheetGrid from '../components/SheetGrid.vue'

const loading = ref(true)
const error = ref('')
const allSheets = ref([])
const categories = ref([])
const activeKey = ref('')
const sheetLoading = ref(false)
const sheetError = ref('')
const visibleSheets = ref([])
const cache = new Map()

function keyOf(category) {
  return `${category.source}|${category.id}|${category.name}`
}

function collectSheets(home) {
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

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [home] = await Promise.all([api.home(), loadMeta()])
    allSheets.value = collectSheets(home)
    categories.value = (home.categories || []).filter(
      (item) => item && item.name && item.name.trim() !== '全部',
    )
    visibleSheets.value = allSheets.value
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function selectCategory(category) {
  if (!category) {
    activeKey.value = ''
    sheetError.value = ''
    visibleSheets.value = allSheets.value
    return
  }
  const key = keyOf(category)
  if (activeKey.value === key) {
    selectCategory(null)
    return
  }
  activeKey.value = key
  sheetError.value = ''
  if (cache.has(key)) {
    visibleSheets.value = cache.get(key)
    return
  }
  sheetLoading.value = true
  visibleSheets.value = []
  try {
    const result = await api.categorySheets({
      source: category.source,
      id: category.id || '',
      name: category.name,
    })
    const items = (result.items || []).map((item) => ({ ...item, source: item.source || category.source }))
    cache.set(key, items)
    if (activeKey.value === key) visibleSheets.value = items
  } catch (err) {
    if (activeKey.value === key) sheetError.value = err.message || '该分类加载失败'
  } finally {
    if (activeKey.value === key) sheetLoading.value = false
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
        <p>先选分类，不选就显示全部推荐歌单。</p>
      </div>
    </header>

    <div v-if="error" class="notice error">
      <i class="ri-error-warning-line"></i>
      {{ error }}
      <button class="text-button" @click="load">重试</button>
    </div>

    <section class="section recommd-cats">
      <div v-if="loading" class="chip-row">
        <span v-for="i in 18" :key="i" class="chip skeleton-chip"></span>
      </div>
      <div v-else-if="categories.length" class="chip-row">
        <button class="chip" :class="{ active: !activeKey }" @click="selectCategory(null)">全部</button>
        <button
          v-for="category in categories.slice(0, 60)"
          :key="keyOf(category)"
          class="chip"
          :class="{ active: activeKey === keyOf(category) }"
          :title="sourceName(category.source)"
          @click="selectCategory(category)"
        >
          {{ category.name }}
        </button>
      </div>
    </section>

    <div v-if="sheetError" class="notice error compact">
      <i class="ri-error-warning-line"></i>
      {{ sheetError }}
    </div>

    <SheetGrid
      :items="visibleSheets"
      :loading="loading || sheetLoading"
      :empty-text="activeKey ? '该分类暂无歌单' : '暂无推荐歌单'"
    />
  </div>
</template>
