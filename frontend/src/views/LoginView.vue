<template>
  <main class="login-page">
    <section class="login-card">
      <div class="brand">
        <strong>图片 AI Agent</strong>
        <span>登录后进入多 Agent 图片与 HTML 生成工作台</span>
      </div>

      <label>
        账号或邮箱
        <input v-model="account" placeholder="请输入账号或邮箱" />
      </label>

      <label>
        密码
        <input v-model="password" placeholder="请输入密码" type="password" />
      </label>

      <button class="primary" @click="login">登录</button>
      <p v-if="authHint" class="hint">{{ authHint }}</p>
      <button class="link-button" @click="router.push('/register')">还没有账号，去注册</button>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, setToken } from '../api'

const router = useRouter()
const account = ref('')
const password = ref('')
const authHint = ref('')

async function login() {
  const identity = account.value.includes('@') ? { email: account.value } : { account: account.value }
  const data = await apiFetch<{ token: string }>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ ...identity, password: password.value })
  })
  setToken(data.token)
  authHint.value = '登录成功，正在进入对话页...'
  await router.push('/chat')
}
</script>
