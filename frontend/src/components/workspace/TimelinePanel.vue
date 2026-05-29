<template>
  <section class="v2-timeline">
    <header v-if="steps.length">
      <strong>运行过程</strong>
      <small>{{ steps.length }} 个步骤 · {{ toolInvocations.length }} 次工具调用</small>
    </header>
    <ol v-if="steps.length">
      <li v-for="step in steps" :key="step.id" :class="step.status">
        <div class="v2-step-head">
          <strong>{{ stepNameLabel(step) }}</strong>
          <span>{{ statusLabel(step.status) }}</span>
          <small v-if="step.attempt">第 {{ step.attempt }} 次</small>
          <small v-if="step.duration_ms">{{ step.duration_ms }}ms</small>
          <small v-if="providerLabelForStep(step)">{{ providerLabelForStep(step) }}</small>
        </div>

        <details class="v2-thinking-box">
          <summary>思考与执行</summary>
          <div class="v2-thinking-content">
            <p v-if="thinkingForStep(step)">{{ thinkingForStep(step) }}</p>
            <p v-else class="muted">该步骤没有返回单独的思考内容。</p>
          </div>
        </details>

        <section class="v2-step-result">
          <strong>输出</strong>
          <p>{{ stepOutput(step) }}</p>
          <dl v-if="outputDetails(step).length">
            <template v-for="item in outputDetails(step)" :key="item.label">
              <dt>{{ item.label }}</dt>
              <dd>{{ item.value }}</dd>
            </template>
          </dl>
        </section>
        <p v-if="errorLabelForStep(step)" class="muted">{{ errorLabelForStep(step) }}</p>
      </li>
    </ol>
    <div v-else class="v2-empty-state">
      <strong>{{ emptyTitle }}</strong>
      <p class="muted">{{ emptyText }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AgentRun, AgentStep, TaskLedgerItem, ToolInvocation } from '../../types'

const props = defineProps<{
  activeRun: AgentRun | null
  steps: AgentStep[]
  taskLedgerItems: TaskLedgerItem[]
  toolInvocations: ToolInvocation[]
}>()

const emptyTitle = computed(() => {
  const status = props.activeRun?.status || 'empty'
  if (status === 'queued') return '运行已排队'
  if (status === 'running') return '运行正在启动'
  if (status === 'waiting_user') return '等待补充信息'
  if (status === 'failed') return '运行失败'
  if (status === 'cancelled') return '运行已取消'
  if (status === 'completed') return '运行已完成'
  return '暂无运行记录'
})

const emptyText = computed(() => {
  if (props.activeRun?.error_message) return props.activeRun.error_message
  return '提交图片需求后，步骤、工具调用和错误会显示在这里。'
})

function parsedStepResult(step: AgentStep) {
  if (!step.output_json) return null
  try {
    return JSON.parse(step.output_json) as { summary?: string; output?: Record<string, unknown> }
  } catch {
    return null
  }
}

function summarizeStep(step: AgentStep) {
  const payload = parsedStepResult(step)
  return localizeSummary(payload?.summary || step.output || step.error_message || '已写入结构化输出')
}

function thinkingForStep(step: AgentStep) {
  return [step.think_content, step.reasoning_content].filter(Boolean).join('\n\n')
}

function stepOutput(step: AgentStep) {
  return step.output || summarizeStep(step) || step.error_message || '暂无输出内容'
}

function outputDetails(step: AgentStep) {
  const output = parsedStepResult(step)?.output
  if (!output) return []
  const details: Array<{ label: string; value: string }> = []
  const add = (label: string, key: string) => {
    const value = output[key]
    if (typeof value === 'string' && value.trim()) details.push({ label, value })
    if (Array.isArray(value) && value.length) details.push({ label, value: value.map(String).join('、') })
    if (typeof value === 'number') details.push({ label, value: String(value) })
    if (typeof value === 'boolean') details.push({ label, value: value ? '是' : '否' })
  }
  add('主体', 'subject')
  add('场景', 'scene')
  add('风格', 'style')
  add('比例', 'aspect_ratio')
  add('正向提示词', 'positive_prompt')
  add('负向提示词', 'negative_prompt')
  add('输入图片ID', 'input_artifact_ids')
  add('输入图片数', 'image_ref_count')
  add('产物数量', 'artifact_count')
  add('文字层数量', 'text_layer_count')
  add('审核问题', 'issues')
  add('是否优化', 'should_refine')
  return details.slice(0, 8)
}

function stepNameLabel(step: AgentStep) {
  const key = step.step_key || step.name
  const labels: Record<string, string> = {
    intent_router: '意图识别',
    requirement_agent: '需求提取',
    memory_agent: '记忆加载',
    prompt_agent: '提示词生成',
    pre_generation_safety_agent: '生成前安全检查',
    image_generation_agent: '图片生成',
    post_generation_safety_agent: '生成后安全检查',
    artifact_agent: '产物保存',
    poster_render_agent: '文字分层',
    vision_review_agent: '视觉审核',
    ranker_agent: '候选排序',
    refiner_agent: '自动优化'
  }
  return labels[key] || key || '未知步骤'
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    created: '已创建',
    queued: '排队中',
    running: '运行中',
    waiting_user: '等待补充',
    completed: '已完成',
    failed: '失败',
    retrying: '重试中',
    cancelled: '已取消',
    skipped: '已跳过'
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

function toolForStep(step: AgentStep) {
  return props.toolInvocations.find(tool => tool.agent_step_id === step.id) || null
}

function ledgerForStep(step: AgentStep) {
  return props.taskLedgerItems.find(item => item.task_key === (step.step_key || step.name)) || null
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
</script>
