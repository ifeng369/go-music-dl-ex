<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { meta } from '../meta'
import { auth } from '../auth'

const route = useRoute()
const router = useRouter()

const navGroups = computed(() => {
  const groups = [
    {
      label: '',
      items: [
        { to: '/', icon: 'ri-compass-3-line', active: 'ri-compass-3-fill', label: '发现音乐', name: 'home' },
        { to: '/toplist', icon: 'ri-bar-chart-2-line', active: 'ri-bar-chart-2-fill', label: '排行榜', name: 'toplist' },
        { to: '/search', icon: 'ri-search-line', active: 'ri-search-fill', label: '搜索', name: 'search' },
      ],
    },
    {
      label: '我的音乐',
      items: [
        { to: '/local', icon: 'ri-folder-music-line', active: 'ri-folder-music-fill', label: '本地音乐', name: 'local' },
        { to: '/my', icon: 'ri-play-list-2-line', active: 'ri-play-list-2-fill', label: '我的歌单', name: 'my-playlists' },
        { to: '/favorites', icon: 'ri-heart-3-line', active: 'ri-heart-3-fill', label: '收藏音乐', name: 'favorites' },
        { to: '/history', icon: 'ri-history-line', active: 'ri-history-fill', label: '播放记录', name: 'history' },
      { to: '/downloads', icon: 'ri-download-cloud-line', active: 'ri-download-cloud-fill', label: '下载记录', name: 'downloads' },
      ],
    },
  ]
  if (auth.isAdmin.value) {
    groups.push({
      label: '管理',
      items: [
        { to: '/users', icon: 'ri-team-line', active: 'ri-team-fill', label: '用户管理', name: 'users' },
      ],
    })
  }
  groups.push({
    label: '',
    items: [
      { to: '/settings', icon: 'ri-settings-3-line', active: 'ri-settings-3-fill', label: '设置', name: 'settings' },
    ],
  })
  return groups
})

const online = computed(() => Boolean(meta.health && meta.health.status === 'ok'))

function isActive(item) {
  if (route.name === item.name) return true
  return item.name === 'my-playlists' && route.name === 'my-playlist'
}

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <aside class="sidebar">
    <router-link to="/" class="brand">
      <span class="brand-mark"><i class="ri-music-2-fill"></i></span>
      <span class="brand-text">
        <strong>MusicDL</strong>
        <small>开源音乐聚合</small>
      </span>
    </router-link>

    <nav class="nav">
      <template v-for="(group, groupIndex) in navGroups" :key="groupIndex">
        <div v-if="group.label" class="nav-label">{{ group.label }}</div>
        <router-link
          v-for="item in group.items"
          :key="item.to"
          :to="item.to"
          class="nav-item"
          :class="{ active: isActive(item) }"
        >
          <i :class="isActive(item) ? item.active : item.icon"></i>
          <span>{{ item.label }}</span>
        </router-link>
      </template>
    </nav>

    <div class="sidebar-account">
      <template v-if="auth.state.user">
        <span class="account-avatar">{{ auth.state.user.username.slice(0, 1).toUpperCase() }}</span>
        <div class="account-info">
          <strong>{{ auth.state.user.username }}</strong>
          <small>{{ auth.state.user.role === 'admin' ? '管理员' : '普通用户' }}</small>
        </div>
        <button class="account-action" title="退出登录" @click="logout">
          <i class="ri-logout-box-r-line"></i>
        </button>
      </template>
      <router-link v-else to="/login" class="account-login">
        <i class="ri-user-line"></i> 登录
      </router-link>
    </div>

    <div class="sidebar-footer">
      <span class="status-dot" :class="{ online }"></span>
      <span>{{ online ? '服务已连接' : '服务未连接' }}</span>
      <small v-if="meta.version && meta.version.version">{{ meta.version.version }}</small>
    </div>
  </aside>
</template>
