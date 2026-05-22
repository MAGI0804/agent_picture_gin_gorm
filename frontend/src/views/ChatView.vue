<template>
  <main class="app-shell compact-shell">
    <section class="chat-page with-left-sidebar">
      <aside class="conversation-sidebar" :style="{ width: sidebarWidth + 'px' }">
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
            <small class="model-state">在线</small>
          </div>
        </section>

        <div class="sidebar-actions">
          <button class="primary new-chat-button" @click="createConversation">+ 新建会话</button>
        </div>

        <div class="sidebar-nav">
          <button class="active" @click="router.push('/chat')">
            <span class="nav-icon icon-chat" aria-hidden="true"></span>
            对话
          </button>
          <button type="button" @click="router.push('/settings')">
            <span class="nav-icon icon-settings" aria-hidden="true"></span>
            设置
          </button>
          <button type="button" @click="logout">
            <span class="nav-icon icon-logout" aria-hidden="true"></span>
            退出登录
          </button>
        </div>

        <section class="conversation-section">
          <h2>会话列表</h2>
          <label class="search-box">
            <span class="sr-only">搜索会话</span>
            <input placeholder="搜索会话..." />
          </label>
          <nav class="conversation-list">
            <button
              v-for="item in conversations"
              :key="item.id"
              :class="{ active: item.id === activeConversationId }"
              @click="openConversation(item.id)"
            >
              <span :title="item.title">{{ item.title }}</span>
              <small>{{ formatMessageTime(item.updated_at) }}</small>
            </button>
          </nav>
        </section>

        <footer class="sidebar-user-menu" @click.stop>
          <button class="sidebar-user-trigger" type="button" @click="userMenuOpen = !userMenuOpen">
            <div class="avatar">{{ avatarInitial }}</div>
            <span>{{ userDisplayName }}</span>
            <i>⌄</i>
          </button>
          <div v-if="userMenuOpen" class="user-popover">
            <button type="button" @click="router.push('/settings'); userMenuOpen = false">
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

      <div class="resize-handle" @mousedown="startResize('left')" />

      <section class="chat-left chat-main">
        <header class="chat-header">
          <div>
            <strong :title="activeTitle">{{ activeTitle }}</strong>
            <span :title="modelSummary">{{ modelSummary }}</span>
          </div>
          <div class="header-actions">
            <span class="status-badge">{{ runStatus }}</span>
            <button class="icon-button" type="button" :disabled="!activeRunId" @click="loadRunEvents">⋮</button>
          </div>
        </header>

        <div v-if="agentSteps.length" class="thinking-panel">
          <button class="thinking-toggle" @click="thinkingExpanded = !thinkingExpanded">
            <span>{{ thinkingExpanded ? '收起' : '展开' }}</span>
            Agent 执行步骤
          </button>
          <div v-show="thinkingExpanded" class="thinking-content">
            <ol>
              <li v-for="step in agentSteps" :key="step.id">
                <span class="step-name">{{ step.name }}</span>
                <span class="step-status">{{ step.status }}</span>
                <p v-if="step.think_content"><b>业务思考：</b>{{ step.think_content }}</p>
                <p v-if="step.reasoning_content"><b>模型推理：</b>{{ step.reasoning_content }}</p>
                <p v-if="step.error_message"><b>错误：</b>{{ step.error_message }}</p>
              </li>
            </ol>
          </div>
        </div>

        <div class="messages">
          <article
            v-for="(message, index) in messages"
            :key="message?.id || index"
            :class="['message', message?.role, message.input_type]"
          >
            <div class="message-meta">
              <span>{{ roleLabel(message.role) }}</span>
              <span>{{ dialogTypeLabel(message) }}</span>
              <span>{{ messageModelName(message) }}</span>
              <span>{{ formatMessageTime(message.created_at) }}</span>
            </div>
            <div
              v-if="message.is_optimized && message.original_prompt && message.original_prompt !== message.content"
              class="original-prompt-block"
            >
              <strong>原始未优化提示词</strong>
              <p>{{ message.original_prompt }}</p>
            </div>
            <p>{{ message.content }}</p>
            <details v-if="message.thinking_content" class="message-thinking-detail">
              <summary>查看思考过程</summary>
              <p>{{ message.thinking_content }}</p>
            </details>
          </article>

          <section v-if="pendingQuestions.length" class="questions">
            <strong>请补充以下信息</strong>
            <ol>
              <li v-for="question in pendingQuestions" :key="question.id">{{ question.question }}</li>
            </ol>
            <div class="inline-answer-box">
              <label>
                回答追问
                <textarea
                  v-model="answerText"
                  placeholder="请回答上方问题。Enter 发送，Shift + Enter 换行。"
                  @keydown.enter.exact.prevent="sendAnswer"
                />
              </label>
              <button :disabled="!canSendAnswer" @click="sendAnswer">提交回答</button>
            </div>
          </section>
        </div>

        <footer class="normal-composer">
          <div class="composer-shell">
            <div class="composer-quick-actions">
              <button type="button" :disabled="!sendingNormal && !sendingAnswer">× 停止生成</button>
              <button type="button" :disabled="!canSendNormal" @click="sendNormal()">再试一次</button>
              <button type="button" @click="normalText = ''">清空上下文</button>
              <button type="button">清空记忆</button>
              <button type="button">复制会话</button>
            </div>
            <label class="composer-label">
              <span class="sr-only">输入问题</span>
              <div class="composer-box">
              <textarea
                v-model="normalText"
                placeholder="请输入你的问题...（Shift + Enter 换行，Enter 发送）"
                @keydown.enter.exact.prevent="sendNormal()"
              />
              <div class="composer-tools">
                <div class="composer-left-tools">
                  <button type="button" aria-label="上传图片">□</button>
                  <button type="button" aria-label="新增附件">+</button>
                  <button type="button" aria-label="上传文件">⇧</button>
                </div>
                <select v-model="taskType" aria-label="模式">
                  <option value="text_chat">文本模式</option>
                  <option value="image_generation">图片模式</option>
                </select>
                <select :value="currentModelId" aria-label="模型" @change="saveComposerModelSelection">
                  <option :value="0">未选择模型</option>
                  <option v-for="item in activeModelOptions" :key="item.id" :value="item.id">
                    {{ item.model_name }}
                  </option>
                </select>
                <button :disabled="!canSendNormal" @click="sendNormal()">发送</button>
                <button class="secondary-action" :disabled="!canOptimizePrompt" @click="optimizeNormalPrompt">
                  {{ optimizingPrompt ? '优化中...' : '智能优化' }}
                </button>
                <button class="secondary-action" :disabled="!canStartSmartQa" @click="startSmartQa">
                  智能问答
                </button>
              </div>
              <div v-if="optimizedPromptText || optimizationError || optimizingPrompt" class="optimization-panel">
                <div class="optimization-header">
                  <strong>优化后的提示词</strong>
                  <span v-if="optimizedPromptText">{{ optimizedPromptText.length }} 字</span>
                </div>
                <textarea
                  v-if="optimizedPromptText"
                  v-model="optimizedPromptText"
                  aria-label="优化后的提示词"
                />
                <p v-if="optimizationError" class="optimization-error">{{ optimizationError }}</p>
                <div v-if="optimizedPromptText" class="optimization-actions">
                  <button class="primary" :disabled="sendingNormal || !optimizedPromptText.trim()" @click="sendNormal(true)">
                    是，使用优化后提问
                  </button>
                  <button :disabled="sendingNormal" @click="sendNormal(false)">否，使用原提示词</button>
                </div>
              </div>
              </div>
            </label>
          </div>
        </footer>
      </section>

      <div class="resize-handle" @mousedown="startResize('right')" />

      <aside class="artifact-panel" :style="{ width: panelWidth + 'px' }">
        <header>
          <strong>工作区</strong>
          <button :disabled="!activeConversationId" @click="refreshWorkspace">刷新</button>
        </header>

        <div class="panel-tabs">
          <button :class="{ active: rightTab === 'artifacts' }" @click="rightTab = 'artifacts'">
            <span class="nav-icon icon-artifact" aria-hidden="true"></span>
            产物
          </button>
          <button :class="{ active: rightTab === 'messages' }" @click="rightTab = 'messages'">
            <span class="nav-icon icon-chat" aria-hidden="true"></span>
            消息
          </button>
          <button :class="{ active: rightTab === 'steps' }" @click="rightTab = 'steps'">
            <span class="nav-icon icon-steps" aria-hidden="true"></span>
            步骤
          </button>
        </div>

        <section v-if="rightTab === 'artifacts'" class="artifact-list">
          <div v-if="processTimeline.length" class="process-timeline">
            <strong>本轮过程</strong>
            <ol>
              <li v-for="item in processTimeline" :key="item.id" :class="item.status">
                <span>{{ item.title }}</span>
                <p v-if="item.detail">{{ item.detail }}</p>
              </li>
            </ol>
          </div>

          <p v-if="!artifacts.length && !processTimeline.length" class="muted">暂无产物。发起图片或 HTML 生成后会显示在这里。</p>
          
          <!-- 图片生成中/等待状态显示 -->
          <div v-if="sendingNormal || sendingAnswer" class="generating-status">
            <div class="spinner"></div>
            <span>任务提交成功，正在构建图片...</span>
          </div>
          
          <!-- 多图片展示区域 -->
          <div v-if="imageArtifacts.length > 0" class="image-gallery">
            <!-- 左侧图片缩略图列表 -->
            <div class="thumbnail-list">
              <div 
                v-for="(artifact, index) in imageArtifacts" 
                :key="artifact.id"
                :class="['thumbnail-item', { active: index === selectedImageIndex }]"
                @click="selectedImageIndex = index"
              >
                <img :src="artifact.preview_url" :alt="artifact.name" />
              </div>
            </div>
            
            <!-- 右侧大图展示区 -->
            <div class="main-image-container">
              <div class="main-image-header">
                <strong>{{ imageArtifacts[selectedImageIndex]?.name }}</strong>
                <button @click="downloadArtifactFile(imageArtifacts[selectedImageIndex])">下载</button>
              </div>
              <img 
                v-if="imageArtifacts[selectedImageIndex]" 
                :src="imageArtifacts[selectedImageIndex].preview_url" 
                :alt="imageArtifacts[selectedImageIndex].name" 
                class="main-image"
              />
              <div class="image-counter">
                {{ selectedImageIndex + 1 }} / {{ imageArtifacts.length }}
              </div>
            </div>
          </div>
          
          <!-- 单图片/其他类型展示区 -->
          <template v-else-if="artifacts.length > 0">
            <article v-for="artifact in artifacts" :key="artifact.id" class="artifact">
              <div class="artifact-title">
                <strong>{{ artifact.name }}</strong>
                <button @click="downloadArtifactFile(artifact)">下载</button>
              </div>
              <img v-if="artifact.kind === 'image'" :src="artifact.preview_url" :alt="artifact.name" />
              <iframe v-else-if="artifact.kind === 'html'" :src="artifact.preview_url" :title="artifact.name" />
              <p v-else>{{ artifact.mime_type }}</p>
            </article>
          </template>
        </section>

        <section v-else-if="rightTab === 'messages'" class="message-list">
          <p v-if="!messages.length" class="muted">暂无消息</p>
          <button
            v-for="(message, index) in messages"
            :key="message?.id || index"
            class="message-item"
            :class="{ active: activeMessageIndex === index }"
            @click="scrollToMessage(index)"
          >
            <div class="message-header">
              <span class="message-role">{{ roleLabel(message.role) }}</span>
              <span class="message-index">#{{ index + 1 }}</span>
            </div>
            <div class="message-mini-meta">
              <span>{{ dialogTypeLabel(message) }}</span>
              <span>{{ messageModelName(message) }}</span>
              <span>{{ formatMessageTime(message.created_at) }}</span>
            </div>
            <p class="message-content">{{ truncateContent(message.content) }}</p>
            <p v-if="message.thinking_content" class="message-thinking">
              <small>思考：</small>{{ truncateContent(message.thinking_content, 40) }}
            </p>
          </button>
        </section>

        <section v-else class="events">
          <ol>
            <li v-for="step in agentSteps" :key="step.id">
              <strong>{{ step.name }}</strong>
              <small>{{ step.status }}</small>
              <p>{{ step.output || step.think_content }}</p>
            </li>
          </ol>
        </section>
      </aside>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, downloadArtifact, getCurrentUser, setCurrentUser, setToken } from '../api'
