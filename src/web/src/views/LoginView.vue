<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { auth } from '../auth'

const route = useRoute()
const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const submitting = ref(false)

async function submit() {
  if (!username.value.trim() || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }
  submitting.value = true
  error.value = ''
  try {
    await auth.login(username.value.trim(), password.value)
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/')
      ? route.query.redirect
      : '/'
    router.replace(redirect)
  } catch (err) {
    error.value = err.message || '登录失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-brand">
        <span class="brand-mark"><i class="ri-music-2-fill"></i></span>
        <div>
          <strong>MusicDL</strong>
          <small>开源音乐聚合</small>
        </div>
      </div>
      <h1>登录音乐库</h1>
      <p class="login-desc">登录后使用我的音乐、本地音乐，并让 MusicFree 插件只读取你自己的内容。</p>

      <form class="login-form" @submit.prevent="submit">
        <label>
          <span>用户名</span>
          <input v-model="username" autocomplete="username" placeholder="请输入用户名" autofocus />
        </label>
        <label>
          <span>密码</span>
          <input v-model="password" type="password" autocomplete="current-password" placeholder="请输入密码" />
        </label>
        <div v-if="error" class="notice error compact">
          <i class="ri-error-warning-line"></i>{{ error }}
        </div>
        <button class="primary-button login-submit" type="submit" :disabled="submitting">
          <i :class="submitting ? 'ri-loader-4-line spin' : 'ri-login-box-line'"></i>
          {{ submitting ? '登录中…' : '登录' }}
        </button>
      </form>

      <p class="login-hint">
        首次启动默认管理员为 <code>admin</code> / <code>admin123</code>，登录后请尽快修改密码。
      </p>
      <router-link to="/" class="login-back"><i class="ri-arrow-left-line"></i> 返回发现音乐</router-link>
    </div>
  </div>
</template>
