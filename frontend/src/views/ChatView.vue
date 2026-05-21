<template>
  <main class="app-shell compact-shell">
    <section class="chat-page with-left-sidebar">
      <aside 
        class="conversation-sidebar" 
        :style="{ width: sidebarWidth + 'px' }"
      >
        <header class="sidebar-head">
          <div class="sidebar-brand">
            <strong title="图片 AI Agent">图片 AI Agent</strong>
            <span :title="modelSummary">{{ modelSummary }}</span>
          </div>
          <button @click="createConversation">新建</button>
        </header>
        <div class="sidebar-nav">
          <button class="active" @click="router.push('/chat')">对话</button>
          <button @click="router.push('/settings')">设置</button>
          <button @click="logout">退出</button>
        </div>
        <nav class="conversation-list">
          <button
            v-for="item in conversations"
            :key="item.id"
            :class="{ active: item.id === activeConversationId }"
            @click="openConversation(item.id)"
          >
            <span :title="item.title">{{ item.title }}</span>
            <small>#{{ item.id }}</small>
          </button>
        </nav>
      </aside>

      <div 
        class="resize-handle" 
        @mousedown="startResize('left')"
        title="拖动调整宽度"
      ></div>

      <section class="chat-left chat-main">
        <header class="chat-header">
          <div>
            <strong :title="activeTitle">{{ activeTitle }}</strong>
            <span>{{ runStatus }}</span>
          </div>
          <div class="header-actions">
            <button :disabled="!activeRunId" @click="loadRunEvents">刷新 Agent 事件</button>
          </div>
        </header>

        <div v-if="agentSteps.length" class="thinking-panel">
          <button class="thinking-toggle" @click="thinkingExpanded = !thinkingExpanded">
            <span>{{ thinkingExpanded ? '▼' : '▶' }}</span>
            Agent 思考过程
          </button>
          <div v-show="thinkingExpanded" class="thinking-content">
            <ol>
              <li v-for="step in agentSteps" :key="step.id">
                <span class="step-name">{{ step.name }}</span>
                <span class="step-status">{{ step.status }}</span>
                <p v-if="step.think_content"><b>业务思考：</b>{{ step.think_content }}</p>
                <p v-if="step.reasoning_content"><b>模型思考：</b>{{ step.reasoning_content }}</p>
              </li>
            </ol>
          </div>
        </div>

        <div class="messages">
          <article v-for="(message, index) in messages" :key="message?.id || index" :class="['message', message?.role, message.input_type]">
            <small>{{ message.role }} / {{ message.input_type }}</small>
            <p>{{ message.content }}</p>
          </article>

          <section v-if="pendingQuestions.length" class="questions">
            <strong>针对上一轮回复的问题</strong>
            <label v-for="question in pendingQuestions" :key="question.id" class="question-item">
              <input v-model="selectedQuestionIds" type="checkbox" :value="question.id" />
              <span>{{ question.question }}</span>
            </label>
            <div class="inline-answer-box">
              <label>补充问题回答框</label>
              <textarea
                v-model="answerText"
                placeholder="在这里回答上方问题，例如：两者都生成，尺寸 16:9，科技感，蓝绿色主色。按 Enter 发送，Shift + Enter 换行。"
                @keydown.enter.exact.prevent="sendAnswer"
              ></textarea>
              <button :disabled="!canSendAnswer" @click="sendAnswer">提交回答并生成</button>
            </div>
          </section>
        </div>

        <footer class="normal-composer">
          <label>正常对话框</label>
          <div class="composer-box">
            <textarea
              v-model="normalText"
              placeholder="输入新的图片、HTML 或修改需求。按 Enter 发送，Shift + Enter 换行。"
              @keydown.enter.exact.prevent="sendNormal"
            ></textarea>
            <div class="composer-tools">
              <select v-model="taskType" aria-label="任务类型">
                <option value="text_chat">文本对话</option>
                <option value="image_generation">图片生成</option>
              </select>
              <button :disabled="!canSendNormal" @click="sendNormal">发送</button>
            </div>
          </div>
        </footer>
      </section>

      <div 
        class="resize-handle" 
        @mousedown="startResize('right')"
        title="拖动调整宽度"
      ></div>

      <aside 
        class="artifact-panel" 
        :style="{ width: panelWidth + 'px' }"
      >
        <header>
          <strong>消息列表 ({{ messages.length }} 条)</strong>
          <button :disabled="!activeConversationId" @click="loadMessages">刷新</button>
        </header>

        <section class="message-list">
          <button 
            v-for="(message, index) in messages" 
            :key="message?.id || index" 
            class="message-item"
            :class="{ active: activeMessageIndex === index }"
            @click="scrollToMessage(index)"
          >
            <span class="message-role">{{ message.role }}</span>
            <span class="message-content">{{ truncateContent(message.content) }}</span>
            <span class="message-index">#{{ index + 1 }}</span>
          </button>
        </section>
      </aside>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch, downloadArtifact, getToken, setToken } from '../api'