import type {
  AgentRun,
  AgentStep,
  Artifact,
  Conversation,
  FollowUpQuestion,
  GlobalModelConfig,
  Message,
  ModelSelection,
  UserProfile
} from '../types'

type TaskType = 'text_chat' | 'image_generation'
type RightTab = 'artifacts' | 'messages' | 'steps'

interface ModelOutput {
  thinking_content?: string
}

interface SendMessageResponse {
  user_message: Message
  assistant_message: Message
  follow_up_questions?: FollowUpQuestion[]
  artifacts?: Artifact[]
  agent_run: AgentRun
  agent_steps?: AgentStep[]
  model_output?: ModelOutput
  conversation?: Conversation
}

interface MessageListResponse {
  messages: Message[]
  agent_runs?: AgentRun[]
}

interface OptimizePromptResponse {
  original_prompt: string
  optimized_prompt: string
  target_length: number
  original_length?: number
  optimized_length?: number
}

interface ProcessTimelineItem {
  id: number
  title: string
  detail: string
  status: 'running' | 'completed' | 'failed'
}

const IMAGE_PROMPT_LIMIT = 750
const IMAGE_PROMPT_BYTE_LIMIT = 780
const router = useRouter()
const conversations = ref<Conversation[]>([])
const activeConversationId = ref<number | null>(null)
const messages = ref<Message[]>([])
const pendingQuestions = ref<FollowUpQuestion[]>([])
const artifacts = ref<Artifact[]>([])
const agentSteps = ref<AgentStep[]>([])
const activeRunId = ref<number | null>(null)
const runMetaMap = ref<Record<number, AgentRun>>({})
const runStatus = ref('就绪')
const answerText = ref('')
const normalText = ref('')
const taskType = ref<TaskType>('text_chat')
const sendingNormal = ref(false)
const sendingAnswer = ref(false)
const optimizingPrompt = ref(false)
const optimizedPromptText = ref('')
const optimizationError = ref('')
const sidebarWidth = ref(280)
const panelWidth = ref(420)
const resizing = ref<'left' | 'right' | null>(null)
const thinkingExpanded = ref(false)
const activeMessageIndex = ref<number | null>(null)
const rightTab = ref<RightTab>('artifacts')
const modelSelection = ref<ModelSelection | null>(null)
const processTimeline = ref<ProcessTimelineItem[]>([])
const smartQaOriginalPrompt = ref('')
const smartQaActive = ref(false)
const userMenuOpen = ref(false)
const currentUser = ref<UserProfile | null>(getCurrentUser())

