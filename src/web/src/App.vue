<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppSidebar from './components/AppSidebar.vue'
import PlayerBar from './components/PlayerBar.vue'
import QueuePanel from './components/QueuePanel.vue'
import LyricsPanel from './components/LyricsPanel.vue'
import NowPlaying from './components/NowPlaying.vue'
import { loadMeta, meta } from './meta'
import { auth } from './auth'
import { loadForUser, resetLibrary } from './library'

const route = useRoute()
const router = useRouter()
const keyword = ref(route.query.q || '')

onMounted(() => loadMeta())

watch(
  () => (auth.state.user ? auth.state.user.id : null),
  (id) => {
    if (id && auth.state.user) loadForUser(auth.state.user)
    else resetLibrary()
  },
  { immediate: true },
)

watch(
  () => route.query.q,
  (value) => {
    if (typeof value === 'string') keyword.value = value
  },
)

function submitSearch() {
  const q = keyword.value.trim()
  if (!q) return
  router.push({ name: 'search', query: { q } })
}
</script>

<template>
  <div v-if="route.name === 'login'" class="login-shell">
    <router-view />
  </div>
  <div v-else class="app">
    <AppSidebar />
    <div class="shell">
      <header class="topbar">
        <div class="history-buttons">
          <button class="icon-button" title="后退" @click="router.back()"><i class="ri-arrow-left-s-line"></i></button>
          <button class="icon-button" title="前进" @click="router.forward()"><i class="ri-arrow-right-s-line"></i></button>
        </div>
        <form class="search-box" @submit.prevent="submitSearch">
          <i class="ri-search-line"></i>
          <input v-model="keyword" type="search" placeholder="搜索歌曲、歌单、专辑、歌手" />
          <button v-if="keyword" type="button" class="clear-search" @click="keyword = ''">
            <i class="ri-close-circle-fill"></i>
          </button>
          <button type="submit" class="search-submit">搜索</button>
        </form>
        <div class="topbar-extra">
          <span class="online-hint" :class="{ off: meta.error }">
            <i :class="meta.error ? 'ri-cloud-off-line' : 'ri-cloud-line'"></i>
            {{ meta.error ? '服务异常' : '在线' }}
          </span>
          <router-link to="/settings" class="icon-button" title="设置"><i class="ri-settings-3-line"></i></router-link>
        </div>
      </header>

      <main class="content">
        <router-view v-slot="{ Component }">
          <component :is="Component" />
        </router-view>
      </main>
    </div>

    <PlayerBar />
    <QueuePanel />
    <LyricsPanel />
    <NowPlaying />
  </div>
</template>
