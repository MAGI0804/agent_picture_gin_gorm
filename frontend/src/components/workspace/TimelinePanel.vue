<template>
  <section class="v2-timeline">
    <header>
      <strong>Timeline</strong>
      <small>{{ steps.length }} steps · {{ toolInvocations.length }} tools</small>
    </header>
    <ol v-if="steps.length">
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
  if (status === 'queued') return 'Run 已排队'
  if (status === 'running') return 'Run 正在启动'
  if (status === 'waiting_user') return '等待补充信息'
  if (status === 'failed') return 'Run 失败'
  if (status === 'cancelled') return 'Run 已取消'
  if (status === 'completed') return 'Run 已完成'
  return '暂无运行记录'
})

const emptyText = computed(() => {
  if (props.activeRun?.error_message) return props.activeRun.error_message
  return '提交图片需求后，步骤、工具调用和错误会显示在这里。'
})

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
