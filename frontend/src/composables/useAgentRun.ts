import { computed, ref } from 'vue'
import type { AgentRun } from '../types'

export const terminalRunStatuses = ['completed', 'failed', 'cancelled']

export function useAgentRun() {
  const activeRun = ref<AgentRun | null>(null)
  const activeRunId = computed(() => activeRun.value?.id || 0)
  const runStatusText = computed(() => runStatusLabel(activeRun.value?.status || 'ready'))
  const canCancelRun = computed(() => {
    const status = activeRun.value?.status || ''
    return ['created', 'queued', 'running', 'waiting_user'].includes(status)
  })

  function isTerminalRunStatus(status: string) {
    return terminalRunStatuses.includes(status)
  }

  return {
    activeRun,
    activeRunId,
    runStatusText,
    canCancelRun,
    isTerminalRunStatus,
    runStatusLabel
  }
}

export function runStatusLabel(status: string) {
  const labels: Record<string, string> = {
    ready: '就绪',
    empty: '未开始',
    created: '已创建',
    queued: '排队中',
    running: '运行中',
    waiting_user: '等待补充信息',
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消'
  }
  return labels[status] || status
}