const activeTitle = computed(() => {
  return conversations.value.find(item => item.id === activeConversationId.value)?.title || '新的图片 Agent 会话'
})
const modelSummary = computed(() => {
  const selection = modelSelection.value
  if (!selection) return '模型选择加载中'
  const text = selection.text_models.find(item => item.id === selection.text_model_config_id)?.model_name || '未选文本模型'
  const image = selection.image_models.find(item => item.id === selection.image_model_config_id)?.model_name || '未选图片模型'
  return `${text} / ${image}`
})
const selectedTextModelName = computed(() => {
  const selection = modelSelection.value
  if (!selection) return '加载中'
  return selection.text_models.find(item => item.id === selection.text_model_config_id)?.model_name || '未选择'
})
const selectedImageModelName = computed(() => {
  const selection = modelSelection.value
  if (!selection) return '加载中'
  return selection.image_models.find(item => item.id === selection.image_model_config_id)?.model_name || '未选择'
})
const userDisplayName = computed(() => {
  return currentUser.value?.nickname || currentUser.value?.account || '用户'
})
const avatarInitial = computed(() => {
  return Array.from(userDisplayName.value.trim())[0]?.toUpperCase() || 'U'
})
const canSendNormal = computed(() => Boolean(normalText.value.trim()) && !sendingNormal.value)
const canOptimizePrompt = computed(() => Boolean(normalText.value.trim()) && !sendingNormal.value && !optimizingPrompt.value)
const canStartSmartQa = computed(() => {
  return Boolean(normalText.value.trim()) && !sendingNormal.value && !optimizingPrompt.value
})
const canSendAnswer = computed(() => {
  return Boolean(activeConversationId.value && answerText.value.trim() && pendingQuestions.value.length) && !sendingAnswer.value
})
const activeModelOptions = computed<GlobalModelConfig[]>(() => {
  const selection = modelSelection.value
  if (!selection) return []
  return taskType.value === 'text_chat' ? selection.text_models : selection.image_models
})
const currentModelId = computed(() => {
	const selection = modelSelection.value
	if (!selection) return 0
	return taskType.value === 'text_chat' ? selection.text_model_config_id : selection.image_model_config_id
})
const selectedTextModelId = computed(() => modelSelection.value?.text_model_config_id || 0)
const selectedImageModelId = computed(() => modelSelection.value?.image_model_config_id || 0)
const selectedImageIndex = ref<number>(0)
const imageArtifacts = computed(() => artifacts.value.filter(item => item.kind === 'image'))

