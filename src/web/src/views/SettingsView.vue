<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import { auth } from '../auth'
import { loadMeta, meta, sourceName } from '../meta'
import { player } from '../player'

const router = useRouter()
const cookies = ref({})
const cookieForm = reactive({})
const message = ref('')
const messageType = ref('ok')
const saving = ref('')
const importing = ref(false)
const importUrl = ref('')
const importType = ref('')
const downloadDir = ref('')
const defaultDownloadDir = ref('data/downloads')
const savingDir = ref(false)

async function loadSettings() {
  if (!auth.isAdmin.value) return
  try {
    const data = await api.settings.get()
    downloadDir.value = data.downloadDir || ''
    defaultDownloadDir.value = data.defaultDownloadDir || 'data/downloads'
  } catch (err) {
    notify(err.message || '下载目录读取失败', 'error')
  }
}

async function saveDownloadDir() {
  if (!downloadDir.value.trim()) return
  savingDir.value = true
  try {
    const data = await api.settings.save(downloadDir.value.trim())
    downloadDir.value = data.downloadDir || downloadDir.value
    notify('下载目录已保存')
  } catch (err) {
    notify(err.message || '保存失败', 'error')
  } finally {
    savingDir.value = false
  }
}

const loginSources = computed(() =>
  meta.sources.filter((item) => item.requiresLogin || (cookies.value[item.id] && cookies.value[item.id].configured)),
)
const apiBase = computed(() => window.location.origin)

function notify(text, type = 'ok') {
  message.value = text
  messageType.value = type
  window.clearTimeout(notify.timer)
  notify.timer = window.setTimeout(() => (message.value = ''), 3200)
}

async function loadCookies() {
  try {
    const data = await api.cookies()
    cookies.value = data.items || {}
  } catch (error) {
    notify(error.message, 'error')
  }
}

async function saveCookie(source) {
  const value = (cookieForm[source] || '').trim()
  if (!value) {
    notify('请先填写 Cookie', 'error')
    return
  }
  saving.value = source
  try {
    await api.saveCookie({ source, cookie: value })
    cookieForm[source] = ''
    await loadCookies()
    notify(`${sourceName(source)} Cookie 已保存`)
  } catch (error) {
    notify(error.message, 'error')
  } finally {
    saving.value = ''
  }
}

async function clearCookie(source) {
  saving.value = source
  try {
    await api.deleteCookie(source)
    await loadCookies()
    notify(`${sourceName(source)} Cookie 已清除`)
  } catch (error) {
    notify(error.message, 'error')
  } finally {
    saving.value = ''
  }
}

async function doImport() {
  if (!importUrl.value.trim()) return
  importing.value = true
  try {
    const result = await api.importUrl({ url: importUrl.value.trim(), type: importType.value || undefined })
    if (result.type === 'music' && result.music) {
      player.playSongs([result.music], 0)
      notify(`已解析并开始播放：${result.music.title}`)
    } else if (result.sheet) {
      router.push({
        name: 'sheet',
        params: { source: result.source, id: result.sheet.sourceId || result.sheet.id },
        query: {
          type: result.type === 'album' ? 'album' : 'playlist',
          title: result.sheet.title,
          cover: result.sheet.artwork || '',
          artist: result.sheet.artist || '',
          desc: result.sheet.description || '',
        },
      })
      notify('链接解析成功')
    } else {
      notify('链接解析成功，但没有返回内容', 'error')
    }
  } catch (error) {
    notify(error.message || '解析失败', 'error')
  } finally {
    importing.value = false
  }
}

async function copyBase() {
  try {
    await navigator.clipboard.writeText(apiBase.value)
    notify('接口地址已复制')
  } catch {
    notify('复制失败，请手动选择地址', 'error')
  }
}

onMounted(async () => {
  await loadMeta(true)
  loadCookies()
  loadSettings()
})
</script>

