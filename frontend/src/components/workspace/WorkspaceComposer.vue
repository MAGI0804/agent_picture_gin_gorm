<template>
  <section class="v2-composer">
    <label class="v2-composer-input">
      <span class="sr-only">图片需求</span>
      <textarea
        :value="prompt"
        placeholder="输入图片需求，Ctrl + Enter 发送"
        @input="$emit('update:prompt', ($event.target as HTMLTextAreaElement).value)"
        @keydown.enter.ctrl.prevent="$emit('run')"
      />
      <div class="v2-inline-tools">
        <button class="v2-tool-image" type="button" aria-label="上传图片" title="上传图片" @click="uploadDialogOpen = true">▧</button>
        <button class="v2-tool-clear" type="button" aria-label="清空" title="清空" @click="$emit('clear')">×</button>
      </div>
    </label>

    <div class="v2-actions">
      <div class="v2-model-row">
        <select aria-label="文本模型" :value="textModelConfigId" @change="$emit('update:textModelConfigId', Number(($event.target as HTMLSelectElement).value))">
          <option :value="0">自动文本</option>
          <option v-for="item in modelSelection?.text_models || []" :key="item.id" :value="item.id">
            {{ item.model_name }}
          </option>
        </select>
        <select aria-label="图片模型" :value="imageModelConfigId" @change="$emit('update:imageModelConfigId', Number(($event.target as HTMLSelectElement).value))">
          <option :value="0">自动图片</option>
          <option v-for="item in modelSelection?.image_models || []" :key="item.id" :value="item.id">
            {{ item.model_name }}
          </option>
        </select>
        <select aria-label="候选数" :value="candidateCount" @change="$emit('update:candidateCount', Number(($event.target as HTMLSelectElement).value))">
          <option :value="1">1 张</option>
          <option :value="2">2 张</option>
          <option :value="3">3 张</option>
        </select>
        <label class="v2-toggle-line">
          <input
            type="checkbox"
            :checked="disableClarification"
            @change="$emit('update:disableClarification', ($event.target as HTMLInputElement).checked)"
          />
          <span>不追问</span>
        </label>
      </div>
      <button class="v2-action-icon" type="button" :disabled="running" @click="$emit('clear')" aria-label="清空" title="清空">×</button>
      <button class="v2-action-icon" type="button" :disabled="!canRetry" @click="$emit('retry')" aria-label="重试" title="重试">↻</button>
      <button class="primary v2-send-button" type="button" :disabled="!canRun" aria-label="发送" @click="$emit('run')">
        <span>{{ running ? '...' : '→' }}</span>
      </button>
      <span v-if="errorMessage">{{ errorMessage }}</span>
    </div>

    <div v-if="uploadDialogOpen" class="v2-upload-backdrop" @click.self="uploadDialogOpen = false">
      <section class="v2-upload-dialog" role="dialog" aria-modal="true" aria-label="上传参考图片">
        <header>
          <strong>上传参考图片</strong>
          <button type="button" aria-label="关闭" @click="uploadDialogOpen = false">×</button>
        </header>
        <label>
          图片文件
          <input type="file" accept="image/png,image/jpeg,image/gif" @change="selectUploadFile" />
        </label>
        <p v-if="selectedUploadName" class="muted">{{ selectedUploadName }}</p>
        <div class="v2-actions">
          <button type="button" @click="uploadDialogOpen = false">取消</button>
          <button class="primary" type="button" :disabled="!selectedUploadFile || uploading" @click="submitUpload">
            {{ uploading ? '上传中...' : '上传并选中' }}
          </button>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { ModelSelection } from '../../types'

defineProps<{
  modelSelection: ModelSelection | null
  textModelConfigId: number
  imageModelConfigId: number
  candidateCount: number
  disableClarification: boolean
  prompt: string
  running: boolean
  uploading: boolean
  canRun: boolean
  canRetry: boolean
  errorMessage: string
}>()

const emit = defineEmits<{
  'update:textModelConfigId': [value: number]
  'update:imageModelConfigId': [value: number]
  'update:candidateCount': [value: number]
  'update:disableClarification': [value: boolean]
  'update:prompt': [value: string]
  upload: [file: File]
  run: []
  clear: []
  retry: []
}>()

const uploadDialogOpen = ref(false)
const selectedUploadFile = ref<File | null>(null)
const selectedUploadName = ref('')

function selectUploadFile(event: Event) {
  const input = event.target as HTMLInputElement
  selectedUploadFile.value = input.files?.[0] || null
  selectedUploadName.value = selectedUploadFile.value?.name || ''
}

function submitUpload() {
  if (!selectedUploadFile.value) return
  emit('upload', selectedUploadFile.value)
  selectedUploadFile.value = null
  selectedUploadName.value = ''
  uploadDialogOpen.value = false
}
</script>
