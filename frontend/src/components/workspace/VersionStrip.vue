<template>
  <section v-if="artifactId" class="v2-versions">
    <header>
      <strong>版本</strong>
      <small>{{ versions.length }}</small>
    </header>
    <button
      v-for="version in versions"
      :key="version.id"
      type="button"
      :class="{ active: version.id === selectedVersionId }"
      @click="$emit('update:selectedVersionId', version.id)"
    >
      <span>v{{ version.version_no }} · {{ operationLabel(version.operation) }}</span>
      <small>{{ version.model_provider }}/{{ version.model_name }}</small>
      <small v-if="version.parent_version_id">父版本 v{{ version.parent_version_id }}</small>
      <small v-if="version.quality_scores">质量分 {{ formatScore(parseQualityScores(version.quality_scores)?.overall_score) }}</small>
    </button>
  </section>
</template>

<script setup lang="ts">
import type { ArtifactVersion, QualityScores } from '../../types'

defineProps<{
  artifactId: number
  versions: ArtifactVersion[]
  selectedVersionId: number
}>()

defineEmits<{
  'update:selectedVersionId': [value: number]
}>()

function parseQualityScores(raw?: string): QualityScores | null {
  if (!raw) return null
  try {
    return JSON.parse(raw) as QualityScores
  } catch {
    return null
  }
}

function formatScore(score?: number) {
  if (typeof score !== 'number') return '-'
  return `${Math.round(score * 100)}`
}

function operationLabel(operation: string) {
  const labels: Record<string, string> = {
    generate: '生成',
    refine: '优化',
    edit: '编辑',
    upload: '上传',
    render_text: '文字分层'
  }
  return labels[operation] || operation || '未知操作'
}
</script>