onMounted(async () => {
  await loadCurrentUser()
  await loadModelSelection()
  await loadConversations()
})

onUnmounted(() => {
  stopResize()
})

async function loadModelSelection() {
  modelSelection.value = await apiFetch<ModelSelection>('/api/settings/model-selection')
}

async function loadCurrentUser() {
  try {
    const user = await apiFetch<UserProfile>('/api/auth/me')
    currentUser.value = user
    setCurrentUser(user)
  } catch (error) {
    console.error('Load current user error:', error)
  }
}

function logout() {
  setToken('')
  router.push('/login')
}

async function loadConversations() {
  const data = await apiFetch<{ conversations: Conversation[] }>('/api/conversations')
  conversations.value = data.conversations || []
  if (!activeConversationId.value && conversations.value.length) {
    await openConversation(conversations.value[0].id)
  }
  if (!activeConversationId.value && conversations.value.length === 0) {
    await createConversation()
  }
}

async function createConversation() {
  const data = await apiFetch<{ conversation: Conversation }>('/api/conversations', {
    method: 'POST',
    body: JSON.stringify({ title: '新的图片 Agent 会话' })
  })
  conversations.value.unshift(data.conversation)
  await openConversation(data.conversation.id)
}

async function ensureConversation() {
  if (!activeConversationId.value) {
    await createConversation()
  }
  return activeConversationId.value
}

async function openConversation(id: number) {
  activeConversationId.value = id
  pendingQuestions.value = []
  agentSteps.value = []
  smartQaOriginalPrompt.value = ''
  smartQaActive.value = false
  processTimeline.value = []
  await refreshWorkspace()
}

async function refreshWorkspace() {
  await Promise.all([loadMessages(), loadArtifacts()])
}

async function loadMessages() {
  if (!activeConversationId.value) return
  const data = await apiFetch<MessageListResponse>(`/api/conversations/${activeConversationId.value}/messages`)
  registerRunMeta(data.agent_runs || [])
  messages.value = await hydrateThinkingMessages(data.messages || [])
  const lastRun = [...messages.value].reverse().find(message => message.agent_run_id)
  activeRunId.value = lastRun?.agent_run_id || null
  if (activeRunId.value) {
    agentSteps.value = await fetchRunSteps(activeRunId.value)
  }
}

async function loadArtifacts() {
  if (!activeConversationId.value) return
  const data = await apiFetch<{ artifacts: Artifact[] }>(`/api/conversations/${activeConversationId.value}/artifacts`)
  artifacts.value = data.artifacts || []
}