import type { AgentRun, AgentStep, Artifact, Conversation, FollowUpQuestion, Message, ModelConfig } from '../types'

type TaskType = 'text_chat' | 'image_generation'

interface SendMessageResponse {
  user_message: Message
  assistant_message: Message
  follow_up_questions?: FollowUpQuestion[]
  artifacts?: Artifact[]
  agent_run: AgentRun
  agent_steps?: AgentStep[]
  conversation?: Conversation
}

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

const conversations = ref<Conversation[]>([])
const activeConversationId = ref<number | null>(null)
const messages = ref<Message[]>([])
const pendingQuestions = ref<FollowUpQuestion[]>([])
const selectedQuestionIds = ref<number[]>([])
const artifacts = ref<Artifact[]>([])
const agentSteps = ref<AgentStep[]>([])
const activeRunId = ref<number | null>(null)
const runStatus = ref('待命')
const answerText = ref('')
const normalText = ref('')
const taskType = ref<TaskType>('text_chat')
const modelConfig = ref<ModelConfig>({ ...defaultModelConfig })
const sendingNormal = ref(false)
const sendingAnswer = ref(false)
const sidebarWidth = ref(280)
const panelWidth = ref(420)
const resizing = ref<'left' | 'right' | null>(null)
const thinkingExpanded = ref(false)
const activeMessageIndex = ref<number | null>(null)

const activeTitle = computed(() => conversations.value.find(item => item.id === activeConversationId.value)?.title || '未选择会话')
const modelSummary = computed(() => {
  const textModel = modelConfig.value.anthropic_model || modelConfig.value.chat_model
  const imageModel = modelConfig.value.image_model || '未配置图片模型'
  return `${modelConfig.value.provider} / 文本 ${textModel} / 图片 ${imageModel}`
})
const canSendNormal = computed(() => Boolean(normalText.value.trim()) && !sendingNormal.value)
const canSendAnswer = computed(() => Boolean(activeConversationId.value && answerText.value.trim() && selectedQuestionIds.value.length) && !sendingAnswer.value)

onMounted(async () => {
  await loadModelConfig()
  await loadConversations()
})

async function loadModelConfig() {
  const data = await apiFetch<{ model_config: ModelConfig }>('/api/settings/model-config')
  modelConfig.value = { ...defaultModelConfig, ...data.model_config }
}

function logout() {
  setToken('')
  router.push('/login')
}

async function loadConversations() {
  const data = await apiFetch<{ conversations: Conversation[] }>('/api/conversations')
  conversations.value = data.conversations
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
    body: JSON.stringify({ title: '图片生成工作台' })
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
  selectedQuestionIds.value = []
  agentSteps.value = []
  await loadMessages()
  await loadArtifacts()
}

async function loadMessages() {
  if (!activeConversationId.value) return
  const data = await apiFetch<{ messages: Message[] }>(`/api/conversations/${activeConversationId.value}/messages`)
  messages.value = await hydrateThinkingMessages(data.messages)
  const lastRun = [...data.messages].reverse().find(message => message.agent_run_id)
  activeRunId.value = lastRun?.agent_run_id || null
  if (activeRunId.value) {
    agentSteps.value = await fetchRunSteps(activeRunId.value)
  }
}

