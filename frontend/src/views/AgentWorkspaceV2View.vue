<template>
  <main class="v2-workspace" :class="{ 'v2-workspace--with-artifacts': hasArtifactPanel }">
    <aside class="v2-sidebar">
      <header class="v2-brand">
        <img class="brand-logo" src="/logo.jpg" alt="平台 Logo" />
      </header>

      <button class="v2-new-button" type="button" aria-label="新建会话" @click="createConversation">+</button>

      <section class="v2-conversations">
        <button
          v-for="item in conversations"
          :key="item.id"
          type="button"
          :class="{ active: item.id === activeConversationId }"
          @click="openConversation(item.id)"
        >
          <span class="v2-chat-dot" aria-hidden="true"></span>
          <span>{{ item.title }}</span>
        </button>
      </section>

      <footer class="v2-sidebar-footer">
        <div v-if="userMenuOpen" class="v2-sidebar-menu">
          <button type="button" @click="router.push('/settings')">
            <svg class="v2-settings-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="3"></circle>
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
            </svg>
            设置
          </button>
          <button type="button">
            <svg class="v2-logout-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
              <polyline points="16 17 21 12 16 7"></polyline>
              <line x1="21" y1="12" x2="9" y2="12"></line>
            </svg>
            退出登录
          </button>
        </div>
        <button class="v2-user-chip" type="button" @click="userMenuOpen = !userMenuOpen">
          <span class="v2-user-avatar" aria-hidden="true"></span>
          <strong>demo_user</strong>
          <span aria-hidden="true">⌃</span>
        </button>
      </footer>
    </aside>

    <section class="v2-main">
      <header class="v2-header">
        <div class="v2-header-brand">
          <img class="brand-logo" src="/logo.jpg" alt="平台 Logo" />
          <span class="v2-online-dot" aria-hidden="true"></span>
          <div>
            <strong>{{ activeTitle }}</strong>
            <span>{{ runStatusText }}</span>
          </div>
        </div>
        <div class="v2-header-actions">
          <button class="v2-icon-button v2-tune-icon" type="button" :disabled="!activeRunId" aria-label="刷新时间线" @click="refreshRun"></button>
          <button class="v2-icon-button v2-close-icon" type="button" :disabled="!canCancelRun" aria-label="取消运行" @click="cancelActiveRun"></button>
        </div>
      </header>

      <div class="v2-main-scroll">
        <section v-if="promptHistory.length" class="v2-prompt-history">
          <header>
            <strong>原提示词记录</strong>
            <span>{{ promptHistory.length }} 条</span>
          </header>
          <ol>
            <li v-for="item in promptHistory" :key="item.id">
              <time>{{ formatHistoryTime(item.createdAt) }}</time>
              <p>{{ item.content }}</p>
            </li>
          </ol>
        </section>

        <section v-if="activeRun?.status === 'waiting_user'" class="v2-clarification">
          <header>
            <strong>需要补充信息</strong>
            <span>运行 #{{ activeRun.id }}</span>
          </header>
          <ul v-if="clarificationQuestions.length">
            <li v-for="question in clarificationQuestions" :key="question">{{ question }}</li>
          </ul>
          <p v-else class="muted">当前运行需要补充说明后才能继续。</p>
          <textarea
            v-model="clarificationAnswer"
            placeholder="补充回答后，系统会继续推进同一个运行。"
            @keydown.enter.ctrl.prevent="resumeActiveRun"
          />
          <div class="v2-actions">
            <button class="primary" type="button" :disabled="!canResumeRun" @click="resumeActiveRun">
              {{ resumingRun ? '继续中...' : '提交补充并继续' }}
            </button>
          </div>
        </section>

        <TimelinePanel
          :active-run="activeRun"
          :steps="steps"
          :task-ledger-items="taskLedgerItems"
          :tool-invocations="toolInvocations"
        />
      </div>

      <WorkspaceComposer
        v-model:text-model-config-id="textModelConfigId"
        v-model:image-model-config-id="imageModelConfigId"
        v-model:candidate-count="candidateCount"
        v-model:disable-clarification="disableClarification"
        v-model:prompt="prompt"
        :model-selection="modelSelection"
        :running="running"
        :uploading="uploadingArtifact"
        :can-run="canRun"
        :can-retry="canRetryFailedRun"
        :error-message="errorMessage"
        @run="runAgent"
        @clear="prompt = ''"
        @retry="retryFailedRun"
        @upload="uploadArtifact"
      />
    </section>

    <aside v-if="hasArtifactPanel" class="v2-artifacts">
      <header>
        <div>
          <strong>产物</strong>
          <span>{{ artifacts.length }} 个结果</span>
        </div>
        <button type="button" :disabled="!activeConversationId" @click="loadArtifacts">刷新</button>
      </header>

      <ArtifactBoard
        :artifacts="rankedArtifacts"
        :selected-artifact-id="selectedArtifact?.id || 0"
        :recommended-artifact-id="recommendedArtifactId"
        :compare-ids="compareArtifactIds"
        :preview-u-r-ls="previewURLs"
        @select="selectArtifact"
        @toggle-compare="toggleCompareArtifact"
      />

      <nav v-if="selectedArtifact" class="v2-artifact-tabs" aria-label="产物面板">
        <button type="button" :class="{ active: activeArtifactTab === 'preview' }" @click="activeArtifactTab = 'preview'">预览</button>
        <button type="button" :class="{ active: activeArtifactTab === 'versions' }" @click="activeArtifactTab = 'versions'">版本</button>
        <button type="button" :class="{ active: activeArtifactTab === 'edit' }" @click="activeArtifactTab = 'edit'">编辑</button>
        <button type="button" :class="{ active: activeArtifactTab === 'review' }" @click="activeArtifactTab = 'review'">审核</button>
        <button type="button" :class="{ active: activeArtifactTab === 'memory' }" @click="activeArtifactTab = 'memory'">记忆</button>
      </nav>

      <section v-if="selectedArtifact && activeArtifactTab === 'preview'" class="v2-preview">
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
          v-if="isPreviewableArtifact(selectedArtifact) && previewUrlFor(selectedArtifact)"
          :src="previewUrlFor(selectedArtifact)"
          :alt="selectedArtifact.name"
        />
        <p v-else>{{ selectedArtifact.mime_type }}</p>
      </section>

      <VersionStrip
        v-if="selectedArtifact && activeArtifactTab === 'versions'"
        v-model:selected-version-id="selectedVersionId"
        :artifact-id="selectedArtifact.id"
        :versions="versions"
      />

      <EditPanel
        v-if="selectedArtifact && activeArtifactTab === 'edit'"
        v-model:edit-prompt="editPrompt"
        v-model:render-title="renderTitle"
        v-model:render-subtitle="renderSubtitle"
        v-model:render-brand="renderBrand"
        :artifact="selectedArtifact"
        :editing="editingArtifact"
        :can-edit="canEditArtifact"
        :rendering="renderingTextLayer"
        :can-render="canRenderTextLayer"
        @edit="editSelectedArtifact"
        @render="renderTextLayer"
      />

      <ReviewPanel
        v-if="selectedArtifact && activeArtifactTab === 'review'"
        :artifact-id="selectedArtifact.id"
        :quality-scores="selectedQualityScores"
        :review-status-text="reviewStatusText"
        :review-summary="reviewStep ? summarizeStep(reviewStep) : ''"
        :artifact-rank-score="selectedArtifact.rank_score"
      />

      <MemoryPanel
        v-if="activeArtifactTab === 'memory'"
        v-model:status-filter="memoryStatusFilter"
        :conversation-id="activeConversationId || 0"
        :memories="memories"
        :displayed-memories="displayedMemories"
        :namespace="memoryNamespace"
        :loading="memoryLoading"
        :promoting-memory-id="promotingMemoryId"
        @refresh="loadMemories"
        @namespace-change="setMemoryNamespace"
        @promote="promoteMemory"
        @edit="editMemory"
        @delete="deleteMemory"
      />
    </aside>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch } from '../api'
