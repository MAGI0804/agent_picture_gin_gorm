<template>
  <main class="app-shell">
    <header class="topbar">
      <div class="brand">
        <strong>图片 AI Agent</strong>
        <span>{{ modelSummary }}</span>
      </div>
      <nav>
        <button @click="router.push('/chat')">对话页</button>
        <button class="active" @click="router.push('/settings')">设置页</button>
        <button @click="logout">退出</button>
      </nav>
    </header>

    <section class="settings-page">
      <form class="settings-form" @submit.prevent="saveModelConfig">
        <header>
          <strong>模型配置</strong>
          <span>配置会保存到后端，并绑定当前登录用户。</span>
        </header>

        <label>
          Provider
          <select v-model="modelConfig.provider" @change="applyProviderPreset">
            <option value="deepseek-anthropic">DeepSeek Anthropic 兼容</option>
            <option value="deepseek">DeepSeek OpenAI 兼容</option>
            <option value="openai">OpenAI 兼容</option>
            <option value="dashscope">通义千问 / DashScope</option>
            <option value="doubao">豆包</option>
            <option value="stable-diffusion">Stable Diffusion</option>
            <option value="mock">Mock 本地演示</option>
          </select>
        </label>

        <label>
          ANTHROPIC_AUTH_TOKEN / API Key
          <input v-model="modelConfig.anthropic_auth_token" placeholder="请输入 API Key" type="password" />
        </label>

        <label>
          ANTHROPIC_BASE_URL
          <input v-model="modelConfig.anthropic_base_url" placeholder="https://api.deepseek.com/anthropic" />
        </label>

        <label>
          ANTHROPIC_MODEL
          <input v-model="modelConfig.anthropic_model" placeholder="deepseek-v4-pro" />
        </label>

        <label>
          ANTHROPIC_DEFAULT_OPUS_MODEL
          <input v-model="modelConfig.anthropic_default_opus_model" placeholder="deepseek-v4-pro" />
        </label>

        <label>
          ANTHROPIC_DEFAULT_SONNET_MODEL
          <input v-model="modelConfig.anthropic_default_sonnet_model" placeholder="deepseek-v4-pro" />
        </label>

        <label>
          ANTHROPIC_DEFAULT_HAIKU_MODEL
          <input v-model="modelConfig.anthropic_default_haiku_model" placeholder="deepseek-v4-pro" />
        </label>

        <label>
          CLAUDE_CODE_SUBAGENT_MODEL
          <input v-model="modelConfig.claude_code_subagent_model" placeholder="deepseek-v4-pro" />
        </label>

        <label>
          CLAUDE_CODE_MAX_OUTPUT_TOKENS
          <input v-model="modelConfig.claude_code_max_output_tokens" placeholder="32000" />
        </label>

        <section class="settings-subsection">
          <strong>文本对话模型配置</strong>
          <label>
            文本对话模型
            <input v-model="modelConfig.chat_model" placeholder="deepseek-v4-pro" />
          </label>
          <label>
            文本对话 Base URL
            <input v-model="modelConfig.base_url" placeholder="https://api.deepseek.com/anthropic" />
          </label>
          <label>
            Temperature
            <input v-model="modelConfig.temperature" placeholder="0.7" />
          </label>
        </section>

        <section class="settings-subsection">
          <strong>图片生成模型配置</strong>
          <label>
            图片生成模型
            <input v-model="modelConfig.image_model" placeholder="例如 stable-diffusion-xl、dall-e-3 或供应商图片模型名" />
          </label>
          <label>
            图片模型说明
            <input value="当前后端已按用户配置保存 image_model，生成链路会独立读取图片模型字段。" disabled />
          </label>
        </section>

        <div class="settings-actions">
          <button class="primary" type="submit">保存到当前用户</button>
          <button type="button" @click="loadModelConfig">重新加载</button>
          <button type="button" @click="resetDeepSeekAnthropic">DeepSeek Anthropic 默认配置</button>
        </div>

        <p v-if="settingsHint" class="hint">{{ settingsHint }}</p>
      </form>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, setToken } from '../api'
