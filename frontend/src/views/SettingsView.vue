<template>
  <main class="app-shell compact-shell settings-shell">
    <aside class="conversation-sidebar settings-layout-sidebar">
      <header class="sidebar-head app-brand">
        <img class="brand-logo sidebar-logo" src="/logo.jpg" alt="平台 Logo" />
      </header>

      <section class="sidebar-model-card">
        <div>
          <span>文本模型</span>
          <strong :title="selectedTextModelName">{{ selectedTextModelName }}</strong>
          <small class="model-state">在线</small>
        </div>
        <div>
          <span>图片模型</span>
          <strong :title="selectedImageModelName">{{ selectedImageModelName }}</strong>
          <small class="model-state">完成</small>
        </div>
      </section>

      <div class="sidebar-nav">
        <button type="button" @click="router.push('/chat')">
          <span class="nav-icon icon-chat" aria-hidden="true"></span>
          对话
        </button>
        <button class="active" type="button">
          <span class="nav-icon icon-settings" aria-hidden="true"></span>
          设置
        </button>
        <button type="button" @click="logout">
          <span class="nav-icon icon-logout" aria-hidden="true"></span>
          退出登录
        </button>
      </div>

      <footer class="sidebar-user-menu" @click.stop>
        <button class="sidebar-user-trigger" type="button" @click="userMenuOpen = !userMenuOpen">
          <div class="avatar">{{ avatarInitial }}</div>
          <span>{{ userDisplayName }}</span>
          <i>⌄</i>
        </button>
        <div v-if="userMenuOpen" class="user-popover">
          <button type="button" @click="userMenuOpen = false">
            <span class="nav-icon icon-settings" aria-hidden="true"></span>
            设置
          </button>
          <button type="button" @click="logout">
            <span class="nav-icon icon-logout" aria-hidden="true"></span>
            退出登录
          </button>
        </div>
      </footer>
    </aside>

    <section class="settings-page">
      <form class="settings-form" @submit.prevent="saveSelection">
        <header>
          <h1>模型设置</h1>
        </header>

        <section class="settings-subsection">
          <div class="settings-section-title">
            <h2>文本模型设置</h2>
            <div class="settings-actions">
              <button type="button" @click="loadSelection">重新加载</button>
              <button class="primary" type="submit">保存</button>
            </div>
          </div>

          <label class="settings-row">
            <span>模型名称</span>
            <select v-model.number="textModelConfigId" :disabled="!textModels.length">
              <option :value="0">未选择</option>
              <option v-for="item in textModels" :key="item.id" :value="item.id">
                {{ item.model_name }}
              </option>
            </select>
          </label>
          <label class="settings-row">
            <span>请求地址</span>
            <input :value="selectedTextModel?.request_url || '本地 / Mock 模型'" readonly />
          </label>
          <label class="settings-row">
            <span>供应商</span>
            <select :value="selectedTextProvider" disabled>
              <option>{{ selectedTextProvider }}</option>
            </select>
          </label>
          <label class="settings-row api-key-row">
            <span>API Key</span>
            <input value="••••••••••••••••••••••••••••••" readonly type="password" />
          </label>
        </section>

        <section class="settings-subsection">
          <div class="settings-section-title">
            <h2>图片模型设置</h2>
            <div class="settings-actions">
              <button type="button" @click="loadSelection">重新加载</button>
              <button class="primary" type="submit">保存</button>
            </div>
          </div>

          <label class="settings-row">
            <span>模型名称</span>
            <select v-model.number="imageModelConfigId" :disabled="!imageModels.length">
              <option :value="0">未选择</option>
              <option v-for="item in imageModels" :key="item.id" :value="item.id">
                {{ item.model_name }}
              </option>
            </select>
          </label>
          <label class="settings-row">
            <span>请求地址</span>
            <input :value="selectedImageModel?.request_url || '本地 / Mock 模型'" readonly />
          </label>
          <label class="settings-row">
            <span>供应商</span>
            <select :value="selectedImageProvider" disabled>
              <option>{{ selectedImageProvider }}</option>
            </select>
          </label>
          <label class="settings-row api-key-row">
            <span>API Key</span>
            <input value="••••••••••••••••••••••••••••••" readonly type="password" />
          </label>
        </section>

        <section class="settings-subsection system-settings">
          <h2>系统设置</h2>
          <label class="settings-row">
            <span>界面语言</span>
            <select>
              <option>简体中文</option>
            </select>
          </label>
          <label class="settings-row">
            <span>主题色</span>
            <select>
              <option>浅橙色</option>
            </select>
          </label>
        </section>

        <p v-if="settingsHint" class="hint">{{ settingsHint }}</p>
      </form>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, getCurrentUser, setCurrentUser, setToken } from '../api'
import type { GlobalModelConfig, ModelSelection, UserProfile } from '../types'

const router = useRouter()
const textModels = ref<GlobalModelConfig[]>([])
const imageModels = ref<GlobalModelConfig[]>([])
const textModelConfigId = ref(0)
const imageModelConfigId = ref(0)
const settingsHint = ref('')
const userMenuOpen = ref(false)
const currentUser = ref<UserProfile | null>(getCurrentUser())

const selectedTextModel = computed(() => textModels.value.find(item => item.id === textModelConfigId.value))
const selectedImageModel = computed(() => imageModels.value.find(item => item.id === imageModelConfigId.value))
const selectedTextModelName = computed(() => selectedTextModel.value?.model_name || '未选择')
const selectedImageModelName = computed(() => selectedImageModel.value?.model_name || '未选择')
const selectedTextProvider = computed(() => providerName(selectedTextModel.value))
const selectedImageProvider = computed(() => providerName(selectedImageModel.value))
const userDisplayName = computed(() => {
  return currentUser.value?.nickname || currentUser.value?.account || '用户'
})
const avatarInitial = computed(() => {
  return Array.from(userDisplayName.value.trim())[0]?.toUpperCase() || 'U'
})

onMounted(async () => {
  await loadCurrentUser()
  await loadSelection()
})

async function loadCurrentUser() {
  try {
    const user = await apiFetch<UserProfile>('/api/auth/me')
    currentUser.value = user
    setCurrentUser(user)
  } catch (error) {
    console.error('Load current user error:', error)
  }
}

async function loadSelection() {
  const data = await apiFetch<ModelSelection>('/api/settings/model-selection')
  textModels.value = data.text_models || []
  imageModels.value = data.image_models || []
  textModelConfigId.value = data.text_model_config_id || 0
  imageModelConfigId.value = data.image_model_config_id || 0
  settingsHint.value = '已加载当前用户的模型选择'
}

async function saveSelection() {
  settingsHint.value = '模型选择已保存'
}

function providerName(model?: GlobalModelConfig) {
  const provider = model?.config_info?.provider
  return typeof provider === 'string' && provider.trim() ? provider : 'Mock'
}

function logout() {
  setToken('')
  router.push('/login')
}
</script>
