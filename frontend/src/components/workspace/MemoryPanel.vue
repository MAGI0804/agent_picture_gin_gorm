<template>
  <section class="v2-memory-panel">
    <header>
      <div>
            <strong>记忆</strong>
        <span>{{ memories.length }} 条</span>
      </div>
      <button type="button" :disabled="loading || !conversationId" @click="$emit('refresh')">刷新</button>
    </header>
    <label>
      状态
      <select :value="statusFilter" @change="$emit('update:statusFilter', ($event.target as HTMLSelectElement).value)">
        <option value="">全部</option>
        <option value="proposal">候选</option>
        <option value="stable">稳定</option>
      </select>
    </label>
    <label>
      命名空间
      <select :value="namespace" :disabled="loading" @change="$emit('namespaceChange', ($event.target as HTMLSelectElement).value)">
        <option value="">全部</option>
        <option value="conversation">会话</option>
        <option value="user_profile">用户画像</option>
        <option value="visual_style">视觉风格</option>
        <option value="artifact_lineage">产物链路</option>
        <option value="tool_experience">工具经验</option>
        <option value="reflection">复盘</option>
      </select>
    </label>
    <ul v-if="displayedMemories.length" class="v2-memory-list">
      <li v-for="memory in displayedMemories" :key="memory.id">
        <div>
          <strong>{{ memory.namespace || memory.kind }}</strong>
          <p>{{ memory.content }}</p>
          <small>
            {{ formatConfidence(memory.confidence) }} · 使用 {{ memory.use_count || 0 }} 次
            <span v-if="isMemoryProposal(memory)" class="v2-memory-proposal-badge">候选</span>
          </small>
          <small v-if="memory.source_type || memory.artifact_id">
            {{ memory.source_type || '来源' }} #{{ memory.source_id || memory.artifact_id }}
          </small>
        </div>
        <div class="v2-memory-actions">
          <button
            v-if="isMemoryProposal(memory)"
            type="button"
            :disabled="promotingMemoryId === memory.id"
            @click="$emit('promote', memory.id)"
          >
            {{ promotingMemoryId === memory.id ? '确认中...' : '确认' }}
          </button>
          <button type="button" @click="$emit('edit', memory)">编辑</button>
          <button type="button" @click="$emit('delete', memory.id)">删除</button>
        </div>
      </li>
    </ul>
    <p v-else class="muted">暂无记忆。</p>
  </section>
</template>

<script setup lang="ts">
import type { ContextMemory } from '../../types'

defineProps<{
  conversationId: number
  memories: ContextMemory[]
  displayedMemories: ContextMemory[]
  namespace: string
  statusFilter: string
  loading: boolean
  promotingMemoryId: number
}>()

defineEmits<{
  refresh: []
  namespaceChange: [value: string]
  'update:statusFilter': [value: string]
  promote: [id: number]
  edit: [memory: ContextMemory]
  delete: [id: number]
}>()

function formatConfidence(confidence?: number) {
  if (typeof confidence !== 'number' || confidence <= 0) return '置信度 -'
  return `置信度 ${Math.round(confidence * 100)}`
}

function isMemoryProposal(memory: ContextMemory) {
  return memory.kind === 'memory_proposal'
}
</script>
