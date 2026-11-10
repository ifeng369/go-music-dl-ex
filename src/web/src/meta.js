import { reactive } from 'vue'
import { api, sourceNames } from './api'

export const meta = reactive({
  sources: [],
  health: null,
  version: null,
  loading: false,
  error: '',
  loaded: false,
})

let pending = null

export async function loadMeta(force = false) {
  if (meta.loaded && !force) return meta
  if (pending && !force) return pending
  meta.loading = true
  meta.error = ''
  pending = (async () => {
    try {
      const [sources, health, version] = await Promise.allSettled([
        api.sources(),
        api.health(),
        api.version(),
      ])
      if (sources.status === 'fulfilled') meta.sources = sources.value.items || sources.value || []
      if (health.status === 'fulfilled') meta.health = health.value
      if (version.status === 'fulfilled') meta.version = version.value
      meta.loaded = true
    } catch (error) {
      meta.error = error.message || '无法连接服务'
    } finally {
      meta.loading = false
      pending = null
    }
  })()
  return pending
}

export function sourceName(source) {
  const found = meta.sources.find((item) => item.id === source)
  return (found && found.name) || sourceNames[source] || source
}

export function sourceSupports(source, capability) {
  const found = meta.sources.find((item) => item.id === source)
  return Boolean(found && found[capability])
}