async function sendNormal(useOptimizedPrompt = false) {
  const originalContent = normalText.value.trim()
  const optimizedContent = optimizedPromptText.value.trim()
  const isUsingOptimized = Boolean(useOptimizedPrompt && optimizedContent)
  const selectedContent = isUsingOptimized ? optimizedContent : originalContent
  if (sendingNormal.value || !selectedContent) return
  let requestSubmitted = false
  let localMessageCreated = false
  clearPromptOptimization()
  resetSmartQaState()
  resetProcessTimeline()
  sendingNormal.value = true
  try {
    const conversationId = await ensureConversation()
    if (!conversationId) return
    if (taskType.value === 'image_generation') {
      messages.value.push(createLocalMessage(conversationId, 'normal', selectedContent, {
        isOptimized: isUsingOptimized,
        originalPrompt: isUsingOptimized ? originalContent : '',
        optimizedPrompt: isUsingOptimized ? optimizedContent : ''
      }))
      localMessageCreated = true
      appendProcessStep('原始未优化提示词', isUsingOptimized ? originalContent : selectedContent, 'completed')
    }
    const promptPayload = await preparePromptForTask(selectedContent, isUsingOptimized)
    normalText.value = ''
    if (!localMessageCreated) {
      messages.value.push(createLocalMessage(conversationId, 'normal', promptPayload.content, {
        isOptimized: promptPayload.isOptimized,
        originalPrompt: promptPayload.originalPrompt,
        optimizedPrompt: promptPayload.optimizedPrompt
      }))
    }
    requestSubmitted = true
    showLocalThinking('frontend_dispatch', '已提交请求，等待 Agent 规划下一步。')
    appendProcessStep('已提交用户输入', promptPayload.content, 'completed')

    const data = await apiFetch<SendMessageResponse>(`/api/conversations/${conversationId}/messages`, {
      method: 'POST',
      body: JSON.stringify({
        input_type: 'normal',
        task_type: taskType.value,
        content: promptPayload.content,
        text_model_config_id: selectedTextModelId.value,
        image_model_config_id: taskType.value === 'image_generation' ? selectedImageModelId.value : 0,
        original_prompt: promptPayload.originalPrompt || (isUsingOptimized ? originalContent : ''),
        is_optimized: promptPayload.isOptimized,
        optimized_prompt: promptPayload.optimizedPrompt,
        stream: true,
        return_reasoning: true
      })
    })
    await applySendResponse(data)
  } catch (error) {
    if (!requestSubmitted) {
      normalText.value = originalContent
    }
    appendErrorMessage(error)
  } finally {
    sendingNormal.value = false
  }
}

async function optimizeNormalPrompt() {
  if (optimizingPrompt.value || !normalText.value.trim()) return
  optimizationError.value = ''
  optimizedPromptText.value = ''
  resetProcessTimeline()
  appendProcessStep('准备智能优化', `原始提示词：${normalText.value.trim()}`, 'completed')
  appendProcessStep('正在调用 deepseek-v4-pro', '请将下面的提示词进行优化，优化后的提示词显示出来。', 'running')
  optimizingPrompt.value = true
  try {
    const data = await apiFetch<OptimizePromptResponse>('/api/prompts/optimize', {
      method: 'POST',
      body: JSON.stringify({
        content: normalText.value.trim(),
        target_length: 700
      })
    })
    optimizedPromptText.value = data.optimized_prompt || ''
    completeLastRunningProcessStep(
      'deepseek-v4-pro 优化完成',
      `原始 ${data.original_length ?? data.original_prompt.length} 字，优化后 ${data.optimized_length ?? optimizedPromptText.value.length} 字。`
    )
    appendProcessStep('优化后的提示词', optimizedPromptText.value, 'completed')
  } catch (error) {
    optimizationError.value = error instanceof Error ? error.message : '提示词太长，请重新输入'
    completeLastRunningProcessStep('智能优化失败', optimizationError.value, 'failed')
  } finally {
    optimizingPrompt.value = false
  }
}

async function startSmartQa() {
  const content = normalText.value.trim()
  if (sendingNormal.value || !content) return
  taskType.value = 'image_generation'
  normalText.value = ''
  clearPromptOptimization()
  resetProcessTimeline()
  resetSmartQaState()
  smartQaOriginalPrompt.value = content
  smartQaActive.value = true
  sendingNormal.value = true
  try {
    const conversationId = await ensureConversation()
    if (!conversationId) return
    messages.value.push(createLocalMessage(conversationId, 'normal', content))
    appendProcessStep('进入智能问答', `原始需求：${content}`, 'completed')
    appendProcessStep('正在生成针对性问题', '调用 deepseek-v4-pro 输出图片生成前需要确认的问题。', 'running')
    showLocalThinking('smart_question_agent', '正在根据用户需求生成针对性问题。')

    const data = await apiFetch<SendMessageResponse>(`/api/conversations/${conversationId}/messages`, {
      method: 'POST',
      body: JSON.stringify({
        input_type: 'normal',
        task_type: 'image_generation',
        content,
        question_mode: 'smart_qa',
        text_model_config_id: selectedTextModelId.value,
        image_model_config_id: selectedImageModelId.value,
        stream: true,
        return_reasoning: true
      })
    })
    completeLastRunningProcessStep('针对性问题已生成', formatQuestions(data.follow_up_questions || []))
    await applySendResponse(data)
  } catch (error) {
    completeLastRunningProcessStep(
      '智能问答失败',
      error instanceof Error ? error.message : '请求失败',
      'failed'
    )
    appendErrorMessage(error)
  } finally {
    sendingNormal.value = false
  }
}

function clearPromptOptimization() {
  optimizedPromptText.value = ''
  optimizationError.value = ''
}

