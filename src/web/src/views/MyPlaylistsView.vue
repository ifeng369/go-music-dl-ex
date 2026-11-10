<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import { loadMeta, meta, sourceName } from '../meta'
import SectionHeader from '../components/SectionHeader.vue'
import { library, createPlaylist, deletePlaylist, removeSheetFavorite } from '../library'
import { player } from '../player'

const router = useRouter()
const sheetFavorites = computed(() => library.sheetFavorites)
const accountGroups = ref([])
const accountErrors = ref([])
const accountLoading = ref(true)
const accountCount = computed(() => accountGroups.value.reduce((total, group) => total + group.items.length, 0))
const accountSourceNames = computed(() => accountGroups.value.map((group) => sourceName(group.source)).join('、'))

async function loadAccountPlaylists() {
  accountLoading.value = true
  accountErrors.value = []
  accountGroups.value = []
  try {
    await loadMeta()
    const cookieData = await api.cookies()
    const cookieItems = cookieData.items || {}
    const configured = Object.keys(cookieItems).filter((id) => cookieItems[id] && cookieItems[id].configured)
    const supported = configured.filter((id) => {
      const item = meta.sources.find((source) => source.id === id)
      return item && item.userPlaylists
    })
    if (!supported.length) {
      accountLoading.value = false
      return
    }
    const results = await Promise.all(
      supported.map(async (source) => {
        try {
          const data = await api.userPlaylists({ source, limit: 50 })
          return { source, items: data.items || [] }
        } catch (err) {
          return { source, error: err.message || '加载失败' }
        }
      }),
    )
    const order = meta.sources.map((item) => item.id)
    accountGroups.value = results
      .filter((group) => group.items && group.items.length)
      .sort((a, b) => order.indexOf(a.source) - order.indexOf(b.source))
    accountErrors.value = results.filter((group) => group.error)
  } catch (err) {
    accountErrors.value = [{ source: '', error: err.message || '账号歌单加载失败' }]
  } finally {
    accountLoading.value = false
  }
}

function create() {
  const name = window.prompt('新建歌单名称')
  if (name && String(name).trim()) createPlaylist(name)
}

function remove(playlist) {
  if (!window.confirm(`确定删除歌单「${playlist.name}」吗？`)) return
  deletePlaylist(playlist.id)
}

function play(playlist) {
  if (playlist.songs.length) player.playSongs(playlist.songs, 0)
}

function coverOf(playlist) {
  const song = playlist.songs.find((item) => item.artwork)
  return song ? song.artwork : ''
}

function formatDate(value) {
  if (!value) return ''
  return new Date(value).toLocaleDateString('zh-CN')
}

function typeLabel(type) {
  if (type === 'album') return '专辑'
  if (type === 'toplist') return '排行榜'
  return '歌单'
}

function sheetRoute(sheet) {
  const query = {
    type: sheet.type || 'playlist',
    title: sheet.title || '',
    cover: sheet.artwork || sheet.cover || '',
    artist: sheet.artist || '',
    desc: sheet.description || '',
    count: sheet.playCount ?? '',
    works: sheet.worksNum ?? sheet.trackCount ?? '',
    link: sheet.link || '',
  }
  Object.keys(query).forEach((key) => {
    if (query[key] === '' || query[key] === null || query[key] === undefined) delete query[key]
  })
  return { name: 'sheet', params: { source: sheet.source, id: sheet.id }, query }
}

function openSheet(sheet) {
  router.push(sheetRoute(sheet))
}

function unfavorite(sheet) {
  removeSheetFavorite(sheet)
}

function accountCover(sheet) {
  return sheet.artwork || sheet.cover || ''
}

onMounted(loadAccountPlaylists)
</script>

