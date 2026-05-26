<template>
  <main class="v2-workspace">
    <aside class="v2-sidebar">
      <header class="v2-brand">
        <img class="brand-logo" src="/logo.jpg" alt="平台 Logo" />
        <strong>Agent V2</strong>
      </header>

      <nav class="v2-nav">
        <button class="active" type="button">V2 工作台</button>
        <button type="button" @click="router.push('/chat')">旧版对话</button>
        <button type="button" @click="router.push('/settings')">设置</button>
      </nav>

      <button class="primary v2-new-button" type="button" @click="createConversation">新建会话</button>

      <section class="v2-conversations">
        <h2>会话</h2>
        <button
          v-for="item in conversations"
          :key="item.id"
          type="button"
          :class="{ active: item.id === activeConversationId }"
          @click="openConversation(item.id)"
        >
          <span>{{ item.title }}</span>
          <small>{{ formatTime(item.updated_at) }}</small>
        </button>
      </section>
    </aside>

    <section class="v2-main">
      <header class="v2-header">
        <div>
          <strong>{{ activeTitle }}</strong>
          <span>{{ runStatusText }}</span>
        </div>
        <button type="button" :disabled="!activeRunId" @click="refreshRun">刷新 Timeline</button>
      </header>

      <section class="v2-composer">
        <div class="v2-model-row">
          <label>
            文本模型
            <select v-model.number="textModelConfigId">
              <option :value="0">自动选择</option>
              <option v-for="item in modelSelection?.text_models || []" :key="item.id" :value="item.id">
                {{ item.model_name }}
              </option>
            </select>
          </label>
          <label>
            图片模型
            <select v-model.number="imageModelConfigId">
              <option :value="0">自动选择</option>
              <option v-for="item in modelSelection?.image_models || []" :key="item.id" :value="item.id">
                {{ item.model_name }}
              </option>
            </select>
          </label>
          <label>
            候选数
            <select v-model.number="candidateCount">
              <option :value="1">1 张</option>
              <option :value="2">2 张</option>
              <option :value="3">3 张</option>
            </select>
          </label>
        </div>

        <label>
          图片需求
          <textarea
            v-model="prompt"
            placeholder="输入图片需求，V2 会抽取需求、生成 prompt、调用图片模型并写入 artifact version。"
            @keydown.enter.ctrl.prevent="runAgent"
          />
        </label>

        <div class="v2-actions">
          <button class="primary" type="button" :disabled="!canRun" @click="runAgent">
            {{ running ? '运行中...' : '运行 V2 Agent' }}
          </button>
          <button type="button" :disabled="running" @click="prompt = ''">清空</button>
          <span v-if="errorMessage">{{ errorMessage }}</span>
        </div>
      </section>

      <section class="v2-timeline">
        <header>
          <strong>Timeline</strong>
          <small>{{ steps.length }} steps</small>
        </header>
        <ol>
          <li v-for="step in steps" :key="step.id" :class="step.status">
            <div>
              <strong>{{ step.name }}</strong>
              <span>{{ step.status }}</span>
              <small v-if="step.duration_ms">{{ step.duration_ms }}ms</small>
            </div>
            <p>{{ step.output || step.error_message || summarizeStep(step) }}</p>
          </li>
        </ol>
        <p v-if="!steps.length" class="muted">暂无运行记录。</p>
      </section>
    </section>

    <aside class="v2-artifacts">
      <header>
        <div>
          <strong>Artifact Board</strong>
          <span>{{ artifacts.length }} 个产物</span>
        </div>
        <button type="button" :disabled="!activeConversationId" @click="loadArtifacts">刷新</button>
      </header>

      <section class="v2-artifact-grid">
        <button
          v-for="artifact in artifacts"
          :key="artifact.id"
          type="button"
          class="v2-artifact-item"
          :class="{ active: artifact.id === selectedArtifact?.id }"
          @click="selectArtifact(artifact)"
        >
          <img v-if="artifact.kind === 'image'" :src="artifact.preview_url" :alt="artifact.name" />
          <span v-else>{{ artifact.kind }}</span>
          <strong>{{ artifact.name }}</strong>
          <small>{{ artifact.mime_type }}</small>
        </button>
      </section>

      <section v-if="selectedArtifact" class="v2-preview">
        <div class="v2-preview-head">
          <strong>{{ selectedArtifact.name }}</strong>
          <button type="button" @click="downloadSelected">下载</button>
        </div>
        <img
          v-if="selectedArtifact.kind === 'image'"
          :src="selectedArtifact.preview_url"
          :alt="selectedArtifact.name"
        />
        <p v-else>{{ selectedArtifact.mime_type }}</p>
      </section>

      <section v-if="selectedArtifact" class="v2-versions">
        <header>
          <strong>版本</strong>
          <small>{{ versions.length }}</small>
        </header>
        <button
          v-for="version in versions"
          :key="version.id"
          type="button"
          :class="{ active: version.id === selectedVersionId }"
          @click="selectedVersionId = version.id"
        >
          <span>v{{ version.version_no }} · {{ version.operation }}</span>
          <small>{{ version.model_provider }}/{{ version.model_name }}</small>
        </button>
      </section>

      <section v-if="selectedArtifact" class="v2-feedback">
        <label>
          反馈
          <select v-model="feedbackType">
            <option value="selected">选中</option>
            <option value="like">满意</option>
            <option value="dislike">不满意</option>
          </select>
        </label>
        <label>
          评分
          <select v-model.number="rating">
            <option :value="0">不评分</option>
            <option v-for="score in [1, 2, 3, 4, 5]" :key="score" :value="score">{{ score }}</option>
          </select>
        </label>
        <textarea v-model="feedbackComment" placeholder="可选反馈说明" />
        <button type="button" :disabled="feedbackSending" @click="sendFeedback">
          {{ feedbackSending ? '提交中...' : '提交反馈' }}
        </button>
      </section>
    </aside>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, downloadV2Artifact } from '../api'