import type { ModelConfig } from '../types'

const router = useRouter()
const defaultModelConfig: ModelConfig = {
  provider: 'deepseek-anthropic',
  chat_model: 'deepseek-v4-pro',
  image_model: '',
  base_url: 'https://api.deepseek.com/anthropic',
  api_key: '',
  temperature: '0.7',
  anthropic_auth_token: '',
  anthropic_base_url: 'https://api.deepseek.com/anthropic',
  anthropic_model: 'deepseek-v4-pro',
  anthropic_default_opus_model: 'deepseek-v4-pro',
  anthropic_default_sonnet_model: 'deepseek-v4-pro',
  anthropic_default_haiku_model: 'deepseek-v4-pro',
  claude_code_subagent_model: 'deepseek-v4-pro',
  claude_code_max_output_tokens: '32000'
}

const settingsHint = ref('')
const modelConfig = ref<ModelConfig>({ ...defaultModelConfig })
const modelSummary = computed(() => `${modelConfig.value.provider} / ${modelConfig.value.anthropic_model || modelConfig.value.chat_model}`)

onMounted(async () => {
  await loadModelConfig()
})

async function loadModelConfig() {
  const data = await apiFetch<{ model_config: ModelConfig }>('/api/settings/model-config')
  modelConfig.value = normalizeModelConfig({ ...defaultModelConfig, ...data.model_config })
  settingsHint.value = '已加载当前用户的模型配置'
}

async function saveModelConfig() {
  modelConfig.value = normalizeModelConfig(modelConfig.value)
  const data = await apiFetch<{ model_config: ModelConfig }>('/api/settings/model-config', {
    method: 'PUT',
    body: JSON.stringify(modelConfig.value)
  })
  modelConfig.value = normalizeModelConfig({ ...defaultModelConfig, ...data.model_config })
  localStorage.setItem('agent_model_config', JSON.stringify(modelConfig.value))
  settingsHint.value = '模型配置已保存并绑定当前用户'
}

function normalizeModelConfig(config: ModelConfig): ModelConfig {
  const model = config.anthropic_model || config.chat_model || 'deepseek-v4-pro'
  const baseURL = config.anthropic_base_url || config.base_url || 'https://api.deepseek.com/anthropic'
  const token = config.anthropic_auth_token || config.api_key || ''
  return {
    ...config,
    provider: config.provider || 'deepseek-anthropic',
    chat_model: config.chat_model || model,
    base_url: config.base_url || baseURL,
    api_key: config.api_key || token,
    anthropic_auth_token: token,
    anthropic_base_url: baseURL,
    anthropic_model: model,
    anthropic_default_opus_model: config.anthropic_default_opus_model || model,
    anthropic_default_sonnet_model: config.anthropic_default_sonnet_model || model,
    anthropic_default_haiku_model: config.anthropic_default_haiku_model || model,
    claude_code_subagent_model: config.claude_code_subagent_model || model,
    claude_code_max_output_tokens: config.claude_code_max_output_tokens || '32000',
    temperature: config.temperature || '0.7'
  }
}

function applyProviderPreset() {
  if (modelConfig.value.provider === 'deepseek-anthropic') {
    resetDeepSeekAnthropic()
  }
  if (modelConfig.value.provider === 'openai') {
    modelConfig.value.chat_model = modelConfig.value.chat_model || 'gpt-4.1-mini'
    modelConfig.value.base_url = modelConfig.value.base_url || 'https://api.openai.com/v1'
  }
}

function resetDeepSeekAnthropic() {
  const token = modelConfig.value.anthropic_auth_token || modelConfig.value.api_key
  modelConfig.value = { ...defaultModelConfig, anthropic_auth_token: token, api_key: token }
  settingsHint.value = '已填入 DeepSeek Anthropic 默认配置，请输入 API Key 后保存'
}

function logout() {
  setToken('')
  router.push('/login')
}
</script>
