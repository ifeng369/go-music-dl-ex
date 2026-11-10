import { computed, reactive } from 'vue'
import { api, getAuthToken, setAuthToken } from './api'

const state = reactive({
  user: null,
  token: getAuthToken(),
  loading: true,
  loaded: false,
  error: '',
})

let pending = null

async function loadMe(force = false) {
  if (state.loaded && !force) return state.user
  if (pending && !force) return pending
  state.loading = true
  state.error = ''
  pending = (async () => {
    try {
      const data = await api.auth.me()
      state.user = data.user || null
      if (data.user) setAuthToken(state.token || getAuthToken())
    } catch {
      state.user = null
      setAuthToken('')
      state.token = ''
    } finally {
      state.loading = false
      state.loaded = true
      pending = null
    }
    return state.user
  })()
  return pending
}

async function login(username, password) {
  state.loading = true
  state.error = ''
  try {
    const data = await api.auth.login(username, password)
    setAuthToken(data.token || '')
    state.token = data.token || ''
    state.user = data.user || null
    state.loaded = true
    return state.user
  } catch (err) {
    state.error = err.message || '登录失败'
    throw err
  } finally {
    state.loading = false
  }
}

async function logout() {
  try {
    await api.auth.logout()
  } catch {
    // The local session is cleared even if the server is unreachable.
  }
  setAuthToken('')
  state.token = ''
  state.user = null
  state.loaded = true
  state.error = ''
}

const isAdmin = computed(() => Boolean(state.user && state.user.role === 'admin'))

export const auth = {
  state,
  isAdmin,
  loadMe,
  login,
  logout,
}
