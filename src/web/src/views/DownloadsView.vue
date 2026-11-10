<script setup>
import { onMounted, ref } from 'vue'
import { api, formatSize } from '../api'
import { sourceName } from '../meta'

const items = ref([])
const loading = ref(true)
const error = ref('')
const message = ref('')
const clearing = ref(false)

function formatTime(value) {
  if (!value) return '—'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.downloads.list({ limit: 300 })
    items.value = data.items || []
  } catch (err) {
    error.value = err.message || '下载记录加载失败'
  } finally {
    loading.value = false
  }
}

async function remove(item) {
  if (!window.confirm(`删除「${item.title}」的下载记录？下载的文件不会被删除。`)) return
  try {
    await api.downloads.remove(item.id)
    items.value = items.value.filter((row) => row.id !== item.id)
    message.value = '下载记录已删除'
  } catch (err) {
    error.value = err.message || '删除失败'
  }
}

async function clearAll() {
  if (!items.value.length) return
  if (!window.confirm('确定清空全部下载记录吗？下载的文件不会被删除。')) return
  clearing.value = true
  try {
    await api.downloads.clear()
    items.value = []
    message.value = '下载记录已清空'
  } catch (err) {
    error.value = err.message || '清空失败'
  } finally {
    clearing.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="library-view downloads-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">我的音乐</span>
        <h1>下载记录</h1>
        <p>这里记录当前账号下载过的歌曲，文件保存在账号自己的下载目录中。</p>
      </div>
      <div class="library-actions">
        <button class="ghost-button" :disabled="!items.length || clearing" @click="clearAll">
          <i class="ri-delete-bin-6-line"></i> 清空记录
        </button>
        <button class="ghost-button" :disabled="loading" @click="load">
          <i :class="loading ? 'ri-loader-4-line spin' : 'ri-refresh-line'"></i> 刷新
        </button>
      </div>
    </header>

    <div v-if="message" class="notice">
      <i class="ri-information-line"></i>{{ message }}
    </div>
    <div v-if="error" class="notice error">
      <i class="ri-error-warning-line"></i>{{ error }}
      <button class="text-button" @click="load">重试</button>
    </div>

    <div class="library-summary">
      <span><i class="ri-download-cloud-line"></i> {{ items.length }} 条下载记录</span>
      <span class="library-tip">仅记录下载历史，不会自动删除已下载文件</span>
    </div>

    <div v-if="loading" class="download-list">
      <div v-for="i in 6" :key="i" class="download-row skeleton-row">
        <span class="skeleton-line"></span>
      </div>
    </div>

    <div v-else-if="items.length" class="download-list">
      <article v-for="item in items" :key="item.id" class="download-row">
        <img v-if="item.cover" class="download-cover" :src="item.cover" alt="" loading="lazy" /><span v-else class="download-icon"><i class="ri-file-music-line"></i></span>
        <div class="download-meta">
          <strong :title="item.title">{{ item.title || '未知歌曲' }}</strong>
          <span>
            {{ item.artist || '未知歌手' }}
            <template v-if="item.album"> · {{ item.album }}</template>
          </span>
          <small :title="item.path">{{ item.filename || item.path }}</small>
        </div>
        <span class="download-source">{{ sourceName(item.source) }}</span>
        <span class="download-size">{{ formatSize(item.size) }}</span>
        <span class="download-time">{{ formatTime(item.createdAt) }}</span>
        <div class="download-actions">
          <button class="icon-button" title="删除记录" @click="remove(item)">
            <i class="ri-delete-bin-6-line"></i>
          </button>
        </div>
      </article>
    </div>

    <div v-else class="empty large">
      <i class="ri-download-cloud-line"></i>
      <p>还没有下载记录，去歌单里下载一首歌试试</p>
    </div>
  </div>
</template>
