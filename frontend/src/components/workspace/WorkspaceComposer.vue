<template>
  <section class="v2-composer">
    <div class="v2-model-row">
      <label>
        文本模型
        <select :value="textModelConfigId" @change="$emit('update:textModelConfigId', Number(($event.target as HTMLSelectElement).value))">
          <option :value="0">自动选择</option>
          <option v-for="item in modelSelection?.text_models || []" :key="item.id" :value="item.id">
            {{ item.model_name }}
          </option>
        </select>
      </label>
      <label>
        图片模型
        <select :value="imageModelConfigId" @change="$emit('update:imageModelConfigId', Number(($event.target as HTMLSelectElement).value))">
          <option :value="0">自动选择</option>
          <option v-for="item in modelSelection?.image_models || []" :key="item.id" :value="item.id">
            {{ item.model_name }}
          </option>
        </select>
      </label>
      <label>
        候选数
        <select :value="candidateCount" @change="$emit('update:candidateCount', Number(($event.target as HTMLSelectElement).value))">
          <option :value="1">1 张</option>
          <option :value="2">2 张</option>
          <option :value="3">3 张</option>
        </select>
      </label>
    </div>

    <label>
      图片需求
      <textarea
        :value="prompt"
        placeholder="输入图片需求，V2 会抽取需求、生成 prompt、调用图片模型并写入 artifact version。"
        @input="$emit('update:prompt', ($event.target as HTMLTextAreaElement).value)"
        @keydown.enter.ctrl.prevent="$emit('run')"
      />
    </label>

    <div class="v2-actions">
      <button class="primary" type="button" :disabled="!canRun" @click="$emit('run')">
        {{ running ? '运行中...' : '运行 V2 Agent' }}
      </button>
      <button type="button" :disabled="running" @click="$emit('clear')">清空</button>
      <button type="button" :disabled="!canRetry" @click="$emit('retry')">重试失败 Run</button>
      <span v-if="errorMessage">{{ errorMessage }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { ModelSelection } from '../../types'

defineProps<{
  modelSelection: ModelSelection | null
  textModelConfigId: number
  imageModelConfigId: number
  candidateCount: number
  prompt: string
  running: boolean
  canRun: boolean
  canRetry: boolean
  errorMessage: string
}>()

defineEmits<{
  'update:textModelConfigId': [value: number]
  'update:imageModelConfigId': [value: number]
  'update:candidateCount': [value: number]
  'update:prompt': [value: string]
  run: []
  clear: []
  retry: []
}>()
</script>
