<template>
  <main class="app-shell">
    <header class="topbar">
      <div class="brand">
        <strong>图片 AI Agent</strong>
        <span>{{ summary }}</span>
      </div>
      <nav>
        <button @click="router.push('/chat')">对话页</button>
        <button class="active" @click="router.push('/settings')">设置页</button>
        <button @click="logout">退出</button>
      </nav>
    </header>

    <section class="settings-page">
      <form class="settings-form" @submit.prevent="saveSelection">
        <header>
          <strong>模型选择</strong>
          <span>模型密钥和请求地址由全局模型目录维护，当前用户只需要选择使用哪个文本模型和图片模型。</span>
        </header>

        <section class="settings-subsection">
          <strong>文本模型</strong>
          <label>
            选择文本模型
            <select v-model.number="textModelConfigId" :disabled="!textModels.length">
              <option :value="0">未选择</option>
              <option v-for="item in textModels" :key="item.id" :value="item.id">
                {{ item.model_name }}
              </option>
            </select>
          </label>
          <div v-if="selectedTextModel" class="model-card">
            <strong>{{ selectedTextModel.model_name }}</strong>
            <span>{{ selectedTextModel.request_url || '本地 / Mock 模型' }}</span>
            <div class="tag-row">
              <span class="tag">文本</span>
              <span v-if="selectedTextModel.support_thinking" class="tag">支持思考</span>
              <span class="tag">{{ providerName(selectedTextModel) }}</span>
            </div>
          </div>
          <p v-else class="muted">暂无可选文本模型，请先在全局模型目录中添加文本模型。</p>
        </section>

        <section class="settings-subsection">
          <strong>图片模型</strong>
          <label>
            选择图片模型
            <select v-model.number="imageModelConfigId" :disabled="!imageModels.length">
              <option :value="0">未选择</option>
              <option v-for="item in imageModels" :key="item.id" :value="item.id">
                {{ item.model_name }}
              </option>
            </select>
          </label>
          <div v-if="selectedImageModel" class="model-card">
            <strong>{{ selectedImageModel.model_name }}</strong>
            <span>{{ selectedImageModel.request_url || '本地 / Mock 模型' }}</span>
            <div class="tag-row">
              <span class="tag">图片</span>
              <span v-if="selectedImageModel.support_thinking" class="tag">支持思考</span>
              <span class="tag">{{ providerName(selectedImageModel) }}</span>
            </div>
          </div>
          <p v-else class="muted">暂无可选图片模型，请先在全局模型目录中添加图片模型。</p>
        </section>

        <div class="settings-actions">
          <button class="primary" type="submit">保存选择</button>
          <button type="button" @click="loadSelection">重新加载</button>
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
import type { GlobalModelConfig, ModelSelection } from '../types'

const router = useRouter()
const textModels = ref<GlobalModelConfig[]>([])
const imageModels = ref<GlobalModelConfig[]>([])
const textModelConfigId = ref(0)
const imageModelConfigId = ref(0)
const settingsHint = ref('')

const selectedTextModel = computed(() => textModels.value.find(item => item.id === textModelConfigId.value))
const selectedImageModel = computed(() => imageModels.value.find(item => item.id === imageModelConfigId.value))
const summary = computed(() => {
  const text = selectedTextModel.value?.model_name || '未选择文本模型'
  const image = selectedImageModel.value?.model_name || '未选择图片模型'
  return `${text} / ${image}`
})

onMounted(async () => {
  await loadSelection()
})

async function loadSelection() {
  const data = await apiFetch<ModelSelection>('/api/settings/model-selection')
  textModels.value = data.text_models || []
  imageModels.value = data.image_models || []
  textModelConfigId.value = data.text_model_config_id || 0
  imageModelConfigId.value = data.image_model_config_id || 0
  settingsHint.value = '已加载当前用户的模型选择'
}

async function saveSelection() {
  const data = await apiFetch<ModelSelection>('/api/settings/model-selection', {
    method: 'PUT',
    body: JSON.stringify({
      text_model_config_id: textModelConfigId.value,
      image_model_config_id: imageModelConfigId.value
    })
  })
  textModels.value = data.text_models || []
  imageModels.value = data.image_models || []
  textModelConfigId.value = data.text_model_config_id || 0
  imageModelConfigId.value = data.image_model_config_id || 0
  settingsHint.value = '模型选择已保存'
}

function providerName(model: GlobalModelConfig) {
  const provider = model.config_info?.provider
  return typeof provider === 'string' && provider.trim() ? provider : 'mock'
}

function logout() {
  setToken('')
  router.push('/login')
}
</script>
