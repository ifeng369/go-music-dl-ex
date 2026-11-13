<script setup>
import { onMounted, ref } from 'vue'
import SongList from '../components/SongList.vue'
import { api } from '../api'
import { player } from '../player'

const songs = ref([])
const loading = ref(true)
const busy = ref(false)
const message = ref('')
const dir = ref('')
const managing = ref(false)
const selected = ref([])

async function load() {
  loading.value = true
  try {
    const data = await api.local.list()
    songs.value = data.items || []
    dir.value = data.dir || ''
    selected.value = selected.value.filter((id) => songs.value.some((song) => song.id === id))
  } catch (err) {
    message.value = err.message || '本地音乐加载失败'
  } finally {
    loading.value = false
  }
}

function songName(song) {
  return song.sourceId || song.extra?.localName
}

function toggleManage() {
  managing.value = !managing.value
  if (!managing.value) selected.value = []
}

function toggleSelect(song) {
  const index = selected.value.indexOf(song.id)
  if (index >= 0) selected.value.splice(index, 1)
  else selected.value.push(song.id)
}

async function removeSelected() {
  if (!selected.value.length || busy.value) return
  const targets = songs.value.filter((song) => selected.value.includes(song.id))
  if (!window.confirm(`确定删除选中的 ${targets.length} 首本地音乐吗？文件会从账号目录中删除。`)) return
  busy.value = true
  message.value = ''
  let failed = 0
  for (const song of targets) {
    try {
      await api.local.remove(songName(song))
    } catch {
      failed += 1
    }
  }
  await load()
  selected.value = []
  busy.value = false
  message.value = failed
    ? `已删除 ${targets.length - failed} 首，${failed} 首删除失败`
    : `已删除 ${targets.length} 首本地音乐`
}

onMounted(load)
</script>

<template>
  <div class="library-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">我的音乐</span>
        <h1>本地音乐</h1>
        <p>下载完成的歌曲会自动出现在这里，也可以管理账号目录中已有的本地音乐。</p>
      </div>
      <div class="library-actions">
        <button class="ghost-button" :class="{ active: managing }" @click="toggleManage">
          <i :class="managing ? 'ri-close-line' : 'ri-list-check-2'"></i>
          {{ managing ? '退出管理' : '音乐管理' }}
        </button>
      </div>
    </header>

    <div v-if="message" class="notice">
      <i class="ri-information-line"></i>{{ message }}
    </div>

    <div v-if="managing" class="manage-toolbar">
      <span class="manage-count">
        <i class="ri-checkbox-multiple-line"></i>
        已选择 {{ selected.length }} 首
      </span>
      <button v-if="selected.length" class="danger-button" :disabled="busy" @click="removeSelected">
        <i :class="busy ? 'ri-loader-4-line spin' : 'ri-delete-bin-6-line'"></i>
        {{ busy ? '删除中…' : '删除' }}
      </button>
      <span v-else class="manage-hint">勾选要删除的本地音乐</span>
    </div>

    <div class="library-summary">
      <span><i class="ri-music-2-line"></i> {{ songs.length }} 首歌曲</span>
      <span class="library-tip">{{ dir || '账号本地音乐目录' }}</span>
    </div>

    <SongList
      :songs="songs"
      :loading="loading"
      show-file-meta
      :selectable="managing"
      :selected="selected"
      empty-text="还没有本地音乐，下载完成的歌曲会自动出现在这里"
      @play="player.playSongFrom(songs, $event)"
      @add="player.addToQueue([$event])"
      @select="toggleSelect"
    />
  </div>
</template>
