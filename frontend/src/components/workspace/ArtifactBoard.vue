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
      <span v-else class="v2-artifact-fallback">{{ artifactLabel(artifact.kind) }}</span>
      <strong>{{ index + 1 }}</strong>
      <small>{{ artifact.name }}</small>
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

function isPreviewableArtifact(artifact: Artifact) {
  return artifact.kind === 'image' || artifact.kind === 'svg'
}

function artifactLabel(kind: string) {
  if (kind === 'svg') return 'SVG'
  if (kind === 'image') return 'IMG'
  return kind || 'FILE'
}
</script>