async function preparePromptForTask(content: string, isAlreadyOptimized: boolean) {
  if (taskType.value !== 'image_generation') {
    return {
      content,
      isOptimized: isAlreadyOptimized,
      originalPrompt: '',
      optimizedPrompt: isAlreadyOptimized ? content : ''
    }
  }
  return prepareImagePromptForSend(content, isAlreadyOptimized)
}

async function prepareImagePromptForSend(content: string, isAlreadyOptimized: boolean) {
  if (fitsImagePromptLimits(content)) {
    return {
      content,
      isOptimized: isAlreadyOptimized,
      originalPrompt: '',
      optimizedPrompt: isAlreadyOptimized ? content : ''
    }
  }

  appendProcessStep(
    '检测到图片提示词超过模型限制',
    `当前 ${content.length} 字、${promptByteLength(content)} 字节，自动调用 deepseek-v4-pro 优化。`,
    'completed'
  )
  appendProcessStep('正在自动优化图片提示词', '不需要用户确认，优化完成后会直接提交图片模型。', 'running')

  let optimizedPrompt = ''
  try {
    const data = await apiFetch<OptimizePromptResponse>('/api/prompts/optimize', {
      method: 'POST',
      body: JSON.stringify({
        content,
        target_length: IMAGE_PROMPT_LIMIT
      })
    })
    optimizedPrompt = data.optimized_prompt || ''
    completeLastRunningProcessStep(
      '自动优化完成',
      `原始 ${data.original_length ?? content.length} 字，优化后 ${data.optimized_length ?? optimizedPrompt.length} 字、${promptByteLength(optimizedPrompt)} 字节。`
    )
  } catch (error) {
    completeLastRunningProcessStep(
      '自动优化失败',
      error instanceof Error ? error.message : '智能优化失败，请重新提交或缩短提示词。',
      'failed'
    )
    throw error instanceof Error ? error : new Error('智能优化失败，请重新提交或缩短提示词。')
  }

  if (!fitsImagePromptLimits(optimizedPrompt)) {
    appendProcessStep(
      '优化结果仍超过图片模型限制',
      `优化后 ${optimizedPrompt.length} 字、${promptByteLength(optimizedPrompt)} 字节，未发送给图片模型。`,
      'failed'
    )
    throw new Error('智能优化后的提示词仍超过图片模型限制，请缩短内容后重试。')
  }

  appendProcessStep('最终发送给图片模型的提示词', optimizedPrompt, 'completed')
  return {
    content: optimizedPrompt,
    isOptimized: true,
    originalPrompt: content,
    optimizedPrompt
  }
}

function fitsImagePromptLimits(content: string) {
  return content.length <= IMAGE_PROMPT_LIMIT && promptByteLength(content) <= IMAGE_PROMPT_BYTE_LIMIT
}

function promptByteLength(content: string) {
  return new TextEncoder().encode(content).length
}

async function sendAnswer() {
  if (sendingAnswer.value || !activeConversationId.value || !answerText.value.trim()) return
  const conversationId = activeConversationId.value
  const rawAnswer = answerText.value.trim()
  const questionsSnapshot = [...pendingQuestions.value]
  const answeredQuestionIds = questionsSnapshot.map(question => question.id)
  const mergedContent = buildQuestionAnswerPrompt(smartQaOriginalPrompt.value, questionsSnapshot, rawAnswer)
  let requestSubmitted = false
  sendingAnswer.value = true
  try {
    appendProcessStep('已收到补充回答', rawAnswer, 'completed')
    const promptPayload = await prepareImagePromptForSend(mergedContent, false)
    answerText.value = ''
    messages.value.push(createLocalMessage(conversationId, 'answer_to_questions', rawAnswer))
    requestSubmitted = true
    showLocalThinking('frontend_dispatch', '已提交补充回答，继续执行生成流程。')
    appendProcessStep('已提交图片模型', promptPayload.content, 'completed')

    const data = await apiFetch<SendMessageResponse>(`/api/conversations/${conversationId}/messages`, {
      method: 'POST',
      body: JSON.stringify({
        input_type: 'answer_to_questions',
        task_type: 'image_generation',
        content: promptPayload.content,
        text_model_config_id: selectedTextModelId.value,
        image_model_config_id: selectedImageModelId.value,
        original_prompt: smartQaOriginalPrompt.value,
        answered_question_ids: answeredQuestionIds,
        is_optimized: promptPayload.isOptimized,
        optimized_prompt: promptPayload.optimizedPrompt,
        stream: true,
        return_reasoning: true
      })
    })
    pendingQuestions.value = []
    resetSmartQaState()
    await applySendResponse(data)
  } catch (error) {
    if (!requestSubmitted) {
      answerText.value = rawAnswer
    }
    appendErrorMessage(error)
  } finally {
    sendingAnswer.value = false
  }
}

