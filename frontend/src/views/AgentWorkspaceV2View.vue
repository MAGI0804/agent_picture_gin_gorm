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
        <div class="v2-header-actions">
          <button type="button" :disabled="!activeRunId" @click="refreshRun">刷新 Timeline</button>
          <button type="button" :disabled="!canCancelRun" @click="cancelActiveRun">取消</button>
        </div>
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

      <section v-if="activeRun?.status === 'waiting_user'" class="v2-clarification">
        <header>
          <strong>需要补充信息</strong>
          <span>Run #{{ activeRun.id }}</span>
        </header>
        <ul v-if="clarificationQuestions.length">
          <li v-for="question in clarificationQuestions" :key="question">{{ question }}</li>
        </ul>
        <p v-else class="muted">当前运行需要补充说明后才能继续。</p>
        <textarea
          v-model="clarificationAnswer"
          placeholder="补充回答后，系统会继续推进同一个 run。"
          @keydown.enter.ctrl.prevent="resumeActiveRun"
        />
        <div class="v2-actions">
          <button class="primary" type="button" :disabled="!canResumeRun" @click="resumeActiveRun">
            {{ resumingRun ? '继续中...' : '提交补充并继续' }}
          </button>
        </div>
      </section>

      <section class="v2-timeline">
        <header>
          <strong>Timeline</strong>
          <small>{{ steps.length }} steps · {{ toolInvocations.length }} tools</small>
        </header>
        <ol>
          <li v-for="step in steps" :key="step.id" :class="step.status">
            <div>
              <strong>{{ step.name }}</strong>
              <span>{{ step.status }}</span>
              <small v-if="step.attempt">attempt {{ step.attempt }}</small>
              <small v-if="step.duration_ms">{{ step.duration_ms }}ms</small>
              <small v-if="providerLabelForStep(step)">{{ providerLabelForStep(step) }}</small>
            </div>
            <p>{{ step.output || step.error_message || summarizeStep(step) }}</p>
            <p v-if="errorLabelForStep(step)" class="muted">{{ errorLabelForStep(step) }}</p>
          </li>
        </ol>
        <p v-if="!steps.length" class="muted">暂无运行记录。</p>
      </section>
    </section>

    <aside class="v2-artifacts">
      <header>
        <div>
          <strong>Artifact Board</strong>
          <span>{{ artifacts.length }} 个产物 · 按 Rank 排序</span>
        </div>
        <button type="button" :disabled="!activeConversationId" @click="loadArtifacts">刷新</button>
      </header>

      <section class="v2-upload-panel">
        <label>
          上传参考图
          <input type="file" accept="image/png,image/jpeg,image/gif" @change="handleUploadFile" />
        </label>
        <button type="button" :disabled="!canUploadArtifact" @click="uploadArtifact">
          {{ uploadingArtifact ? '上传中...' : '上传为产物' }}
        </button>
      </section>

      <section class="v2-artifact-grid">
        <button
          v-for="(artifact, index) in rankedArtifacts"
          :key="artifact.id"
          type="button"
          class="v2-artifact-item"
          :class="{
            active: artifact.id === selectedArtifact?.id,
            chosen: Boolean(artifact.selected_at),
            recommended: isRecommendedArtifact(artifact)
          }"
          @click="selectArtifact(artifact)"
        >
          <img v-if="artifact.kind === 'image' && previewUrlFor(artifact)" :src="previewUrlFor(artifact)" :alt="artifact.name" />
          <span v-else>{{ artifact.kind }}</span>
          <strong>{{ artifact.name }}</strong>
          <small>{{ artifact.mime_type }}</small>
          <div class="v2-artifact-metrics">
            <small>#{{ index + 1 }}</small>
            <small>Rank {{ formatRankScore(artifact.rank_score) }}</small>
          </div>
          <div class="v2-artifact-badges">
            <small v-if="isRecommendedArtifact(artifact)" class="v2-recommended-badge">推荐</small>
            <small v-if="artifact.selected_at" class="v2-selected-badge">已选中</small>
          </div>
        </button>
      </section>

      <section v-if="selectedArtifact" class="v2-preview">
        <div class="v2-preview-head">
          <strong>{{ selectedArtifact.name }}</strong>
          <div class="v2-preview-actions">
            <button type="button" :disabled="selectingArtifact" @click="markSelected">
              {{ selectingArtifact ? '保存中...' : '设为选中' }}
            </button>
            <button type="button" @click="downloadSelected">下载</button>
          </div>
        </div>
        <img
          v-if="selectedArtifact.kind === 'image' && previewUrlFor(selectedArtifact)"
          :src="previewUrlFor(selectedArtifact)"
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
          <small v-if="version.parent_version_id">parent v{{ version.parent_version_id }}</small>
          <small v-if="version.quality_scores">score {{ formatScore(parseQualityScores(version.quality_scores)?.overall_score) }}</small>
        </button>
      </section>

      <section v-if="selectedArtifact" class="v2-edit-panel">
        <label>
          编辑 Prompt
          <textarea
            v-model="editPrompt"
            placeholder="描述要基于当前版本继续修改的内容。"
            @keydown.enter.ctrl.prevent="editSelectedArtifact"
          />
        </label>
        <button type="button" :disabled="!canEditArtifact" @click="editSelectedArtifact">
          {{ editingArtifact ? '编辑中...' : '继续编辑' }}
        </button>
      </section>

      <section v-if="selectedArtifact" class="v2-review-panel">
        <header>
          <strong>Review / Eval</strong>
          <small>{{ reviewStatusText }}</small>
        </header>
        <div v-if="selectedQualityScores" class="v2-score-block">
          <div>
            <span>质量分</span>
            <strong>{{ formatScore(selectedQualityScores.overall_score) }}</strong>
          </div>
          <div>
            <span>Requirement</span>
            <strong>{{ formatScore(selectedQualityScores.requirement_match) }}</strong>
          </div>
          <div>
            <span>Composition</span>
            <strong>{{ formatScore(selectedQualityScores.composition_score) }}</strong>
          </div>
          <div>
            <span>Text</span>
            <strong>{{ formatScore(selectedQualityScores.text_readability) }}</strong>
          </div>
          <div>
            <span>Layout</span>
            <strong>{{ formatScore(selectedQualityScores.layout_score) }}</strong>
          </div>
          <div>
            <span>Refine</span>
            <strong>{{ selectedQualityScores.should_refine ? '需要' : '不需要' }}</strong>
          </div>
          <div>
            <span>排序分</span>
            <strong>{{ formatRankScore(selectedQualityScores.rank_score ?? selectedArtifact.rank_score) }}</strong>
          </div>
        </div>
        <ul v-if="selectedQualityScores?.issues?.length" class="v2-issue-list">
          <li v-for="issue in selectedQualityScores.issues" :key="issue">{{ issue }}</li>
        </ul>
        <p v-else-if="!selectedQualityScores" class="muted">暂无版本质量分。</p>
        <p v-if="selectedQualityScores?.extracted_text" class="muted">
          OCR: {{ selectedQualityScores.extracted_text }}
        </p>
        <details v-if="reviewStep" class="v2-step-detail">
          <summary>vision_review_agent</summary>
          <p>{{ summarizeStep(reviewStep) }}</p>
        </details>
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

      <section class="v2-memory-panel">
        <header>
          <div>
            <strong>Memory</strong>
            <span>{{ memories.length }} 条</span>
          </div>
          <button type="button" :disabled="memoryLoading || !activeConversationId" @click="loadMemories">刷新</button>
        </header>
        <label>
          Status
          <select v-model="memoryStatusFilter">
            <option value="">all</option>
            <option value="proposal">proposal</option>
            <option value="stable">stable</option>
          </select>
        </label>
        <label>
          Namespace
          <select v-model="memoryNamespace" :disabled="memoryLoading" @change="loadMemories">
            <option value="">全部</option>
            <option value="conversation">conversation</option>
            <option value="user_profile">user_profile</option>
            <option value="visual_style">visual_style</option>
            <option value="artifact_lineage">artifact_lineage</option>
            <option value="tool_experience">tool_experience</option>
            <option value="reflection">reflection</option>
          </select>
        </label>
        <ul v-if="displayedMemories.length" class="v2-memory-list">
          <li v-for="memory in displayedMemories" :key="memory.id">
            <div>
              <strong>{{ memory.namespace || memory.kind }}</strong>
              <p>{{ memory.content }}</p>
              <small>
                {{ formatConfidence(memory.confidence) }} · used {{ memory.use_count || 0 }}
                <span v-if="isMemoryProposal(memory)" class="v2-memory-proposal-badge">候选</span>
              </small>
              <small v-if="memory.source_type || memory.artifact_id">
                {{ memory.source_type || 'source' }} #{{ memory.source_id || memory.artifact_id }}
              </small>
            </div>
            <div class="v2-memory-actions">
              <button
                v-if="isMemoryProposal(memory)"
                type="button"
                :disabled="promotingMemoryId === memory.id"
                @click="promoteMemory(memory.id)"
              >
                {{ promotingMemoryId === memory.id ? '确认中...' : '确认' }}
              </button>
              <button type="button" @click="editMemory(memory)">编辑</button>
              <button type="button" @click="deleteMemory(memory.id)">删除</button>
            </div>
          </li>
        </ul>
        <p v-else class="muted">暂无记忆。</p>
      </section>
    </aside>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, downloadV2Artifact, fetchV2ArtifactPreviewURL } from '../api'