async function loadArtifacts() {
  if (!activeConversationId.value) return
  const data = await apiFetch<{ artifacts: Artifact[] }>(`/api/conversations/${activeConversationId.value}/artifacts`)
  artifacts.value = data.artifacts
}

async function sendNormal() {
  if (sendingNormal.value || !normalText.value.trim()) return
  const content = normalText.value.trim()
  normalText.value = ''
  sendingNormal.value = true
  try {
    const conversationId = await ensureConversation()
    if (!conversationId) return
    messages.value.push(createLocalMessage(conversationId, taskType.value, content))
    showLocalThinking('frontend_dispatch', '已发送请求，等待后端 Agent 返回思考过程。')

    const data = await apiFetch<SendMessageResponse>(`/api/conversations/${conversationId}/messages`, {
      method: 'POST',
      body: JSON.stringify({
        input_type: 'normal',
        task_type: taskType.value,
        content,
        model_config: modelConfig.value,
        stream: true,
        return_reasoning: true
      })
    })
    await applySendResponse(data)
  } catch (error) {
    appendErrorMessage(error)
  } finally {
    sendingNormal.value = false
  }
}

async function sendAnswer() {
  if (sendingAnswer.value || !activeConversationId.value || !answerText.value.trim()) return
  const conversationId = activeConversationId.value
  const content = answerText.value.trim()
  answerText.value = ''
  sendingAnswer.value = true
  try {
    messages.value.push(createLocalMessage(conversationId, 'answer_to_questions', content))
    showLocalThinking('frontend_dispatch', '已提交补充问题答案，后端正在继续多 Agent 生成。')

    const data = await apiFetch<SendMessageResponse>(`/api/conversations/${conversationId}/messages`, {
      method: 'POST',
      body: JSON.stringify({
        input_type: 'answer_to_questions',
        task_type: 'image_generation',
        content,
        answered_question_ids: selectedQuestionIds.value,
        model_config: modelConfig.value,
        stream: true,
        return_reasoning: true
      })
    })
    pendingQuestions.value = []
    selectedQuestionIds.value = []
    await applySendResponse(data)
  } catch (error) {
    appendErrorMessage(error)
  } finally {
    sendingAnswer.value = false
  }
}

async function applySendResponse(data: SendMessageResponse) {
  updateConversationTitle(data.conversation)
  pendingQuestions.value = data.follow_up_questions || pendingQuestions.value
  selectedQuestionIds.value = pendingQuestions.value.map(question => question.id)
  if (data.artifacts) {
    artifacts.value = data.artifacts
  }
  activeRunId.value = data.agent_run.id
  runStatus.value = data.agent_run.status
  if (data.agent_steps?.length) {
    agentSteps.value = []
    for (const step of data.agent_steps) {
      agentSteps.value.push({ ...step })
      await typeStepContent(agentSteps.value.length - 1)
    }
  } else {
    await loadRunEvents()
  }
  if (data.assistant_message) {
    appendThinkingMessage(data.assistant_message.conversation_id, data.agent_run.id, agentSteps.value)
    messages.value.push(data.assistant_message)
  }
}

async function loadRunEvents() {
  if (!activeRunId.value) return
  agentSteps.value = []
  await pollRunEvents()
}

async function pollRunEvents() {
  if (!activeRunId.value) return
  let lastStepId = 0
  const maxRetries = 50
  let retries = 0
  
  while (retries < maxRetries) {
    try {
      const data = await apiFetch<{ steps: AgentStep[] }>(`/api/runs/${activeRunId.value}/steps`)
      const newSteps = data.steps.filter(step => step.id > lastStepId)
      
      for (const step of newSteps) {
        agentSteps.value.push({ ...step })
        lastStepId = step.id
        
        await typeStepContent(agentSteps.value.length - 1)
        
        if (step.status === 'completed') {
          await delay(500)
        }
      }
      
      const lastStep = data.steps[data.steps.length - 1]
      if (lastStep && (lastStep.status === 'completed' || lastStep.status === 'failed')) {
        runStatus.value = lastStep.status === 'completed' ? '已完成' : '失败'
        break
      }
      
      await delay(800)
      retries++
    } catch (error) {
      console.error('Poll run events error:', error)
      await delay(1000)
      retries++
    }
  }
}