async function applySendResponse(data: SendMessageResponse) {
  updateConversationTitle(data.conversation)
  registerRunMeta([data.agent_run])
  replaceLocalUserMessage(data.user_message)
  pendingQuestions.value = data.follow_up_questions || []
  if (pendingQuestions.value.length) {
    appendProcessStep('等待用户补充回答', formatQuestions(pendingQuestions.value), 'completed')
  }
  if (data.artifacts) {
    artifacts.value = data.artifacts
    if (data.artifacts.length) {
      rightTab.value = 'artifacts'
      appendProcessStep('图片已生成，右侧显示', `共 ${data.artifacts.length} 个产物。`, 'completed')
    }
  }
  activeRunId.value = data.agent_run.id
  runStatus.value = statusLabel(data.agent_run.status)
  agentSteps.value = data.agent_steps || []
  if (data.assistant_message) {
    messages.value.push({
      ...data.assistant_message,
      thinking_content: data.model_output?.thinking_content || collectThinkingContent(agentSteps.value)
    })
  }
  await loadConversations()
}

function registerRunMeta(runs: AgentRun[]) {
  if (!runs.length) return
  runMetaMap.value = runs.reduce<Record<number, AgentRun>>((acc, run) => {
    acc[run.id] = run
    return acc
  }, { ...runMetaMap.value })
}

function replaceLocalUserMessage(userMessage?: Message) {
  if (!userMessage) return
  const localIndex = [...messages.value]
    .reverse()
    .findIndex(message => message.id < 0 && message.role === 'user')
  if (localIndex >= 0) {
    const targetIndex = messages.value.length - 1 - localIndex
    messages.value[targetIndex] = userMessage
    return
  }
  if (!messages.value.some(message => message.id === userMessage.id)) {
    messages.value.push(userMessage)
  }
}

async function saveComposerModelSelection(event: Event) {
	const target = event.target as HTMLSelectElement
	const selectedId = Number(target.value)
	const selection = modelSelection.value
	if (!selection) return
	modelSelection.value = {
		...selection,
		text_model_config_id: taskType.value === 'text_chat' ? selectedId : selection.text_model_config_id,
		image_model_config_id: taskType.value === 'image_generation' ? selectedId : selection.image_model_config_id
	}
}

async function loadRunEvents() {
  if (!activeRunId.value) return
  agentSteps.value = await fetchRunSteps(activeRunId.value)
  rightTab.value = 'steps'
}

async function fetchRunSteps(runID: number) {
  try {
    const data = await apiFetch<{ steps: AgentStep[] }>(`/api/runs/${runID}/steps`)
    return data.steps || []
  } catch (error) {
    console.error('Fetch run steps error:', error)
    return []
  }
}

async function hydrateThinkingMessages(sourceMessages: Message[]) {
  const stepsCache = new Map<number, AgentStep[]>()
  const hydrated: Message[] = []
  for (const message of sourceMessages) {
    if (message.role === 'assistant' && message.agent_run_id) {
      let steps = stepsCache.get(message.agent_run_id)
      if (!steps) {
        steps = await fetchRunSteps(message.agent_run_id)
        stepsCache.set(message.agent_run_id, steps)
      }
      hydrated.push({ ...message, thinking_content: collectThinkingContent(steps) })
      continue
    }
    hydrated.push(message)
  }
  return hydrated
}

function collectThinkingContent(steps: AgentStep[]) {
  return steps
    .flatMap(step => {
      const parts: string[] = []
      if (step.think_content) {
        parts.push(`${step.name} 业务思考：${step.think_content}`)
      }
      if (step.reasoning_content) {
        parts.push(`${step.name} 模型推理：${step.reasoning_content}`)
      }
      return parts
    })
    .join('\n\n')
}

function appendProcessStep(title: string, detail = '', status: ProcessTimelineItem['status'] = 'completed') {
  rightTab.value = 'artifacts'
  processTimeline.value.push({
    id: Date.now() + processTimeline.value.length,
    title,
    detail,
    status
  })
}

function completeLastRunningProcessStep(
  title: string,
  detail = '',
  status: ProcessTimelineItem['status'] = 'completed'
) {
  const index = [...processTimeline.value].reverse().findIndex(item => item.status === 'running')
  if (index === -1) {
    appendProcessStep(title, detail, status)
    return
  }
  const targetIndex = processTimeline.value.length - 1 - index
  processTimeline.value[targetIndex] = {
    ...processTimeline.value[targetIndex],
    title,
    detail,
    status
  }
}

function resetProcessTimeline() {
  processTimeline.value = []
}

function resetSmartQaState() {
  smartQaOriginalPrompt.value = ''
  smartQaActive.value = false
}

function formatQuestions(questions: FollowUpQuestion[]) {
  if (!questions.length) return '未生成新的补充问题。'
  return questions.map((question, index) => `${index + 1}. ${question.question}`).join('\n')
}

function buildQuestionAnswerPrompt(
  originalPrompt: string,
  questions: FollowUpQuestion[],
  answer: string
) {
  const sourcePrompt = originalPrompt.trim()
  if (!sourcePrompt || !questions.length) return answer

  return [
    '原始需求：',
    sourcePrompt,
    '',
    '补充问题与回答：',
    ...questions.map((question, index) => `${index + 1}. ${question.question}\n回答：${answer}`)
  ].join('\n')
}

function createLocalMessage(
  conversationId: number,
  inputType: string,
  content: string,
  options: { isOptimized?: boolean; originalPrompt?: string; optimizedPrompt?: string } = {}
): Message {
  return {
    id: -Date.now(),
    conversation_id: conversationId,
    user_id: 0,
    role: 'user',
    input_type: inputType,
    content,
    is_optimized: Boolean(options.isOptimized),
    original_prompt: options.originalPrompt || '',
    optimized_prompt: options.optimizedPrompt || '',
    agent_run_id: 0,
    created_at: Math.floor(Date.now() / 1000),
    updated_at: Math.floor(Date.now() / 1000)
  }
}