import { cancelAgentRun, createAgentRun, fetchAgentRun, fetchModelSelection, resumeAgentRun } from '../api/agentV2'
import {
  downloadV2Artifact,
  editArtifactVersion,
  fetchV2ArtifactPreviewURL,
  listArtifactVersions,
  listConversationArtifacts,
  recordArtifactFeedback,
  renderArtifactText,
  selectArtifactVersion,
  uploadConversationArtifact
} from '../api/artifacts'
import { deleteMemoryById, listMemories, promoteMemoryProposal, updateMemoryContent } from '../api/memories'
import ArtifactBoard from '../components/workspace/ArtifactBoard.vue'
import EditPanel from '../components/workspace/EditPanel.vue'
import MemoryPanel from '../components/workspace/MemoryPanel.vue'
import ReviewPanel from '../components/workspace/ReviewPanel.vue'
import TimelinePanel from '../components/workspace/TimelinePanel.vue'
import VersionStrip from '../components/workspace/VersionStrip.vue'
import WorkspaceComposer from '../components/workspace/WorkspaceComposer.vue'
import { useAgentRun } from '../composables/useAgentRun'
import { useArtifacts } from '../composables/useArtifacts'
import { useMemories } from '../composables/useMemories'
import { useRunEvents } from '../composables/useRunEvents'
import type {
  AgentStep,
  AgentPromptVersion,
  AgentV2RunResponse,
  Artifact,
  Conversation,
  ContextMemory,
  EvolutionSummaryItem,
  ModelSelection,
  QualityScores,
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
const disableClarification = ref(false)
const prompt = ref('')
const running = ref(false)
const errorMessage = ref('')
const { activeRun, activeRunId, runStatusText, canCancelRun, isTerminalRunStatus } = useAgentRun()
const steps = ref<AgentStep[]>([])
const taskLedgerItems = ref<TaskLedgerItem[]>([])
const toolInvocations = ref<ToolInvocation[]>([])
const {
  artifacts,
  selectedArtifact,
  versions,
  selectedVersionId,
  previewURLs,
  rankedArtifacts,
  recommendedArtifactId,
  selectedVersion,
  cleanupPreviewURLs,
  revokeAllPreviewURLs
} = useArtifacts()
const uploadFile = ref<File | null>(null)
const uploadingArtifact = ref(false)
const editPrompt = ref('')
const editingArtifact = ref(false)
const renderTitle = ref('')
const renderSubtitle = ref('')
const renderBrand = ref('')
const renderingTextLayer = ref(false)
const feedbackType = ref('selected')
const rating = ref(0)
const feedbackComment = ref('')
const feedbackSending = ref(false)
const selectingArtifact = ref(false)
const {
  memories,
  memoryNamespace,
  memoryStatusFilter,
  memoryLoading,
  promotingMemoryId,
  displayedMemories
} = useMemories()
const evolutionAgent = ref('prompt_agent')
const evolutionSummary = ref<EvolutionSummaryItem[]>([])
const promptVersions = ref<AgentPromptVersion[]>([])
const evolutionLoading = ref(false)
const clarificationAnswer = ref('')
const resumingRun = ref(false)
const lastRunState = ref<Record<string, unknown> | null>(null)
const compareArtifactIds = ref<number[]>([])
const activeArtifactTab = ref<'preview' | 'versions' | 'edit' | 'review' | 'memory'>('preview')
const userMenuOpen = ref(false)
const { startRunPolling, clearRunPolling } = useRunEvents(refreshRun)

interface PromptHistoryItem {
  id: number
  content: string
  createdAt: number
}

interface StepResultSnapshot {
  output?: {
    questions?: unknown
  }
}

const promptHistory = ref<PromptHistoryItem[]>([])
const canRun = computed(() => Boolean(prompt.value.trim() && activeConversationId.value && !running.value))
const canRetryFailedRun = computed(() => activeRun.value?.status === 'failed' && Boolean(retryPromptText().trim()) && Boolean(activeConversationId.value))
const clarificationQuestions = computed(() => extractClarificationQuestions())
const canResumeRun = computed(() => {
  return activeRun.value?.status === 'waiting_user' && Boolean(clarificationAnswer.value.trim()) && !resumingRun.value
})
const canEditArtifact = computed(() => Boolean(selectedArtifact.value && selectedVersionId.value && editPrompt.value.trim() && !editingArtifact.value))
const canRenderTextLayer = computed(() => Boolean(selectedArtifact.value?.kind === 'image' && selectedVersionId.value && renderTitle.value.trim() && !renderingTextLayer.value))
const selectedQualityScores = computed(() => parseQualityScores(selectedVersion.value?.quality_scores))
const compareArtifacts = computed(() => rankedArtifacts.value.filter(artifact => compareArtifactIds.value.includes(artifact.id)))
const hasArtifactPanel = computed(() => artifacts.value.length > 0)
const reviewStep = computed(() => {
  return [...steps.value].reverse().find(step => step.step_key === 'vision_review_agent' || step.name === 'vision_review_agent') || null
})
const reviewStatusText = computed(() => {
  if (selectedQualityScores.value?.reviewer) return reviewerLabel(selectedQualityScores.value.reviewer)
  if (reviewStep.value) return runStatusLabel(reviewStep.value.status)
  return '待审核'
})
const activeTitle = computed(() => {
  return conversations.value.find(item => item.id === activeConversationId.value)?.title || 'V2 图片工作台'
})
onMounted(async () => {
  await Promise.all([loadModelSelection(), loadConversations()])
  await restoreLastRun()
})

onBeforeUnmount(() => {
  clearRunPolling()
  revokeAllPreviewURLs()
})

async function loadModelSelection() {
  modelSelection.value = await fetchModelSelection()
  textModelConfigId.value = modelSelection.value.text_model_config_id || 0
  imageModelConfigId.value = modelSelection.value.image_model_config_id || 0
}

async function loadConversations() {
  const data = await apiFetch<{ conversations: Conversation[] }>('/api/conversations')
  conversations.value = data.conversations || []
  const savedConversationId = Number(localStorage.getItem('agent_v2_conversation_id') || 0)
  const savedConversation = conversations.value.find(item => item.id === savedConversationId)
  if (!activeConversationId.value && savedConversation) {
    await openConversation(savedConversation.id)
  } else if (!activeConversationId.value && conversations.value.length) {
    await openConversation(conversations.value[0].id)
  }
  if (!activeConversationId.value) {
    await createConversation()
  }
}

async function createConversation() {
  const data = await apiFetch<{ conversation: Conversation }>('/api/conversations', {
    method: 'POST',
    body: JSON.stringify({ title: 'V2 图片助手会话' })
  })
  conversations.value.unshift(data.conversation)
  await openConversation(data.conversation.id)
}

async function openConversation(id: number) {
  activeConversationId.value = id
  localStorage.setItem('agent_v2_conversation_id', String(id))
  activeRun.value = null
  steps.value = []
  taskLedgerItems.value = []
  toolInvocations.value = []
  selectedArtifact.value = null
  versions.value = []
  clarificationAnswer.value = ''
  compareArtifactIds.value = []
  loadPromptHistory(id)
  await Promise.all([loadArtifacts(), loadMemories(), loadEvolution()])
}

async function runAgent() {
  if (!canRun.value || !activeConversationId.value) return
  const originalPrompt = prompt.value.trim()
  running.value = true
  errorMessage.value = ''
  steps.value = []
  taskLedgerItems.value = []
  toolInvocations.value = []
  clarificationAnswer.value = ''
  try {
    recordPromptHistory(activeConversationId.value, originalPrompt)
    const data = await createAgentRun({
      conversationId: activeConversationId.value,
      content: originalPrompt,
      textModelConfigId: textModelConfigId.value,
      imageModelConfigId: imageModelConfigId.value,
      candidateCount: candidateCount.value,
      disableClarification: disableClarification.value
    })
    await applyRunResponse(data)
    if (data.agent_run?.id && ['created', 'queued', 'running'].includes(data.agent_run.status)) {
      startRunPolling(data.agent_run.id, () => activeRunId.value, message => {
        errorMessage.value = message
      })
    }
    prompt.value = ''
    await loadConversations()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '运行失败'
  } finally {
    running.value = false
  }
}

async function retryFailedRun() {
  const content = retryPromptText().trim()
  if (!activeConversationId.value || !content || running.value) return
  prompt.value = content
  await runAgent()
}

function retryPromptText() {
  const statePrompt = lastRunState.value?.user_request
  if (typeof statePrompt === 'string' && statePrompt.trim()) return statePrompt
  if (selectedVersion.value?.prompt?.trim()) return selectedVersion.value.prompt
  if (activeRun.value?.optimized_prompt?.trim()) return activeRun.value.optimized_prompt
  return prompt.value
}

function promptHistoryKey(conversationId: number) {
  return `agent_v2_prompt_history_${conversationId}`
}

function loadPromptHistory(conversationId: number) {
  try {
    const raw = localStorage.getItem(promptHistoryKey(conversationId))
    const parsed = raw ? JSON.parse(raw) : []
    promptHistory.value = Array.isArray(parsed)
      ? parsed.filter(isPromptHistoryItem)
      : []
  } catch {
    promptHistory.value = []
  }
}

function recordPromptHistory(conversationId: number, content: string) {
  if (!content.trim()) return
  const nextItem: PromptHistoryItem = {
    id: Date.now(),
    content,
    createdAt: Date.now()
  }
  promptHistory.value = [...promptHistory.value, nextItem]
  localStorage.setItem(promptHistoryKey(conversationId), JSON.stringify(promptHistory.value))
}

function isPromptHistoryItem(value: unknown): value is PromptHistoryItem {
  if (!value || typeof value !== 'object') return false
  const item = value as Partial<PromptHistoryItem>
  return typeof item.id === 'number' && typeof item.content === 'string' && typeof item.createdAt === 'number'
}

async function applyRunResponse(data: AgentV2RunResponse) {
  activeRun.value = data.agent_run
  lastRunState.value = data.state || null
  if (data.agent_run?.id) {
    localStorage.setItem('agent_v2_run_id', String(data.agent_run.id))
  }
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

async function restoreLastRun() {
  const savedRunId = Number(localStorage.getItem('agent_v2_run_id') || 0)
  if (!savedRunId || activeRun.value) return
  try {
    const data = await fetchAgentRun(savedRunId)
    if (activeConversationId.value && data.agent_run.conversation_id !== activeConversationId.value) return
    await applyRunResponse(data)
    if (['created', 'queued', 'running'].includes(data.agent_run.status)) {
      startRunPolling(data.agent_run.id, () => activeRunId.value, message => {
        errorMessage.value = message
      })
    }
  } catch {
    localStorage.removeItem('agent_v2_run_id')
  }
}

async function refreshRun() {
  if (!activeRunId.value) return
  const data = await fetchAgentRun(activeRunId.value)
  activeRun.value = data.agent_run
  lastRunState.value = data.state || null
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
    const data = await resumeAgentRun(activeRunId.value, clarificationAnswer.value.trim())
    clarificationAnswer.value = ''
    await applyRunResponse(data)
    if (data.agent_run?.id && ['created', 'queued', 'running'].includes(data.agent_run.status)) {
      startRunPolling(data.agent_run.id, () => activeRunId.value, message => {
        errorMessage.value = message
      })
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
    const data = await cancelAgentRun(activeRunId.value)
    activeRun.value = data.agent_run
    clearRunPolling()
    await refreshRun()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '取消失败'
  }
}

async function loadArtifacts() {
  if (!activeConversationId.value) return
  const data = await listConversationArtifacts(activeConversationId.value)
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
  activeArtifactTab.value = 'preview'
  selectedVersionId.value = 0
  feedbackComment.value = ''
  const data = await listArtifactVersions(artifact.id)
  versions.value = data.versions || []
  selectedVersionId.value = versions.value[versions.value.length - 1]?.id || 0
}

async function downloadSelected() {
  if (!selectedArtifact.value) return
  await downloadV2Artifact(selectedArtifact.value.id, selectedArtifact.value.name)
}

async function uploadArtifact(file?: File) {
  const selectedFile = file || uploadFile.value
  if (!activeConversationId.value || !selectedFile || uploadingArtifact.value) return
  uploadingArtifact.value = true
  errorMessage.value = ''
  try {
    const data = await uploadConversationArtifact(activeConversationId.value, selectedFile)
    uploadFile.value = null
    await loadArtifacts()
    const current = artifacts.value.find(item => item.id === data.artifact.id) || data.artifact
    await selectArtifact(current)
    activeArtifactTab.value = 'edit'
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
    await editArtifactVersion(selectedArtifact.value.id, selectedVersionId.value, promptText, imageModelConfigId.value)
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

async function renderTextLayer() {
  if (!selectedArtifact.value || !selectedVersionId.value || renderingTextLayer.value) return
  const title = renderTitle.value.trim()
  if (!title) return
  renderingTextLayer.value = true
  errorMessage.value = ''
  try {
    const data = await renderArtifactText(
      selectedArtifact.value.id,
      selectedVersionId.value,
      title,
      renderSubtitle.value.trim(),
      renderBrand.value.trim()
    )
    renderTitle.value = ''
    renderSubtitle.value = ''
    renderBrand.value = ''
    await loadArtifacts()
    const current = artifacts.value.find(item => item.id === data.artifact.id) || data.artifact
    await selectArtifact(current)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '文字分层渲染失败'
  } finally {
    renderingTextLayer.value = false
  }
}

async function markSelected() {
  if (!selectedArtifact.value || selectingArtifact.value) return
  selectingArtifact.value = true
  errorMessage.value = ''
  try {
    await selectArtifactVersion(selectedArtifact.value.id, selectedVersionId.value)
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
    await recordArtifactFeedback(
      selectedArtifact.value.id,
      selectedVersionId.value,
      feedbackType.value,
      rating.value,
      feedbackComment.value.trim()
    )
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
    const data = await listMemories({
      conversationId: activeConversationId.value,
      namespace: memoryNamespace.value,
      limit: 20
    })
    memories.value = data.memories || []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '记忆加载失败'
  } finally {
    memoryLoading.value = false
  }
}

async function loadEvolution() {
  evolutionLoading.value = true
  try {
    const params = new URLSearchParams({ agent_name: evolutionAgent.value, limit: '20' })
    const [summaryData, versionsData] = await Promise.all([
      apiFetch<{ summary: EvolutionSummaryItem[] }>(`/api/v2/evolution/summary?${params.toString()}`),
      apiFetch<{ prompt_versions: AgentPromptVersion[] }>(`/api/v2/evolution/prompt-versions?${params.toString()}`)
    ])
    evolutionSummary.value = summaryData.summary || []
    promptVersions.value = versionsData.prompt_versions || []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '演进数据加载失败'
  } finally {
    evolutionLoading.value = false
  }
}

async function draftPromptVersion() {
  if (evolutionLoading.value) return
  evolutionLoading.value = true
  try {
    await apiFetch<{ prompt_version: AgentPromptVersion }>('/api/v2/evolution/prompt-versions/draft', {
      method: 'POST',
      body: JSON.stringify({ agent_name: evolutionAgent.value, limit: 20 })
    })
    await loadEvolution()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '提示词草稿生成失败'
  } finally {
    evolutionLoading.value = false
  }
}

async function reviewPromptVersion(id: number) {
  await updatePromptVersionLifecycle(id, 'review')
}

async function activatePromptVersion(id: number) {
  await updatePromptVersionLifecycle(id, 'activate')
}

async function archivePromptVersion(id: number) {
  await updatePromptVersionLifecycle(id, 'archive')
}

async function updatePromptVersionLifecycle(id: number, action: string) {
  try {
    await apiFetch<{ prompt_version: AgentPromptVersion }>(`/api/v2/evolution/prompt-versions/${id}/${action}`, {
      method: 'POST'
    })
    await loadEvolution()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '提示词版本状态更新失败'
  }
}

async function deleteMemory(id: number) {
  await deleteMemoryById(id)
  memories.value = memories.value.filter(item => item.id !== id)
}

async function promoteMemory(id: number) {
  if (promotingMemoryId.value) return
  promotingMemoryId.value = id
  errorMessage.value = ''
  try {
    await promoteMemoryProposal(id)
    await loadMemories()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '记忆确认失败'
  } finally {
    promotingMemoryId.value = 0
  }
}

async function editMemory(memory: ContextMemory) {
  const nextContent = window.prompt('记忆内容', memory.content)
  if (nextContent === null) return
  const content = nextContent.trim()
  if (!content || content === memory.content) return
  try {
    await updateMemoryContent(memory.id, content)
    await loadMemories()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '记忆编辑失败'
  }
}

async function setMemoryNamespace(value: string) {
  memoryNamespace.value = value
  await loadMemories()
}

function toggleCompareArtifact(artifactId: number) {
  if (compareArtifactIds.value.includes(artifactId)) {
    compareArtifactIds.value = compareArtifactIds.value.filter(id => id !== artifactId)
    return
  }
  compareArtifactIds.value = [...compareArtifactIds.value, artifactId].slice(-3)
}

async function preloadArtifactPreviews(items: Artifact[]) {
  await Promise.all(items.filter(item => item.kind === 'image' || item.kind === 'svg').map(async item => {
    if (previewURLs.value[item.id]) return
    try {
      const url = await fetchV2ArtifactPreviewURL(item.id)
      previewURLs.value = { ...previewURLs.value, [item.id]: url }
    } catch {
      previewURLs.value = { ...previewURLs.value, [item.id]: '' }
    }
  }))
}

function previewUrlFor(artifact: Artifact) {
  return previewURLs.value[artifact.id] || ''
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

function formatRankScore(score?: number) {
  if (typeof score !== 'number') return '-'
  if (score >= 0 && score <= 1) return `${Math.round(score * 100)}`
  return score.toFixed(2)
}

function isPreviewableArtifact(artifact: Artifact) {
  return artifact.kind === 'image' || artifact.kind === 'svg'
}

function summarizeStep(step: AgentStep) {
  if (!step.output_json) return '等待结构化输出'
  try {
    const payload = JSON.parse(step.output_json)
    return localizeSummary(payload.summary || '已写入结构化输出')
  } catch {
    return '已写入结构化输出'
  }
}

function reviewerLabel(reviewer: string) {
  const labels: Record<string, string> = {
    mock_vision_review: '模拟视觉审核',
    real_vision_review: '真实视觉审核',
    real_vision_ocr_review: '真实视觉与文字审核',
    ranker_agent: '候选排序'
  }
  return labels[reviewer] || reviewer || '待审核'
}

function runStatusLabel(status: string) {
  const labels: Record<string, string> = {
    created: '已创建',
    queued: '排队中',
    running: '运行中',
    waiting_user: '等待补充',
    completed: '已完成',
    failed: '失败',
    retrying: '重试中',
    cancelled: '已取消'
  }
  return labels[status] || status || '未知'
}

function promptVersionStatusLabel(status: string) {
  const labels: Record<string, string> = {
    draft: '草稿',
    review: '审核中',
    active: '已启用',
    archived: '已归档'
  }
  return labels[status] || status || '未知'
}

function localizeSummary(summary: string) {
  const value = String(summary || '').trim()
  const map: Record<string, string> = {
    'classified request as image_generation': '已识别为图片生成任务',
    'extracted structured image requirements': '已提取结构化图片需求',
    'prepared structured image prompt bundle': '已生成结构化图片提示词',
    'text safety check passed': '文本安全检查已通过',
    'mock vision review completed': '模拟视觉审核已完成',
    'real vision review found no generated image': '视觉审核未找到生成图片',
    'real vision review completed for image candidates': '候选图片视觉审核已完成',
    'ranker found no candidate review': '排序器未找到候选审核结果',
    'ranked image candidates': '候选图片排序已完成'
  }
  if (map[value]) return map[value]
  const loaded = value.match(/^loaded (\d+) memory items$/)
  if (loaded) return `已加载 ${loaded[1]} 条记忆`
  const generated = value.match(/^generated (\d+) image candidate\(s\)$/)
  if (generated) return `已生成 ${generated[1]} 张候选图片`
  const persisted = value.match(/^persisted (\d+) artifact candidate\(s\)$/)
  if (persisted) return `已保存 ${persisted[1]} 个候选产物`
  const safety = value.match(/^image safety check passed for (\d+) candidate\(s\)$/)
  if (safety) return `已通过 ${safety[1]} 张候选图片的安全检查`
  return value
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

function formatHistoryTime(timestamp: number) {
  return new Date(timestamp).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}
</script>