<template>
  <div class="library-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">我的音乐</span>
        <h1>我的歌单</h1>
        <p>账号收藏歌单、平台收藏歌单和自建歌单，都在这里。</p>
      </div>
      <button class="primary-button" @click="create">
        <i class="ri-add-line"></i> 新建歌单
      </button>
    </header>

    <div class="library-summary">
      <span><i class="ri-play-list-2-line"></i> {{ library.playlists.length }} 个自建歌单 · {{ sheetFavorites.length }} 个收藏歌单<span v-if="accountCount"> · {{ accountCount }} 个账号歌单</span></span>
      <span class="library-tip">在歌单详情页点击「收藏歌单」即可加入这里</span>
    </div>

    <section class="section account-playlists">
      <SectionHeader
        title="账号歌单"
        :subtitle="accountLoading ? '正在读取已配置账号…' : (accountCount ? `来自 ${accountSourceNames} 的收藏歌单` : '读取已配置音乐账号中的收藏歌单')"
      />

      <div v-if="accountLoading" class="playlist-grid">
        <div v-for="i in 4" :key="i" class="playlist-card account-skeleton">
          <div class="playlist-cover skeleton-line"></div>
          <div class="skeleton-line"></div>
        </div>
      </div>

      <template v-else>
        <div v-if="accountErrors.length" class="notice error compact">
          <i class="ri-error-warning-line"></i>
          {{ accountErrors.map((item) => `${item.source ? sourceName(item.source) + '：' : ''}${item.error}`).join('；') }}
          <button class="text-button" @click="loadAccountPlaylists">重试</button>
        </div>

        <div v-for="group in accountGroups" :key="group.source" class="account-group">
          <div class="account-group-head">
            <h3>{{ sourceName(group.source) }}</h3>
            <span>{{ group.items.length }} 个歌单</span>
          </div>
          <div class="playlist-grid">
            <article
              v-for="sheet in group.items"
              :key="sheet.source + sheet.id"
              class="playlist-card"
            >
              <div class="playlist-cover">
                <router-link :to="sheetRoute(sheet)">
                  <img v-if="accountCover(sheet)" :src="accountCover(sheet)" :alt="sheet.title" loading="lazy" />
                  <div v-else class="playlist-cover-fallback"><i class="ri-music-2-line"></i></div>
                </router-link>
                <button class="playlist-play" title="查看歌单" @click="openSheet(sheet)">
                  <i class="ri-arrow-right-up-line"></i>
                </button>
                <span class="playlist-count"><i class="ri-music-2-line"></i> {{ sheet.worksNum ?? sheet.trackCount ?? 0 }}</span>
              </div>
              <div class="playlist-card-body">
                <router-link :to="sheetRoute(sheet)" class="playlist-name">{{ sheet.title }}</router-link>
                <small>{{ sourceName(sheet.source) }} · {{ sheet.worksNum ?? sheet.trackCount ?? 0 }} 首</small>
              </div>
              <div class="playlist-card-actions">
                <span class="account-source-badge">{{ typeLabel(sheet.type) }}</span>
              </div>
            </article>
          </div>
        </div>

        <div v-if="!accountGroups.length && !accountErrors.length" class="empty account-empty">
          <i class="ri-user-voice-line"></i>
          <p>未检测到已配置账号的收藏歌单</p>
          <router-link to="/settings" class="text-button">去设置中配置 Cookie</router-link>
        </div>
      </template>
    </section>

    <section class="section">
      <SectionHeader
        title="收藏歌单"
        :subtitle="`${sheetFavorites.length} 个来自音乐平台的歌单`"
      />
      <div v-if="sheetFavorites.length" class="playlist-grid">
        <article
          v-for="sheet in sheetFavorites"
          :key="sheet.source + sheet.type + sheet.id"
          class="playlist-card"
        >
          <div class="playlist-cover">
            <router-link :to="sheetRoute(sheet)">
              <img v-if="sheet.artwork || sheet.cover" :src="sheet.artwork || sheet.cover" :alt="sheet.title" loading="lazy" />
              <div v-else class="playlist-cover-fallback"><i class="ri-heart-3-line"></i></div>
            </router-link>
            <button class="playlist-play" title="查看歌单" @click="openSheet(sheet)">
              <i class="ri-arrow-right-up-line"></i>
            </button>
            <span class="playlist-count"><i class="ri-bookmark-line"></i> {{ typeLabel(sheet.type) }}</span>
          </div>
          <div class="playlist-card-body">
            <router-link :to="sheetRoute(sheet)" class="playlist-name">{{ sheet.title }}</router-link>
            <small>{{ sheet.artist || sourceName(sheet.source) }}<template v-if="sheet.trackCount"> · {{ sheet.trackCount }} 首</template></small>
          </div>
          <div class="playlist-card-actions">
            <button class="icon-button" title="取消收藏" @click="unfavorite(sheet)">
              <i class="ri-bookmark-fill"></i>
            </button>
          </div>
        </article>
      </div>
      <div v-else class="empty">
        <i class="ri-bookmark-line"></i>
        <p>还没有收藏歌单，打开任意歌单详情页点击「收藏歌单」</p>
      </div>
    </section>

    <section class="section">
      <SectionHeader title="自建歌单" :subtitle="`${library.playlists.length} 个歌单`" />
      <div v-if="library.playlists.length" class="playlist-grid">
        <article v-for="playlist in library.playlists" :key="playlist.id" class="playlist-card">
          <div class="playlist-cover">
            <router-link :to="{ name: 'my-playlist', params: { id: playlist.id } }">
              <img v-if="coverOf(playlist)" :src="coverOf(playlist)" :alt="playlist.name" loading="lazy" />
              <div v-else class="playlist-cover-fallback"><i class="ri-play-list-2-line"></i></div>
            </router-link>
            <button class="playlist-play" title="播放歌单" :disabled="!playlist.songs.length" @click="play(playlist)">
              <i class="ri-play-fill"></i>
            </button>
            <span class="playlist-count"><i class="ri-music-2-line"></i> {{ playlist.songs.length }}</span>
          </div>
          <div class="playlist-card-body">
            <router-link :to="{ name: 'my-playlist', params: { id: playlist.id } }" class="playlist-name">
              {{ playlist.name }}
            </router-link>
            <small>创建于 {{ formatDate(playlist.createdAt) }}</small>
          </div>
          <div class="playlist-card-actions">
            <button class="icon-button" title="播放" :disabled="!playlist.songs.length" @click="play(playlist)">
              <i class="ri-play-circle-line"></i>
            </button>
            <button class="icon-button" title="删除歌单" @click="remove(playlist)">
              <i class="ri-delete-bin-6-line"></i>
            </button>
          </div>
        </article>
      </div>
      <div v-else class="empty">
        <i class="ri-play-list-2-line"></i>
        <p>还没有自建歌单，点击「新建歌单」开始整理音乐</p>
      </div>
    </section>
  </div>
</template>