<template>
  <div class="settings-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">设置</span>
        <h1>服务与音源</h1>
        <p>配置登录 Cookie、导入链接，并查看 MusicFree 接口信息</p>
      </div>
      <button class="ghost-button" @click="loadMeta(true)"><i class="ri-refresh-line"></i> 刷新状态</button>
    </header>

    <div v-if="message" class="notice" :class="messageType">
      <i :class="messageType === 'error' ? 'ri-error-warning-line' : 'ri-checkbox-circle-line'"></i>
      {{ message }}
    </div>

    <section class="settings-card">
      <div class="card-title">
        <i class="ri-server-line"></i>
        <div>
          <h2>服务状态</h2>
          <p>Go 服务同时提供 Web 界面与 MusicFree JSON 接口</p>
        </div>
      </div>
      <div class="stat-grid">
        <div class="stat">
          <span>状态</span>
          <strong :class="{ ok: meta.health && meta.health.status === 'ok' }">
            {{ meta.health ? meta.health.status : '未连接' }}
          </strong>
        </div>
        <div class="stat">
          <span>版本</span>
          <strong>{{ (meta.version && meta.version.version) || '—' }}</strong>
        </div>
        <div class="stat">
          <span>核心版本</span>
          <strong>{{ (meta.health && meta.health.core) || '—' }}</strong>
        </div>
        <div class="stat">
          <span>音源数量</span>
          <strong>{{ meta.sources.length }}</strong>
        </div>
      </div>
      <div class="api-row">
        <code>{{ apiBase }}/api</code>
        <button class="ghost-button small" @click="copyBase"><i class="ri-file-copy-line"></i> 复制接口地址</button>
      </div>
      <p class="hint">
        在 MusicFree 中安装 <code>plugin.js</code> 后，将插件里的「API 地址」改成
        <code>{{ apiBase }}</code>，即可搜索与播放本服务的全部音源。
      </p>
    </section>

    <section v-if="auth.isAdmin.value" class="settings-card">
      <div class="card-title">
        <i class="ri-folder-download-line"></i>
        <div>
          <h2>下载目录</h2>
          <p>下载文件与本地音乐按用户名分文件夹保存</p>
        </div>
      </div>
      <div class="import-row">
        <input v-model="downloadDir" type="text" placeholder="例如 /vol1/1000/music" />
        <button class="primary-button" :disabled="savingDir || !downloadDir.trim()" @click="saveDownloadDir">
          <i :class="savingDir ? 'ri-loader-4-line spin' : 'ri-save-line'"></i>
          {{ savingDir ? '保存中…' : '保存目录' }}
        </button>
      </div>
      <p class="hint">
        每个用户使用 <code>&lt;下载目录&gt;/&lt;用户名&gt;</code>，本地音乐位于其下的
        <code>local/</code>。默认 <code>{{ defaultDownloadDir }}</code>。
      </p>
    </section>

    <section class="settings-card">
      <div class="card-title">
        <i class="ri-link-m"></i>
        <div>
          <h2>链接导入</h2>
          <p>粘贴网易云、QQ音乐、酷狗等平台的歌曲 / 歌单 / 专辑分享链接</p>
        </div>
      </div>
      <div class="import-row">
        <input v-model="importUrl" type="url" placeholder="https://music.163.com/song?id=..." @keyup.enter="doImport" />
        <select v-model="importType">
          <option value="">自动识别</option>
          <option value="music">歌曲</option>
          <option value="sheet">歌单</option>
          <option value="album">专辑</option>
        </select>
        <button class="primary-button" :disabled="importing || !importUrl.trim()" @click="doImport">
          <i :class="importing ? 'ri-loader-4-line spin' : 'ri-download-2-line'"></i>
          {{ importing ? '解析中…' : '解析' }}
        </button>
      </div>
    </section>

    <section class="settings-card">
      <div class="card-title">
        <i class="ri-key-2-line"></i>
        <div>
          <h2>登录 Cookie</h2>
          <p>部分平台需要 Cookie 才能搜索高音质或访问个人歌单，Cookie 仅保存在本机</p>
        </div>
      </div>
      <div class="cookie-list">
        <div v-for="source in loginSources" :key="source.id" class="cookie-row">
          <div class="cookie-head">
            <span class="source-avatar small">{{ sourceName(source.id).slice(0, 1) }}</span>
            <div class="cookie-name">
              <strong>{{ sourceName(source.id) }}</strong>
              <small :class="{ configured: cookies[source.id] && cookies[source.id].configured }">
                {{ cookies[source.id] && cookies[source.id].configured ? `已配置 · ${cookies[source.id].preview}` : '未配置' }}
              </small>
            </div>
          </div>
          <div class="cookie-controls">
            <input v-model="cookieForm[source.id]" type="password" :placeholder="`粘贴 ${sourceName(source.id)} Cookie`" />
            <button class="ghost-button small" :disabled="saving === source.id" @click="saveCookie(source.id)">
              保存
            </button>
            <button
              v-if="cookies[source.id] && cookies[source.id].configured"
              class="text-button danger"
              :disabled="saving === source.id"
              @click="clearCookie(source.id)"
            >
              清除
            </button>
          </div>
        </div>
        <div v-if="!loginSources.length" class="empty">暂无需要登录的音源</div>
      </div>
    </section>

    <section class="settings-card">
      <div class="card-title">
        <i class="ri-plug-2-line"></i>
        <div>
          <h2>MusicFree 插件</h2>
          <p>插件与 Web 端共用同一套接口，支持搜索、歌单、专辑、排行榜、歌词与播放</p>
        </div>
      </div>
      <ol class="steps">
        <li>安装 MusicFree 客户端（Android / 桌面端）。</li>
        <li>在插件管理中选择「从本地安装」，导入项目 Release 中的 <code>plugin.js</code>。</li>
        <li>打开插件设置，将 API 地址填写为 <code>{{ apiBase }}</code>。</li>
        <li>回到首页即可搜索各平台音乐。</li>
      </ol>
    </section>

    <section class="settings-card">
      <div class="card-title">
        <i class="ri-database-2-line"></i>
        <div>
          <h2>音源能力</h2>
          <p>接口会按能力自动过滤每个平台的搜索、歌单与榜单请求</p>
        </div>
      </div>
      <div class="capability-table">
        <div class="capability-row head">
          <span>音源</span><span>搜索</span><span>歌单</span><span>专辑</span><span>榜单</span><span>分类</span><span>推荐</span>
        </div>
        <div v-for="source in meta.sources" :key="source.id" class="capability-row">
          <span>{{ sourceName(source.id) }}</span>
          <span><i :class="source.search ? 'ri-check-line yes' : 'ri-subtract-line no'"></i></span>
          <span><i :class="source.sheet ? 'ri-check-line yes' : 'ri-subtract-line no'"></i></span>
          <span><i :class="source.album ? 'ri-check-line yes' : 'ri-subtract-line no'"></i></span>
          <span><i :class="source.toplist ? 'ri-check-line yes' : 'ri-subtract-line no'"></i></span>
          <span><i :class="source.categories ? 'ri-check-line yes' : 'ri-subtract-line no'"></i></span>
          <span><i :class="source.recommend ? 'ri-check-line yes' : 'ri-subtract-line no'"></i></span>
        </div>
      </div>
    </section>
  </div>
</template>