import type {
  AgentRun,
  AgentStep,
  AgentV2RunResponse,
  Artifact,
  ArtifactVersion,
  Conversation,
  ModelSelection
} from '../types'

const router = useRouter()
const conversations = ref<Conversation[]>([])
const activeConversationId = ref<number | null>(null)
const modelSelection = ref<ModelSelection | null>(null)
const textModelConfigId = ref(0)
const imageModelConfigId = ref(0)
const candidateCount = ref(1)
const prompt = ref('')
const running = ref(false)
const errorMessage = ref('')
const activeRun = ref<AgentRun | null>(null)
const steps = ref<AgentStep[]>([])
const artifacts = ref<Artifact[]>([])
const selectedArtifact = ref<Artifact | null>(null)
const versions = ref<ArtifactVersion[]>([])
const selectedVersionId = ref(0)
const feedbackType = ref('selected')
const rating = ref(0)
const feedbackComment = ref('')
const feedbackSending = ref(false)

const activeRunId = computed(() => activeRun.value?.id || 0)
const canRun = computed(() => Boolean(prompt.value.trim() && activeConversationId.value && !running.value))
const activeTitle = computed(() => {
  return conversations.value.find(item => item.id === activeConversationId.value)?.title || 'V2 图片工作台'
})
const runStatusText = computed(() => {
  const status = activeRun.value?.status || 'ready'
  const labels: Record<string, string> = {
    ready: '就绪',
    running: '运行中',
    completed: '已完成',
    failed: '失败'
  }
  return labels[status] || status
})

onMounted(async () => {
  await Promise.all([loadModelSelection(), loadConversations()])
})

async function loadModelSelection() {
  modelSelection.value = await apiFetch<ModelSelection>('/api/settings/model-selection')
  textModelConfigId.value = modelSelection.value.text_model_config_id || 0
  imageModelConfigId.value = modelSelection.value.image_model_config_id || 0
}

