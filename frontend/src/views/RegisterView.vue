<template>
  <main class="login-page register-page">
    <header class="auth-brandbar">
      <img class="brand-logo auth-logo-image" src="/logo.jpg" alt="平台 Logo" />
    </header>

    <section class="login-card auth-card register-card">
      <div class="auth-heading">
        <h1>创建新账号</h1>
        <p>注册新账号以使用本平台的全部功能</p>
      </div>

      <label>
        <span>账号</span>
        <small>支持 6-20 位字符、字母、数字或下划线</small>
        <input v-model="form.account" placeholder="请输入账号" />
      </label>

      <label>
        <span>邮箱</span>
        <input v-model="form.email" placeholder="请输入邮箱" />
      </label>

      <div class="code-row">
        <label>
          <span>验证码</span>
          <input v-model="form.verify_code" placeholder="请输入验证码" />
        </label>
        <button type="button" :disabled="!canSendCode || countdown > 0" @click="sendVerifyCode">
          {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
        </button>
      </div>

      <label>
        <span>密码</span>
        <input v-model="form.password" placeholder="请输入密码（6-20位，包含字母和数字）" type="password" />
      </label>

      <label>
        <span>确认密码</span>
        <input v-model="form.password_confirm" placeholder="请再次输入密码" type="password" />
      </label>

      <label class="check-line agreement-line">
        <input type="checkbox" />
        <span>我已阅读并同意 <b>《用户协议》</b> 和 <b>《隐私政策》</b></span>
      </label>

      <button class="primary" :disabled="!canRegister" @click="registerUsingEmail">注册</button>
      <p v-if="hint" class="hint">{{ hint }}</p>
      <p class="auth-switch">已有账号？<button class="link-button" @click="router.push('/login')">立即登录</button></p>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, setCurrentUser, setToken } from '../api'
import type { UserProfile } from '../types'

const router = useRouter()
const hint = ref('')
const countdown = ref(0)
let timer: number | undefined

const form = ref({
  account: '',
  email: '',
  verify_code: '',
  password: '',
  password_confirm: ''
})

const canSendCode = computed(() => Boolean(form.value.email.trim()))
const canRegister = computed(() => Boolean(
  form.value.account.trim()
  && form.value.email.trim()
  && form.value.verify_code.trim()
  && form.value.password
  && form.value.password_confirm
))

async function sendVerifyCode() {
  hint.value = '正在发送验证码...'
  try {
    await apiFetch<{ email: string }>('/api/auth/register/email-verify-code', {
      method: 'POST',
      body: JSON.stringify({ email: form.value.email })
    })
    hint.value = '验证码已发送，请检查邮箱'
  } catch (error) {
    hint.value = error instanceof Error ? error.message : '验证码发送失败，请稍后重试'
    return
  }
  countdown.value = 60
  if (timer) {
    window.clearInterval(timer)
  }
  timer = window.setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0 && timer) {
      window.clearInterval(timer)
      timer = undefined
    }
  }, 1000)
}

async function registerUsingEmail() {
  hint.value = '正在注册...'
  try {
    const data = await apiFetch<{ token: string }>('/api/auth/register/using-email', {
      method: 'POST',
      body: JSON.stringify(form.value)
    })
    setToken(data.token)
    const user = await apiFetch<UserProfile>('/api/auth/me')
    setCurrentUser(user)
    hint.value = '注册成功，正在进入对话页...'
    await router.push('/chat')
  } catch (error) {
    hint.value = error instanceof Error ? error.message : '注册失败，请稍后重试'
  }
}
</script>
