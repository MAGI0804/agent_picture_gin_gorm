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
      <span>v{{ version.version_no }} · {{ version.operation }}</span>
      <small>{{ version.model_provider }}/{{ version.model_name }}</small>
      <small v-if="version.parent_version_id">parent v{{ version.parent_version_id }}</small>
      <small v-if="version.quality_scores">score {{ formatScore(parseQualityScores(version.quality_scores)?.overall_score) }}</small>
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
</script>