async function loadConversations() {
  const data = await apiFetch<{ conversations: Conversation[] }>('/api/conversations')
  conversations.value = data.conversations || []
  if (!activeConversationId.value && conversations.value.length) {
    await openConversation(conversations.value[0].id)
  }
  if (!activeConversationId.value) {
    await createConversation()
  }
}

async function createConversation() {
  const data = await apiFetch<{ conversation: Conversation }>('/api/conversations', {
    method: 'POST',
    body: JSON.stringify({ title: 'V2 图片 Agent 会话' })
  })
  conversations.value.unshift(data.conversation)
  await openConversation(data.conversation.id)
}

async function openConversation(id: number) {
  activeConversationId.value = id
  activeRun.value = null
  steps.value = []
  selectedArtifact.value = null
  versions.value = []
  await loadArtifacts()
}

async function runAgent() {
  if (!canRun.value || !activeConversationId.value) return
  running.value = true
  errorMessage.value = ''
  steps.value = []
  try {
    const data = await apiFetch<AgentV2RunResponse>(`/api/v2/conversations/${activeConversationId.value}/runs`, {
      method: 'POST',
      body: JSON.stringify({
        content: prompt.value.trim(),
        task_type: 'image_generation',
        text_model_config_id: textModelConfigId.value,
        image_model_config_id: imageModelConfigId.value,
        candidate_count: candidateCount.value,
        idempotency_key: `${activeConversationId.value}-${Date.now()}`
      })
    })
    applyRunResponse(data)
    prompt.value = ''
    await loadConversations()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '运行失败'
  } finally {
    running.value = false
  }
}

function applyRunResponse(data: AgentV2RunResponse) {
  activeRun.value = data.agent_run
  steps.value = data.steps || []
  artifacts.value = data.artifacts || []
  if (artifacts.value.length) {
    selectArtifact(artifacts.value[0])
  }
}

async function refreshRun() {
  if (!activeRunId.value) return
  const data = await apiFetch<{ agent_run: AgentRun; steps: AgentStep[] }>(`/api/v2/runs/${activeRunId.value}`)
  activeRun.value = data.agent_run
  steps.value = data.steps || []
  await loadArtifacts()
}

async function loadArtifacts() {
  if (!activeConversationId.value) return
  const data = await apiFetch<{ artifacts: Artifact[] }>(`/api/v2/conversations/${activeConversationId.value}/artifacts`)
  artifacts.value = data.artifacts || []
  if (!selectedArtifact.value && artifacts.value.length) {
    selectArtifact(artifacts.value[0])
  }
}

async function selectArtifact(artifact: Artifact) {
  selectedArtifact.value = artifact
  selectedVersionId.value = 0
  feedbackComment.value = ''
  const data = await apiFetch<{ versions: ArtifactVersion[] }>(`/api/v2/artifacts/${artifact.id}/versions`)
  versions.value = data.versions || []
  selectedVersionId.value = versions.value[versions.value.length - 1]?.id || 0
}

async function downloadSelected() {
  if (!selectedArtifact.value) return
  await downloadV2Artifact(selectedArtifact.value.id, selectedArtifact.value.name)
}

async function sendFeedback() {
  if (!selectedArtifact.value || feedbackSending.value) return
  feedbackSending.value = true
  try {
    await apiFetch<{ recorded: boolean }>(`/api/v2/artifacts/${selectedArtifact.value.id}/feedback`, {
      method: 'POST',
      body: JSON.stringify({
        artifact_version_id: selectedVersionId.value,
        feedback_type: feedbackType.value,
        rating: rating.value,
        comment: feedbackComment.value.trim()
      })
    })
    feedbackComment.value = ''
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '反馈提交失败'
  } finally {
    feedbackSending.value = false
  }
}

function summarizeStep(step: AgentStep) {
  if (!step.output_json) return '等待结构化输出'
  try {
    const payload = JSON.parse(step.output_json)
    return payload.summary || '已写入结构化输出'
  } catch {
    return '已写入结构化输出'
  }
}

function formatTime(timestamp?: number) {
  if (!timestamp) return ''
  return new Date(timestamp * 1000).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}
</script>