function showLocalThinking(name: string, content: string) {
  agentSteps.value = [{
    id: -Date.now(),
    agent_run_id: activeRunId.value || 0,
    name,
    status: 'running',
    input: '',
    output: '',
    think_content: content,
    reasoning_content: '',
    error_message: ''
  }]
}

function appendErrorMessage(error: unknown) {
  const content = error instanceof Error ? error.message : '请求失败'
  const conversationId = activeConversationId.value || 0
  messages.value.push({
    id: -Date.now(),
    conversation_id: conversationId,
    user_id: 0,
    role: 'assistant',
    input_type: 'error',
    content,
    agent_run_id: 0,
    created_at: Math.floor(Date.now() / 1000),
    updated_at: Math.floor(Date.now() / 1000)
  })
}

function updateConversationTitle(conversation?: Conversation) {
  if (!conversation) return
  const index = conversations.value.findIndex(item => item.id === conversation.id)
  if (index >= 0) {
    conversations.value[index] = { ...conversations.value[index], title: conversation.title }
  }
}

async function downloadArtifactFile(artifact: Artifact) {
  await downloadArtifact(artifact.id, artifact.name)
}

function startResize(side: 'left' | 'right') {
  resizing.value = side
  document.addEventListener('mousemove', onResize)
  document.addEventListener('mouseup', stopResize)
  document.body.style.cursor = 'col-resize'
}

function onResize(e: MouseEvent) {
  if (!resizing.value) return
  const container = document.querySelector('.chat-page.with-left-sidebar')
  if (!container) return
  const containerRect = container.getBoundingClientRect()
  if (resizing.value === 'left') {
    const newWidth = e.clientX - containerRect.left
    sidebarWidth.value = Math.min(Math.max(newWidth, 220), 400)
    return
  }
  const newWidth = containerRect.right - e.clientX
  panelWidth.value = Math.min(Math.max(newWidth, 320), 620)
}

function stopResize() {
  resizing.value = null
  document.removeEventListener('mousemove', onResize)
  document.removeEventListener('mouseup', stopResize)
  document.body.style.cursor = ''
}

function scrollToMessage(index: number) {
  activeMessageIndex.value = index
  const messagesContainer = document.querySelector('.messages')
  const messageElements = messagesContainer?.querySelectorAll('.message')
  if (messageElements && messageElements[index]) {
    messageElements[index].scrollIntoView({ behavior: 'smooth', block: 'center' })
    messageElements[index].classList.add('highlighted')
    setTimeout(() => messageElements[index]?.classList.remove('highlighted'), 1500)
  }
}

function truncateContent(content: string, maxLength = 60) {
  if (content.length <= maxLength) return content
  return content.slice(0, maxLength) + '...'
}

function runMetaForMessage(message: Message) {
  return message.agent_run_id ? runMetaMap.value[message.agent_run_id] : undefined
}

function dialogTypeLabel(message: Message) {
  const run = runMetaForMessage(message)
  const taskType = run?.task_type || run?.intent || ''
  if (message.input_type === 'answer_to_questions') return '补充回答'
  if (message.input_type === 'follow_up_questions') return '智能问答'
  if (taskType === 'image_generation') return '图片问答'
  if (taskType === 'html_generation') return 'HTML 问答'
  if (taskType === 'mixed_generation') return '混合问答'
  if (taskType === 'text_chat') return '文本问答'
  return inputTypeLabel(message.input_type)
}

function messageModelName(message: Message) {
  const run = runMetaForMessage(message)
  if (message.input_type === 'follow_up_questions' && run?.text_model_name) {
    return run.text_model_name
  }
  if ((run?.task_type || run?.intent) === 'image_generation' && run?.image_model_name) {
    return run.image_model_name
  }
  if (run?.text_model_name) return run.text_model_name
  if (run?.image_model_name) return run.image_model_name

  const selection = modelSelection.value
  if (!selection) return '模型待记录'
  const options = [...selection.text_models, ...selection.image_models]
  const modelID = taskType.value === 'image_generation' ? selection.image_model_config_id : selection.text_model_config_id
  return options.find(item => item.id === modelID)?.model_name || '模型待记录'
}

function formatMessageTime(timestamp?: number) {
  if (!timestamp) return '时间待记录'
  return new Date(timestamp * 1000).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function roleLabel(role: string) {
  if (role === 'user') return '用户'
  if (role === 'assistant') return '助手'
  if (role === 'system') return '系统'
  return role
}

function inputTypeLabel(inputType: string) {
  const labels: Record<string, string> = {
    normal: '普通输入',
    answer_to_questions: '追问回答',
    follow_up_questions: '追问',
    agent_result: '生成结果',
    error: '错误'
  }
  return labels[inputType] || inputType
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    running: '运行中',
    completed: '已完成',
    failed: '失败',
    waiting_questions: '等待补充信息'
  }
  return labels[status] || status || '就绪'
}
</script>
