<template>
  <section class="v2-artifact-grid">
    <button
      v-for="(artifact, index) in artifacts"
      :key="artifact.id"
      type="button"
      class="v2-artifact-item"
      :class="{
        active: artifact.id === selectedArtifactId,
        chosen: Boolean(artifact.selected_at),
        recommended: artifact.id === recommendedArtifactId,
        comparing: compareIds.includes(artifact.id)
      }"
      @click="$emit('select', artifact)"
    >
      <img v-if="isPreviewableArtifact(artifact) && previewURLs[artifact.id]" :src="previewURLs[artifact.id]" :alt="artifact.name" />
      <span v-else>{{ artifact.kind }}</span>
      <strong>{{ artifact.name }}</strong>
      <small>{{ artifact.mime_type }}</small>
      <div class="v2-artifact-metrics">
        <small>#{{ index + 1 }}</small>
        <small>Rank {{ formatRankScore(artifact.rank_score) }}</small>
      </div>
      <div class="v2-artifact-badges">
        <small v-if="artifact.id === recommendedArtifactId" class="v2-recommended-badge">推荐</small>
        <small v-if="artifact.selected_at" class="v2-selected-badge">已选中</small>
        <small v-if="compareIds.includes(artifact.id)" class="v2-selected-badge">对比</small>
      </div>
      <label class="v2-compare-toggle" @click.stop>
        <input
          type="checkbox"
          :checked="compareIds.includes(artifact.id)"
          @change="$emit('toggleCompare', artifact.id)"
        />
        对比
      </label>
    </button>
    <p v-if="!artifacts.length" class="muted">暂无产物。</p>
  </section>
</template>

<script setup lang="ts">
import type { Artifact } from '../../types'

defineProps<{
  artifacts: Artifact[]
  selectedArtifactId: number
  recommendedArtifactId: number
  compareIds: number[]
  previewURLs: Record<number, string>
}>()

defineEmits<{
  select: [artifact: Artifact]
  toggleCompare: [artifactId: number]
}>()

function formatRankScore(score?: number) {
  if (typeof score !== 'number') return '-'
  if (score >= 0 && score <= 1) return `${Math.round(score * 100)}`
  return score.toFixed(2)
}

function isPreviewableArtifact(artifact: Artifact) {
  return artifact.kind === 'image' || artifact.kind === 'svg'
}
</script>
