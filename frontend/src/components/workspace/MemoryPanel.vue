<template>
  <section class="v2-memory-panel">
    <header>
      <div>
        <strong>Memory</strong>
        <span>{{ memories.length }} 条</span>
      </div>
      <button type="button" :disabled="loading || !conversationId" @click="$emit('refresh')">刷新</button>
    </header>
    <label>
      Status
      <select :value="statusFilter" @change="$emit('update:statusFilter', ($event.target as HTMLSelectElement).value)">
        <option value="">all</option>
        <option value="proposal">proposal</option>
        <option value="stable">stable</option>
      </select>
    </label>
    <label>
      Namespace
      <select :value="namespace" :disabled="loading" @change="$emit('namespaceChange', ($event.target as HTMLSelectElement).value)">
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
  if (typeof confidence !== 'number' || confidence <= 0) return 'confidence -'
  return `confidence ${Math.round(confidence * 100)}`
}

function isMemoryProposal(memory: ContextMemory) {
  return memory.kind === 'memory_proposal'
}
</script>
