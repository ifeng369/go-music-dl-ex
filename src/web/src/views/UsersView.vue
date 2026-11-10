<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { auth } from '../auth'

const users = ref([])
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const message = ref('')
const form = reactive({ username: '', password: '', role: 'user' })

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.users.list()
    users.value = data.items || []
  } catch (err) {
    error.value = err.message || '用户列表加载失败'
  } finally {
    loading.value = false
  }
}

async function createUser() {
  if (!form.username.trim()) {
    error.value = '请输入用户名'
    return
  }
  if (form.password.length < 6) {
    error.value = '密码至少 6 位'
    return
  }
  submitting.value = true
  error.value = ''
  message.value = ''
  try {
    await api.users.create({
      username: form.username.trim(),
      password: form.password,
      role: form.role,
    })
    message.value = `用户 ${form.username.trim()} 已创建`
    form.username = ''
    form.password = ''
    form.role = 'user'
    await load()
  } catch (err) {
    error.value = err.message || '创建用户失败'
  } finally {
    submitting.value = false
  }
}

async function removeUser(user) {
  if (!window.confirm(`确定删除用户「${user.username}」吗？该用户的我的音乐也会被删除。`)) return
  error.value = ''
  message.value = ''
  try {
    await api.users.remove(user.id)
    message.value = `用户 ${user.username} 已删除`
    await load()
  } catch (err) {
    error.value = err.message || '删除用户失败'
  }
}

async function resetPassword(user) {
  const password = window.prompt(`为「${user.username}」设置新密码`, '')
  if (password === null) return
  if (password.length < 6) {
    error.value = '密码至少 6 位'
    return
  }
  error.value = ''
  message.value = ''
  try {
    await api.users.updatePassword(user.id, password)
    message.value = `用户 ${user.username} 的密码已更新`
  } catch (err) {
    error.value = err.message || '修改密码失败'
  }
}

function formatDate(value) {
  if (!value) return '—'
  return new Date(value).toLocaleString('zh-CN')
}

onMounted(load)
</script>

<template>
  <div class="library-view users-view">
    <header class="page-head">
      <div>
        <span class="eyebrow">管理员</span>
        <h1>用户管理</h1>
        <p>新增用户并设置密码。每个用户拥有独立的我的音乐、本地音乐和下载目录。</p>
      </div>
      <button class="ghost-button" :disabled="loading" @click="load">
        <i class="ri-refresh-line"></i> 刷新
      </button>
    </header>

    <div v-if="error" class="notice error compact"><i class="ri-error-warning-line"></i>{{ error }}</div>
    <div v-if="message" class="notice compact"><i class="ri-check-line"></i>{{ message }}</div>

    <section class="settings-card create-user-card">
      <div class="settings-card-head">
        <div>
          <h2>新增用户</h2>
          <p>用户名 3-32 位，密码至少 6 位。</p>
        </div>
      </div>
      <form class="create-user-form" @submit.prevent="createUser">
        <label class="field">
          <span>用户名</span>
          <input v-model="form.username" type="text" autocomplete="off" placeholder="例如 alice" />
        </label>
        <label class="field">
          <span>密码</span>
          <input v-model="form.password" type="password" autocomplete="new-password" placeholder="至少 6 位" />
        </label>
        <label class="field">
          <span>角色</span>
          <select v-model="form.role">
            <option value="user">普通用户</option>
            <option value="admin">管理员</option>
          </select>
        </label>
        <button class="primary-button" type="submit" :disabled="submitting">
          <i :class="submitting ? 'ri-loader-4-line spin' : 'ri-user-add-line'"></i>
          {{ submitting ? '创建中…' : '创建用户' }}
        </button>
      </form>
    </section>

    <section class="settings-card">
      <div class="settings-card-head">
        <div>
          <h2>全部用户</h2>
          <p>{{ loading ? '正在加载…' : `${users.length} 个账号` }}</p>
        </div>
      </div>
      <div v-if="loading" class="user-list">
        <div v-for="i in 3" :key="i" class="user-row skeleton-row"><span class="skeleton-line"></span></div>
      </div>
      <div v-else class="user-list">
        <div v-for="user in users" :key="user.id" class="user-row">
          <span class="user-avatar">{{ user.username.slice(0, 1).toUpperCase() }}</span>
          <div class="user-meta">
            <strong>
              {{ user.username }}
              <span v-if="user.id === auth.state.user?.id" class="self-badge">当前账号</span>
            </strong>
            <small>创建于 {{ formatDate(user.createdAt) }}</small>
          </div>
          <span class="role-badge" :class="user.role">{{ user.role === 'admin' ? '管理员' : '普通用户' }}</span>
          <div class="user-actions">
            <button class="icon-button" title="重置密码" @click="resetPassword(user)">
              <i class="ri-lock-password-line"></i>
            </button>
            <button
              class="icon-button danger-text"
              title="删除用户"
              :disabled="user.id === auth.state.user?.id"
              @click="removeUser(user)"
            >
              <i class="ri-delete-bin-6-line"></i>
            </button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
