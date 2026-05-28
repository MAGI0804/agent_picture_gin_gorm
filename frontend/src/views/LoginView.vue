<template>
  <main class="login-page">
    <header class="auth-brandbar">
      <img class="brand-logo auth-logo-image" src="/logo.jpg" alt="平台 Logo" />
    </header>

    <section class="login-card auth-card">
      <div class="auth-heading">
        <h1>欢迎回来</h1>
        <p>登录您的账号以继续使用本平台</p>
      </div>

      <div class="auth-tabs">
        <button class="active" type="button">账号登录</button>
      </div>

      <label class="field-with-icon">
        <span>账号</span>
        <input v-model="account" placeholder="请输入账号" />
      </label>

      <label class="field-with-icon">
        <span>密码</span>
        <input v-model="password" placeholder="请输入密码" type="password" />
      </label>

      <div class="auth-options">
        <label class="check-line">
          <input type="checkbox" />
          <span>记住我</span>
        </label>
        <button class="text-button" type="button">忘记密码?</button>
      </div>

      <button class="primary" @click="login">登录</button>
      <p v-if="authHint" class="hint">{{ authHint }}</p>
      <p class="auth-switch">还没有账号？<button class="link-button" @click="router.push('/register')">立即注册</button></p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, setCurrentUser, setToken } from '../api'
import type { UserProfile } from '../types'

interface LoginResponse {
  token: string
  user: UserProfile
}

const router = useRouter()
const account = ref('')
const password = ref('')
const authHint = ref('')

async function login() {
  authHint.value = '正在登录...'
  try {
    if (!account.value.trim() || !password.value) {
      authHint.value = '请输入账号和密码'
      return
    }

    const identity = account.value.includes('@') ? { email: account.value } : { account: account.value }
    const data = await apiFetch<LoginResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ ...identity, password: password.value })
    })
    setToken(data.token)
    setCurrentUser(data.user)
    authHint.value = '登录成功，正在进入 V2 工作台...'
    const redirect = typeof router.currentRoute.value.query.redirect === 'string'
      ? router.currentRoute.value.query.redirect
      : '/workspace'
    await router.push(redirect)
  } catch (error) {
    authHint.value = error instanceof Error ? error.message : '登录失败，请稍后重试'
  }
}
</script>