import type {
  AgentRun,
  AgentStep,
  AgentV2RunResponse,
  Artifact,
  ArtifactVersion,
  Conversation,
  ContextMemory,
  ModelSelection,
  TaskLedgerItem,
  ToolInvocation
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
const taskLedgerItems = ref<TaskLedgerItem[]>([])
const toolInvocations = ref<ToolInvocation[]>([])
const artifacts = ref<Artifact[]>([])
const selectedArtifact = ref<Artifact | null>(null)
const versions = ref<ArtifactVersion[]>([])
const selectedVersionId = ref(0)
const uploadFile = ref<File | null>(null)
const uploadingArtifact = ref(false)
const editPrompt = ref('')
const editingArtifact = ref(false)
const feedbackType = ref('selected')
const rating = ref(0)
const feedbackComment = ref('')
const feedbackSending = ref(false)
const selectingArtifact = ref(false)
const previewURLs = ref<Record<number, string>>({})
const memories = ref<ContextMemory[]>([])
const memoryNamespace = ref('')
const memoryStatusFilter = ref('')
const memoryLoading = ref(false)
const promotingMemoryId = ref(0)
const runPollTimer = ref<ReturnType<typeof window.setInterval> | null>(null)
const clarificationAnswer = ref('')
const resumingRun = ref(false)

interface QualityScores {
  overall_score?: number
  requirement_match?: number
  composition_score?: number
  text_readability?: number
  layout_score?: number
  rank_score?: number
  issues?: string[]
  should_refine?: boolean
  reviewer?: string
  reviewed_at?: number
  extracted_text?: string
}

interface StepResultSnapshot {
  output?: {
    questions?: unknown
  }
}

const activeRunId = computed(() => activeRun.value?.id || 0)
const canRun = computed(() => Boolean(prompt.value.trim() && activeConversationId.value && !running.value))
const canCancelRun = computed(() => {
  const status = activeRun.value?.status || ''
  return ['created', 'queued', 'running', 'waiting_user'].includes(status)
})
const clarificationQuestions = computed(() => extractClarificationQuestions())
const canResumeRun = computed(() => {
  return activeRun.value?.status === 'waiting_user' && Boolean(clarificationAnswer.value.trim()) && !resumingRun.value
})
const canUploadArtifact = computed(() => Boolean(activeConversationId.value && uploadFile.value && !uploadingArtifact.value))
const canEditArtifact = computed(() => Boolean(selectedArtifact.value && selectedVersionId.value && editPrompt.value.trim() && !editingArtifact.value))
const selectedVersion = computed(() => versions.value.find(item => item.id === selectedVersionId.value) || null)
const selectedQualityScores = computed(() => parseQualityScores(selectedVersion.value?.quality_scores))
const rankedArtifacts = computed(() => {
  return [...artifacts.value].sort((left, right) => {
    const rankDelta = (right.rank_score || 0) - (left.rank_score || 0)
    if (rankDelta !== 0) return rankDelta
    return right.id - left.id
  })
})
const displayedMemories = computed(() => {
  if (memoryStatusFilter.value === 'proposal') {
    return memories.value.filter(memory => isMemoryProposal(memory))
  }
  if (memoryStatusFilter.value === 'stable') {
    return memories.value.filter(memory => !isMemoryProposal(memory))
  }
  return memories.value
})
const recommendedArtifactId = computed(() => rankedArtifacts.value[0]?.id || 0)
const reviewStep = computed(() => {
  return [...steps.value].reverse().find(step => step.step_key === 'vision_review_agent' || step.name === 'vision_review_agent') || null
})
const reviewStatusText = computed(() => {
  if (selectedQualityScores.value?.reviewer) return selectedQualityScores.value.reviewer
  if (reviewStep.value) return reviewStep.value.status
  return 'pending'
})
const activeTitle = computed(() => {
  return conversations.value.find(item => item.id === activeConversationId.value)?.title || 'V2 图片工作台'
})
const runStatusText = computed(() => {
  const status = activeRun.value?.status || 'ready'
  const labels: Record<string, string> = {
    ready: '就绪',
    created: '已创建',
    queued: '排队中',
    running: '运行中',
    waiting_user: '等待补充信息',
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消'
  }
  return labels[status] || status
})

onMounted(async () => {
  await Promise.all([loadModelSelection(), loadConversations()])
})

onBeforeUnmount(() => {
  clearRunPolling()
  revokeAllPreviewURLs()
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
  taskLedgerItems.value = []
  toolInvocations.value = []
  selectedArtifact.value = null
  versions.value = []
  clarificationAnswer.value = ''
  await Promise.all([loadArtifacts(), loadMemories()])
}

async function runAgent() {
  if (!canRun.value || !activeConversationId.value) return
  running.value = true
  errorMessage.value = ''
  steps.value = []
  taskLedgerItems.value = []
  toolInvocations.value = []
  clarificationAnswer.value = ''
  try {
    const data = await apiFetch<AgentV2RunResponse>(`/api/v2/conversations/${activeConversationId.value}/runs/async`, {
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
    await applyRunResponse(data)
    if (data.agent_run?.id && ['created', 'queued', 'running'].includes(data.agent_run.status)) {
      startRunPolling(data.agent_run.id)
    }
    prompt.value = ''
    await loadConversations()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '运行失败'
  } finally {
    running.value = false
  }
}

async function applyRunResponse(data: AgentV2RunResponse) {
  activeRun.value = data.agent_run
  steps.value = data.steps || []
  taskLedgerItems.value = data.task_ledger_items || []
  toolInvocations.value = data.tool_invocations || []
  artifacts.value = data.artifacts || []
  cleanupPreviewURLs(artifacts.value)
  await preloadArtifactPreviews(artifacts.value)
  if (rankedArtifacts.value.length) {
    await selectArtifact(rankedArtifacts.value[0])
  }
  await loadMemories()
}

async function refreshRun() {
  if (!activeRunId.value) return
  const data = await apiFetch<AgentV2RunResponse>(`/api/v2/runs/${activeRunId.value}`)
  activeRun.value = data.agent_run
  steps.value = data.steps || []
  taskLedgerItems.value = data.task_ledger_items || []
  toolInvocations.value = data.tool_invocations || []
  if (isTerminalRunStatus(data.agent_run.status)) {
    clearRunPolling()
    await loadArtifacts()
    await loadMemories()
  } else if (data.agent_run.status === 'waiting_user') {
    clearRunPolling()
  }
}

async function resumeActiveRun() {
  if (!activeRunId.value || !canResumeRun.value) return
  resumingRun.value = true
  errorMessage.value = ''
  try {
    const data = await apiFetch<AgentV2RunResponse>(`/api/v2/runs/${activeRunId.value}/resume`, {
      method: 'POST',
      body: JSON.stringify({
        content: clarificationAnswer.value.trim()
      })
    })
    clarificationAnswer.value = ''
    await applyRunResponse(data)
    if (data.agent_run?.id && ['created', 'queued', 'running'].includes(data.agent_run.status)) {
      startRunPolling(data.agent_run.id)
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '继续运行失败'
  } finally {
    resumingRun.value = false
  }
}

async function cancelActiveRun() {
  if (!activeRunId.value || !canCancelRun.value) return
  errorMessage.value = ''
  try {
    const data = await apiFetch<{ agent_run: AgentRun; cancelled: boolean }>(`/api/v2/runs/${activeRunId.value}/cancel`, {
      method: 'POST'
    })
    activeRun.value = data.agent_run
    clearRunPolling()
    await refreshRun()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '取消失败'
  }
}

async function loadArtifacts() {
  if (!activeConversationId.value) return
  const data = await apiFetch<{ artifacts: Artifact[] }>(`/api/v2/conversations/${activeConversationId.value}/artifacts`)
  artifacts.value = data.artifacts || []
  cleanupPreviewURLs(artifacts.value)
  await preloadArtifactPreviews(artifacts.value)
  if (selectedArtifact.value) {
    const current = artifacts.value.find(item => item.id === selectedArtifact.value?.id)
    if (current) {
      selectedArtifact.value = current
    }
  }
  if (!selectedArtifact.value && rankedArtifacts.value.length) {
    await selectArtifact(rankedArtifacts.value[0])
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

function handleUploadFile(event: Event) {
  const input = event.target as HTMLInputElement
  uploadFile.value = input.files?.[0] || null
}

async function uploadArtifact() {
  if (!activeConversationId.value || !uploadFile.value || uploadingArtifact.value) return
  uploadingArtifact.value = true
  errorMessage.value = ''
  try {
    const form = new FormData()
    form.set('file', uploadFile.value)
    const data = await apiFetch<{ artifact: Artifact; version: ArtifactVersion }>(
      `/api/v2/conversations/${activeConversationId.value}/artifacts/upload`,
      { method: 'POST', body: form }
    )
    uploadFile.value = null
    await loadArtifacts()
    const current = artifacts.value.find(item => item.id === data.artifact.id) || data.artifact
    await selectArtifact(current)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '上传图片失败'
  } finally {
    uploadingArtifact.value = false
  }
}

async function editSelectedArtifact() {
  if (!selectedArtifact.value || !selectedVersionId.value || editingArtifact.value) return
  const promptText = editPrompt.value.trim()
  if (!promptText) return
  editingArtifact.value = true
  errorMessage.value = ''
  try {
    await apiFetch<{ version: ArtifactVersion }>(`/api/v2/artifacts/${selectedArtifact.value.id}/edit`, {
      method: 'POST',
      body: JSON.stringify({
        artifact_version_id: selectedVersionId.value,
        prompt: promptText,
        image_model_config_id: imageModelConfigId.value
      })
    })
    editPrompt.value = ''
    const artifactID = selectedArtifact.value.id
    await loadArtifacts()
    const current = artifacts.value.find(item => item.id === artifactID)
    if (current) {
      await selectArtifact(current)
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '编辑图片失败'
  } finally {
    editingArtifact.value = false
  }
}

async function markSelected() {
  if (!selectedArtifact.value || selectingArtifact.value) return
  selectingArtifact.value = true
  errorMessage.value = ''
  try {
    await apiFetch<{ selected: boolean }>(`/api/v2/artifacts/${selectedArtifact.value.id}/select`, {
      method: 'POST',
      body: JSON.stringify({
        artifact_version_id: selectedVersionId.value
      })
    })
    await loadArtifacts()
    const current = artifacts.value.find(item => item.id === selectedArtifact.value?.id)
    if (current) {
      selectedArtifact.value = current
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '选择产物失败'
  } finally {
    selectingArtifact.value = false
  }
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

async function loadMemories() {
  if (!activeConversationId.value) return
  memoryLoading.value = true
  try {
    const params = new URLSearchParams({
      conversation_id: String(activeConversationId.value),
      limit: '20'
    })
    if (memoryNamespace.value) {
      params.set('namespace', memoryNamespace.value)
    }
    const data = await apiFetch<{ memories: ContextMemory[] }>(`/api/v2/memories?${params.toString()}`)
    memories.value = data.memories || []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '记忆加载失败'
  } finally {
    memoryLoading.value = false
  }
}

async function deleteMemory(id: number) {
  await apiFetch<{ deleted: boolean }>(`/api/v2/memories/${id}`, { method: 'DELETE' })
  memories.value = memories.value.filter(item => item.id !== id)
}

async function promoteMemory(id: number) {
  if (promotingMemoryId.value) return
  promotingMemoryId.value = id
  errorMessage.value = ''
  try {
    await apiFetch<{ memory: ContextMemory; promoted: boolean }>(`/api/v2/memories/${id}/promote`, {
      method: 'POST',
      body: JSON.stringify({ confidence: 0.85 })
    })
    await loadMemories()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '记忆确认失败'
  } finally {
    promotingMemoryId.value = 0
  }
}

async function editMemory(memory: ContextMemory) {
  const nextContent = window.prompt('Memory', memory.content)
  if (nextContent === null) return
  const content = nextContent.trim()
  if (!content || content === memory.content) return
  try {
    await apiFetch<{ memory: ContextMemory }>(`/api/v2/memories/${memory.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ content })
    })
    await loadMemories()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '璁板繂缂栬緫澶辫触'
  }
}

async function preloadArtifactPreviews(items: Artifact[]) {
  await Promise.all(items.filter(item => item.kind === 'image').map(async item => {
    if (previewURLs.value[item.id]) return
    try {
      const url = await fetchV2ArtifactPreviewURL(item.id)
      previewURLs.value = { ...previewURLs.value, [item.id]: url }
    } catch {
      previewURLs.value = { ...previewURLs.value, [item.id]: '' }
    }
  }))
}

function cleanupPreviewURLs(items: Artifact[]) {
  const keep = new Set(items.map(item => item.id))
  const next = { ...previewURLs.value }
  for (const [id, url] of Object.entries(previewURLs.value)) {
    if (!keep.has(Number(id))) {
      if (url) URL.revokeObjectURL(url)
      delete next[Number(id)]
    }
  }
  previewURLs.value = next
}

function revokeAllPreviewURLs() {
  Object.values(previewURLs.value).forEach(url => {
    if (url) URL.revokeObjectURL(url)
  })
  previewURLs.value = {}
}

function previewUrlFor(artifact: Artifact) {
  return previewURLs.value[artifact.id] || ''
}

function startRunPolling(runID: number) {
  clearRunPolling()
  runPollTimer.value = window.setInterval(async () => {
    if (activeRunId.value !== runID) {
      clearRunPolling()
      return
    }
    try {
      await refreshRun()
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : '刷新运行状态失败'
      clearRunPolling()
    }
  }, 2000)
}

function clearRunPolling() {
  if (!runPollTimer.value) return
  window.clearInterval(runPollTimer.value)
  runPollTimer.value = null
}

function isTerminalRunStatus(status: string) {
  return ['completed', 'failed', 'cancelled'].includes(status)
}

function extractClarificationQuestions() {
  const requirementStep = [...steps.value]
    .reverse()
    .find(step => step.step_key === 'requirement_agent' || step.name === 'requirement_agent')
  if (!requirementStep?.output_json) return []
  try {
    const payload = JSON.parse(requirementStep.output_json) as StepResultSnapshot
    const questions = payload.output?.questions
    if (!Array.isArray(questions)) return []
    return questions.filter((item): item is string => typeof item === 'string' && Boolean(item.trim()))
  } catch {
    return []
  }
}

function parseQualityScores(raw?: string): QualityScores | null {
  if (!raw) return null
  try {
    const payload = JSON.parse(raw) as QualityScores
    return {
      overall_score: typeof payload.overall_score === 'number' ? payload.overall_score : undefined,
      requirement_match: typeof payload.requirement_match === 'number' ? payload.requirement_match : undefined,
      composition_score: typeof payload.composition_score === 'number' ? payload.composition_score : undefined,
      text_readability: typeof payload.text_readability === 'number' ? payload.text_readability : undefined,
      layout_score: typeof payload.layout_score === 'number' ? payload.layout_score : undefined,
      rank_score: typeof payload.rank_score === 'number' ? payload.rank_score : undefined,
      issues: Array.isArray(payload.issues) ? payload.issues.filter(item => typeof item === 'string') : [],
      should_refine: Boolean(payload.should_refine),
      reviewer: typeof payload.reviewer === 'string' ? payload.reviewer : '',
      reviewed_at: typeof payload.reviewed_at === 'number' ? payload.reviewed_at : undefined,
      extracted_text: typeof payload.extracted_text === 'string' ? payload.extracted_text : ''
    }
  } catch {
    return null
  }
}

function formatScore(score?: number) {
  if (typeof score !== 'number') return '-'
  return `${Math.round(score * 100)}`
}

function formatRankScore(score?: number) {
  if (typeof score !== 'number') return '-'
  if (score >= 0 && score <= 1) return `${Math.round(score * 100)}`
  return score.toFixed(2)
}

function isRecommendedArtifact(artifact: Artifact) {
  return artifact.id === recommendedArtifactId.value
}

function formatConfidence(confidence?: number) {
  if (typeof confidence !== 'number' || confidence <= 0) return 'confidence -'
  return `confidence ${Math.round(confidence * 100)}`
}

function isMemoryProposal(memory: ContextMemory) {
  return memory.kind === 'memory_proposal'
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

function toolForStep(step: AgentStep) {
  return toolInvocations.value.find(tool => tool.agent_step_id === step.id) || null
}

function ledgerForStep(step: AgentStep) {
  return taskLedgerItems.value.find(item => item.task_key === (step.step_key || step.name)) || null
}

function providerLabelForStep(step: AgentStep) {
  const tool = toolForStep(step)
  const provider = tool?.provider_name || step.provider_name
  const model = tool?.model_name || step.model_name
  if (!provider && !model) return ''
  return [provider, model].filter(Boolean).join(' / ')
}

function errorLabelForStep(step: AgentStep) {
  const tool = toolForStep(step)
  const ledger = ledgerForStep(step)
  const code = step.error_code || tool?.error_code
  const message = step.error_message || tool?.error_message || ledger?.error_message
  if (code && message) return `${code}: ${message}`
  return code || message || ''
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
