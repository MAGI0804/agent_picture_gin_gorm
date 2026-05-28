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

      <WorkspaceComposer
        v-model:text-model-config-id="textModelConfigId"
        v-model:image-model-config-id="imageModelConfigId"
        v-model:candidate-count="candidateCount"
        v-model:prompt="prompt"
        :model-selection="modelSelection"
        :running="running"
        :can-run="canRun"
        :can-retry="canRetryFailedRun"
        :error-message="errorMessage"
        @run="runAgent"
        @clear="prompt = ''"
        @retry="retryFailedRun"
      />

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

      <TimelinePanel
        :active-run="activeRun"
        :steps="steps"
        :task-ledger-items="taskLedgerItems"
        :tool-invocations="toolInvocations"
      />
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

      <ArtifactBoard
        :artifacts="rankedArtifacts"
        :selected-artifact-id="selectedArtifact?.id || 0"
        :recommended-artifact-id="recommendedArtifactId"
        :compare-ids="compareArtifactIds"
        :preview-u-r-ls="previewURLs"
        @select="selectArtifact"
        @toggle-compare="toggleCompareArtifact"
      />

      <section v-if="compareArtifacts.length" class="v2-preview">
        <div class="v2-preview-head">
          <strong>候选对比</strong>
          <button type="button" @click="compareArtifactIds = []">清空</button>
        </div>
        <div class="v2-compare-grid">
          <article v-for="artifact in compareArtifacts" :key="artifact.id">
            <img v-if="isPreviewableArtifact(artifact) && previewUrlFor(artifact)" :src="previewUrlFor(artifact)" :alt="artifact.name" />
            <strong>{{ artifact.name }}</strong>
            <small>Rank {{ formatRankScore(artifact.rank_score) }}</small>
          </article>
        </div>
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
          v-if="isPreviewableArtifact(selectedArtifact) && previewUrlFor(selectedArtifact)"
          :src="previewUrlFor(selectedArtifact)"
          :alt="selectedArtifact.name"
        />
        <p v-else>{{ selectedArtifact.mime_type }}</p>
      </section>

      <VersionStrip
        :artifact-id="selectedArtifact?.id || 0"
        :versions="versions"
        v-model:selected-version-id="selectedVersionId"
      />

      <EditPanel
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
        :artifact-id="selectedArtifact?.id || 0"
        :quality-scores="selectedQualityScores"
        :review-status-text="reviewStatusText"
        :review-summary="reviewStep ? summarizeStep(reviewStep) : ''"
        :artifact-rank-score="selectedArtifact?.rank_score"
      />

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
            <strong>Evolution</strong>
            <span>{{ promptVersions.length }} versions</span>
          </div>
          <button type="button" :disabled="evolutionLoading" @click="loadEvolution">刷新</button>
        </header>
        <label>
          Agent
          <select v-model="evolutionAgent" :disabled="evolutionLoading" @change="loadEvolution">
            <option value="prompt_agent">prompt_agent</option>
            <option value="vision_review_agent">vision_review_agent</option>
            <option value="ranker_agent">ranker_agent</option>
            <option value="poster_render_agent">poster_render_agent</option>
          </select>
        </label>
        <button type="button" :disabled="evolutionLoading" @click="draftPromptVersion">
          {{ evolutionLoading ? '处理中...' : '生成 Prompt Draft' }}
        </button>
        <ul v-if="evolutionSummary.length" class="v2-memory-list">
          <li v-for="item in evolutionSummary" :key="item.failure_type">
            <div>
              <strong>{{ item.failure_type }} · {{ item.count }}</strong>
              <p>{{ item.action_item }}</p>
            </div>
          </li>
        </ul>
        <ul v-if="promptVersions.length" class="v2-memory-list">
          <li v-for="version in promptVersions" :key="version.id">
            <div>
              <strong>{{ version.agent_name }} · {{ version.version }}</strong>
              <p>{{ version.status }}</p>
            </div>
            <div class="v2-memory-actions">
              <button v-if="version.status === 'draft'" type="button" @click="reviewPromptVersion(version.id)">Review</button>
              <button v-if="version.status === 'review' || version.status === 'archived'" type="button" @click="activatePromptVersion(version.id)">Active</button>
              <button v-if="version.status !== 'archived'" type="button" @click="archivePromptVersion(version.id)">Archive</button>
            </div>
          </li>
        </ul>
      </section>

      <MemoryPanel
        :conversation-id="activeConversationId || 0"
        :memories="memories"
        :displayed-memories="displayedMemories"
        :namespace="memoryNamespace"
        v-model:status-filter="memoryStatusFilter"
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
const { startRunPolling, clearRunPolling } = useRunEvents(refreshRun)

interface StepResultSnapshot {
  output?: {
    questions?: unknown
  }
}

const canRun = computed(() => Boolean(prompt.value.trim() && activeConversationId.value && !running.value))
const canRetryFailedRun = computed(() => activeRun.value?.status === 'failed' && Boolean(retryPromptText().trim()) && Boolean(activeConversationId.value))
const clarificationQuestions = computed(() => extractClarificationQuestions())
const canResumeRun = computed(() => {
  return activeRun.value?.status === 'waiting_user' && Boolean(clarificationAnswer.value.trim()) && !resumingRun.value
})
const canUploadArtifact = computed(() => Boolean(activeConversationId.value && uploadFile.value && !uploadingArtifact.value))
const canEditArtifact = computed(() => Boolean(selectedArtifact.value && selectedVersionId.value && editPrompt.value.trim() && !editingArtifact.value))
const canRenderTextLayer = computed(() => Boolean(selectedArtifact.value?.kind === 'image' && selectedVersionId.value && renderTitle.value.trim() && !renderingTextLayer.value))
const selectedQualityScores = computed(() => parseQualityScores(selectedVersion.value?.quality_scores))
const compareArtifacts = computed(() => rankedArtifacts.value.filter(artifact => compareArtifactIds.value.includes(artifact.id)))
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
    body: JSON.stringify({ title: 'V2 图片 Agent 会话' })
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
  await Promise.all([loadArtifacts(), loadMemories(), loadEvolution()])
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
    const data = await createAgentRun({
      conversationId: activeConversationId.value,
      content: prompt.value.trim(),
      textModelConfigId: textModelConfigId.value,
      imageModelConfigId: imageModelConfigId.value,
      candidateCount: candidateCount.value
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

function handleUploadFile(event: Event) {
  const input = event.target as HTMLInputElement
  uploadFile.value = input.files?.[0] || null
}

async function uploadArtifact() {
  if (!activeConversationId.value || !uploadFile.value || uploadingArtifact.value) return
  uploadingArtifact.value = true
  errorMessage.value = ''
  try {
    const data = await uploadConversationArtifact(activeConversationId.value, uploadFile.value)
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
    errorMessage.value = error instanceof Error ? error.message : 'Prompt draft 生成失败'
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
    errorMessage.value = error instanceof Error ? error.message : 'Prompt version 状态更新失败'
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
  const nextContent = window.prompt('Memory', memory.content)
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
