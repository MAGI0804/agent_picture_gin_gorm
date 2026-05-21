<template>
  <main class="login-page">
    <section class="login-card">
      <div class="brand">
        <strong>注册账号</strong>
        <span>完成邮箱验证码注册后自动进入 Agent 对话页</span>
      </div>

      <label>
        账号
        <input v-model="form.account" placeholder="3-20 位英文或数字" />
      </label>

      <label>
        邮箱
        <input v-model="form.email" placeholder="用于接收注册验证码" />
      </label>

      <div class="code-row">
        <label>
          邮箱验证码
          <input v-model="form.verify_code" placeholder="6 位数字验证码" />
        </label>
        <button :disabled="!canSendCode || countdown > 0" @click="sendVerifyCode">
          {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
        </button>
      </div>

      <label>
        密码
        <input v-model="form.password" placeholder="至少 6 位" type="password" />
      </label>

      <label>
        确认密码
        <input v-model="form.password_confirm" placeholder="再次输入密码" type="password" />
      </label>

      <button class="primary" :disabled="!canRegister" @click="registerUsingEmail">注册并进入</button>
      <p v-if="hint" class="hint">{{ hint }}</p>
      <button class="link-button" @click="router.push('/login')">已有账号，返回登录</button>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, setToken } from '../api'

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
  await apiFetch<{ email: string }>('/api/auth/register/email-verify-code', {
    method: 'POST',
    body: JSON.stringify({ email: form.value.email })
  })
  hint.value = '验证码已发送，请检查邮箱'
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
  const data = await apiFetch<{ token: string }>('/api/auth/register/using-email', {
    method: 'POST',
    body: JSON.stringify(form.value)
  })
  setToken(data.token)
  hint.value = '注册成功，正在进入对话页...'
  await router.push('/chat')
}
</script>
