import { computed, ref } from 'vue'
import type { Artifact, ArtifactVersion } from '../types'

export function useArtifacts() {
  const artifacts = ref<Artifact[]>([])
  const selectedArtifact = ref<Artifact | null>(null)
  const versions = ref<ArtifactVersion[]>([])
  const selectedVersionId = ref(0)
  const previewURLs = ref<Record<number, string>>({})

  const rankedArtifacts = computed(() => {
    return [...artifacts.value].sort((left, right) => {
      const rankDelta = (right.rank_score || 0) - (left.rank_score || 0)
      if (rankDelta !== 0) return rankDelta
      return right.id - left.id
    })
  })
  const recommendedArtifactId = computed(() => rankedArtifacts.value[0]?.id || 0)
  const selectedVersion = computed(() => versions.value.find(item => item.id === selectedVersionId.value) || null)

  function cleanupPreviewURLs(items: Artifact[]) {
    const keep = new Set(items.map(item => item.id))
    const next = { ...previewURLs.value }
    for (const [id, url] of Object.entries(previewURLs.value)) {
      if (!keep.has(Number(id))) {
        if (url) URL.revokeObjectURL(url)
        delete next[Number(id)]
      }
    }
    previewURLs.value = next
  }

  function revokeAllPreviewURLs() {
    Object.values(previewURLs.value).forEach(url => {
      if (url) URL.revokeObjectURL(url)
    })
    previewURLs.value = {}
  }

  return {
    artifacts,
    selectedArtifact,
    versions,
    selectedVersionId,
    previewURLs,
    rankedArtifacts,
    recommendedArtifactId,
    selectedVersion,
    cleanupPreviewURLs,
    revokeAllPreviewURLs
  }
}