async function hydrateThinkingMessages(sourceMessages: Message[]) {
  const hydrated: Message[] = []
  const stepsCache = new Map<number, AgentStep[]>()
  for (const message of sourceMessages) {
    if (message.role === 'assistant' && message.agent_run_id) {
      let steps = stepsCache.get(message.agent_run_id)
      if (!steps) {
        steps = await fetchRunSteps(message.agent_run_id)
        stepsCache.set(message.agent_run_id, steps)
      }
      const thinkingMessage = createThinkingMessage(message.conversation_id, message.agent_run_id, steps)
      if (thinkingMessage) {
        hydrated.push(thinkingMessage)
      }
    }
    hydrated.push(message)
  }
  return hydrated
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

function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

async function typeStepContent(index: number) {
  const step = agentSteps.value[index]
  if (!step) return
  
  const originalThink = step.think_content || ''
  const originalReasoning = step.reasoning_content || ''
  
  step.think_content = ''
  step.reasoning_content = ''
  
  if (originalThink) {
    for (let i = 0; i <= originalThink.length; i++) {
      step.think_content = originalThink.slice(0, i)
      await delay(20)
    }
  }
  
  await delay(300)
  
  if (originalReasoning) {
    for (let i = 0; i <= originalReasoning.length; i++) {
      step.reasoning_content = originalReasoning.slice(0, i)
      await delay(20)
    }
  }
}

function createLocalMessage(conversationId: number, inputType: string, content: string): Message {
  return {
    id: -Date.now(),
    conversation_id: conversationId,
    user_id: 0,
    role: 'user',
    input_type: inputType,
    content,
    agent_run_id: 0,
    created_at: Math.floor(Date.now() / 1000),
    updated_at: Math.floor(Date.now() / 1000)
  }
}

function appendThinkingMessage(conversationId: number, runId: number, steps: AgentStep[]) {
  const message = createThinkingMessage(conversationId, runId, steps)
  if (message) {
    messages.value.push(message)
  }
}

function createThinkingMessage(conversationId: number, runId: number, steps: AgentStep[]) {
  const content = steps
    .flatMap(step => {
      const lines = []
      if (step.think_content) {
        lines.push(`${step.name} 业务思考：${step.think_content}`)
      }
      if (step.reasoning_content) {
        lines.push(`${step.name} 模型思考：${step.reasoning_content}`)
      }
      return lines
    })
    .join('\n')
  if (!content) return null
  return {
    id: -Number(`${runId}001`),
    conversation_id: conversationId,
    user_id: 0,
    role: 'assistant',
    input_type: 'thinking',
    content,
    agent_run_id: runId,
    created_at: Math.floor(Date.now() / 1000),
    updated_at: Math.floor(Date.now() / 1000)
  } satisfies Message
}

function updateConversationTitle(conversation?: Conversation) {
  if (!conversation) return
  const index = conversations.value.findIndex(item => item.id === conversation.id)
  if (index >= 0) {
    conversations.value[index] = { ...conversations.value[index], title: conversation.title }
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
    sidebarWidth.value = Math.min(Math.max(newWidth, 200), 400)
  } else {
    const newWidth = containerRect.right - e.clientX
    panelWidth.value = Math.min(Math.max(newWidth, 300), 600)
  }
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
    setTimeout(() => {
      messageElements[index]?.classList.remove('highlighted')
    }, 1500)
  }
}

function truncateContent(content: string, maxLength: number = 60): string {
  if (content.length <= maxLength) return content
  return content.slice(0, maxLength) + '...'
}
</script>
