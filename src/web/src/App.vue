<script setup>
import { computed, onMounted, ref, watch } from 'vue'
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
const navOpen = ref(false)

const userInitial = computed(() => {
  const name = auth.state.user && auth.state.user.username
  return name ? name.slice(0, 1).toUpperCase() : ''
})

onMounted(() => loadMeta())

watch(
  () => route.fullPath,
  () => {
    navOpen.value = false
  },
)

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
    <div v-if="navOpen" class="sidebar-backdrop" @click="navOpen = false"></div>
    <AppSidebar :open="navOpen" @close="navOpen = false" />
    <div class="shell">
      <header class="topbar">
        <button class="icon-button mobile-menu" title="打开菜单" @click="navOpen = true">
          <i class="ri-menu-2-line"></i>
        </button>
        <div class="history-buttons">
          <button class="icon-button" title="后退" @click="router.back()"><i class="ri-arrow-left-s-line"></i></button>
          <button class="icon-button" title="前进" @click="router.forward()"><i class="ri-arrow-right-s-line"></i></button>
        </div>
        <form class="search-box" @submit.prevent="submitSearch">
          <i class="ri-search-line"></i>
          <input v-model="keyword" type="search" placeholder="搜索音乐、歌单、歌手" />
          <button v-if="keyword" type="button" class="clear-search" title="清空" @click="keyword = ''">
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
          <router-link v-if="auth.state.user" to="/settings" class="topbar-user" :title="auth.state.user.username">
            {{ userInitial }}
          </router-link>
          <router-link v-else to="/login" class="topbar-login">登录</router-link>
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
