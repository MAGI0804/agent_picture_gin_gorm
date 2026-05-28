import { apiFetch, downloadV2Artifact, fetchV2ArtifactPreviewURL } from '../api'
import type { Artifact, ArtifactVersion } from '../types'

export function listConversationArtifacts(conversationId: number) {
  return apiFetch<{ artifacts: Artifact[] }>(`/api/v2/conversations/${conversationId}/artifacts`)
}

export function listArtifactVersions(artifactId: number) {
  return apiFetch<{ versions: ArtifactVersion[] }>(`/api/v2/artifacts/${artifactId}/versions`)
}

export function uploadConversationArtifact(conversationId: number, file: File) {
  const form = new FormData()
  form.set('file', file)
  return apiFetch<{ artifact: Artifact; version: ArtifactVersion }>(
    `/api/v2/conversations/${conversationId}/artifacts/upload`,
    { method: 'POST', body: form }
  )
}

export function editArtifactVersion(artifactId: number, versionId: number, prompt: string, imageModelConfigId: number) {
  return apiFetch<{ version: ArtifactVersion }>(`/api/v2/artifacts/${artifactId}/edit`, {
    method: 'POST',
    body: JSON.stringify({
      artifact_version_id: versionId,
      prompt,
      image_model_config_id: imageModelConfigId
    })
  })
}

export function renderArtifactText(
  artifactId: number,
  versionId: number,
  title: string,
  subtitle: string,
  brand: string
) {
  return apiFetch<{ artifact: Artifact; version: ArtifactVersion }>(`/api/v2/artifacts/${artifactId}/render-text`, {
    method: 'POST',
    body: JSON.stringify({
      artifact_version_id: versionId,
      title,
      subtitle,
      brand
    })
  })
}

export function selectArtifactVersion(artifactId: number, versionId: number) {
  return apiFetch<{ selected: boolean }>(`/api/v2/artifacts/${artifactId}/select`, {
    method: 'POST',
    body: JSON.stringify({ artifact_version_id: versionId })
  })
}

export function recordArtifactFeedback(
  artifactId: number,
  versionId: number,
  feedbackType: string,
  rating: number,
  comment: string
) {
  return apiFetch<{ recorded: boolean }>(`/api/v2/artifacts/${artifactId}/feedback`, {
    method: 'POST',
    body: JSON.stringify({
      artifact_version_id: versionId,
      feedback_type: feedbackType,
      rating,
      comment
    })
  })
}

export { downloadV2Artifact, fetchV2ArtifactPreviewURL }
