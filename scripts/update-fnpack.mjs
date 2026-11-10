import { createHash } from 'node:crypto'
import { existsSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const version = String(process.env.VERSION || '').replace(/^v/, '')
const tag = String(process.env.TAG || `v${version}`)
const repository = String(process.env.REPOSITORY || '')
const assetsDir = String(process.env.ASSETS_DIR || 'dist/fnos')

if (!version || !repository) {
  console.error('VERSION and REPOSITORY are required')
  process.exit(1)
}

const manifestPath = 'fnpack.json'
let manifest = {}
if (existsSync(manifestPath)) {
  manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
}

manifest.schema_version = '2'
manifest.source_info = {
  name: 'MusicDL Ex',
  author: 'ifeng369',
  description: 'MusicDL Ex 飞牛应用源，提供 x86 / arm FPK 安装包与内置 MusicFree / Mint-Music 插件。',
  homepage: `https://github.com/${repository}`,
  ...(manifest.source_info || {}),
}

manifest.apps = manifest.apps || {}
const app = { ...(manifest.apps['musicdl-ex'] || {}) }
app.display_name ||= 'MusicDL Ex'
app.desc ||= '多平台音乐聚合播放器，支持 MusicFree 与 Mint-Music 插件。'
app.platform ||= ['x86', 'arm']
app.categories ||= ['影音娱乐']
app.icon_url ||= `https://raw.githubusercontent.com/${repository}/main/packaging/fnos/ICON_256.PNG`
app.readme_url ||= `https://raw.githubusercontent.com/${repository}/main/README.md`
app.bug_report_url ||= `https://github.com/${repository}/issues`
app.maintainer ||= 'ifeng369'
app.maintainer_url ||= 'https://github.com/ifeng369'
app.run_as ||= 'package'
app.install_type ??= ''
app.is_docker ??= false
app.service_port ||= '18080'

function packageFor(fileName) {
  const path = join(assetsDir, fileName)
  if (!existsSync(path)) {
    throw new Error(`missing FPK asset: ${path}`)
  }
  const data = readFileSync(path)
  return {
    download_url: `https://github.com/${repository}/releases/download/${tag}/${fileName}`,
    sha256: createHash('sha256').update(data).digest('hex'),
    size: statSync(path).size,
  }
}

const amd64Name = `musicdl-ex-${version}-fnos-amd64.fpk`
const arm64Name = `musicdl-ex-${version}-fnos-arm64.fpk`
const release = {
  changelog: process.env.CHANGELOG || `MusicDL Ex ${version}`,
  updated_at: new Date().toISOString(),
  os_min_version: '1.1.8',
  packages: {
    x86: packageFor(amd64Name),
    arm: packageFor(arm64Name),
  },
}

const previous = app.releases || {}
app.releases = {
  [version]: release,
  ...Object.fromEntries(Object.entries(previous).filter(([key]) => key !== version)),
}
manifest.apps['musicdl-ex'] = app

writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`)
console.log(`fnpack.json updated for ${version}`)
