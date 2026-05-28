<template>
  <section v-if="artifactId" class="v2-review-panel">
    <header>
      <strong>Review / Eval</strong>
      <small>{{ reviewStatusText }}</small>
    </header>
    <div v-if="qualityScores" class="v2-score-block">
      <div>
        <span>质量分</span>
        <strong>{{ formatScore(qualityScores.overall_score) }}</strong>
      </div>
      <div>
        <span>Requirement</span>
        <strong>{{ formatScore(qualityScores.requirement_match) }}</strong>
      </div>
      <div>
        <span>Composition</span>
        <strong>{{ formatScore(qualityScores.composition_score) }}</strong>
      </div>
      <div>
        <span>Text</span>
        <strong>{{ formatScore(qualityScores.text_readability) }}</strong>
      </div>
      <div>
        <span>Layout</span>
        <strong>{{ formatScore(qualityScores.layout_score) }}</strong>
      </div>
      <div>
        <span>Refine</span>
        <strong>{{ qualityScores.should_refine ? '需要' : '不需要' }}</strong>
      </div>
      <div>
        <span>排序分</span>
        <strong>{{ formatRankScore(qualityScores.rank_score ?? artifactRankScore) }}</strong>
      </div>
    </div>
    <ul v-if="qualityScores?.issues?.length" class="v2-issue-list">
      <li v-for="issue in qualityScores.issues" :key="issue">{{ issue }}</li>
    </ul>
    <p v-else-if="!qualityScores" class="muted">暂无版本质量分。</p>
    <p v-if="qualityScores?.extracted_text" class="muted">
      OCR: {{ qualityScores.extracted_text }}
    </p>
    <details v-if="reviewSummary" class="v2-step-detail">
      <summary>vision_review_agent</summary>
      <p>{{ reviewSummary }}</p>
    </details>
  </section>
</template>

<script setup lang="ts">
import type { QualityScores } from '../../types'

defineProps<{
  artifactId: number
  qualityScores: QualityScores | null
  reviewStatusText: string
  reviewSummary: string
  artifactRankScore?: number
}>()

function formatScore(score?: number) {
  if (typeof score !== 'number') return '-'
  return `${Math.round(score * 100)}`
}

function formatRankScore(score?: number) {
  if (typeof score !== 'number') return '-'
  if (score >= 0 && score <= 1) return `${Math.round(score * 100)}`
  return score.toFixed(2)
}
</script>
