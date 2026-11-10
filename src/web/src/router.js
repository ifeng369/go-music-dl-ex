import { createRouter, createWebHistory } from 'vue-router'
import { auth } from './auth'

const routes = [
  { path: '/', name: 'home', component: () => import('./views/HomeView.vue') },
  { path: '/search', name: 'search', component: () => import('./views/SearchView.vue') },
  { path: '/sheet/:source/:id', name: 'sheet', component: () => import('./views/SheetView.vue') },
  { path: '/toplist', name: 'toplist', component: () => import('./views/TopListView.vue') },
  { path: '/artist/:source/:name', name: 'artist', component: () => import('./views/ArtistView.vue') },
  { path: '/category/:source', name: 'category', component: () => import('./views/CategoryView.vue') },
  { path: '/local', name: 'local', component: () => import('./views/LocalMusicView.vue'), meta: { requiresAuth: true } },
  { path: '/my', name: 'my-playlists', component: () => import('./views/MyPlaylistsView.vue'), meta: { requiresAuth: true } },
  { path: '/my/:id', name: 'my-playlist', component: () => import('./views/LocalPlaylistView.vue'), meta: { requiresAuth: true } },
  { path: '/favorites', name: 'favorites', component: () => import('./views/FavoritesView.vue'), meta: { requiresAuth: true } },
  { path: '/history', name: 'history', component: () => import('./views/HistoryView.vue'), meta: { requiresAuth: true } },
  { path: '/downloads', name: 'downloads', component: () => import('./views/DownloadsView.vue'), meta: { requiresAuth: true } },
  { path: '/login', name: 'login', component: () => import('./views/LoginView.vue') },
  { path: '/users', name: 'users', component: () => import('./views/UsersView.vue'), meta: { requiresAuth: true, requiresAdmin: true } },
  { path: '/settings', name: 'settings', component: () => import('./views/SettingsView.vue'), meta: { requiresAuth: true } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

const titles = {
  home: '发现音乐',
  search: '搜索',
  sheet: '歌单',
  toplist: '排行榜',
  artist: '歌手',
  category: '分类歌单',
  local: '本地音乐',
  'my-playlists': '我的歌单',
  'my-playlist': '歌单详情',
  favorites: '收藏音乐',
  history: '播放记录',
  downloads: '下载记录',
  login: '登录',
  users: '用户管理',
  settings: '设置',
}

router.beforeEach(async (to) => {
  if (!auth.state.loaded) await auth.loadMe()
  if (to.meta.requiresAuth && !auth.state.user) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.requiresAdmin && auth.state.user?.role !== 'admin') {
    return { name: 'home' }
  }
  if (to.name === 'login' && auth.state.user) {
    return { name: 'home' }
  }
})

router.afterEach((to) => {
  const title = titles[to.name] || '音乐库'
  document.title = `${title} · MusicDL`
})

export default router
