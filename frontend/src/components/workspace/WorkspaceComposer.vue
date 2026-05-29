<template>
  <section
    class="v2-composer"
    :class="{ 'is-dragging': dragging }"
    @dragenter.prevent="dragging = true"
    @dragover.prevent="dragging = true"
    @dragleave.prevent="dragging = false"
    @drop.prevent="handleDrop"
  >
    <label class="v2-composer-input">
      <span class="sr-only">图片编辑需求</span>
      <textarea
        :value="prompt"
        placeholder="描述要加的文字、图标位置和样式。可直接粘贴或拖入图片，第一张作为模板。"
        @input="$emit('update:prompt', ($event.target as HTMLTextAreaElement).value)"
        @paste="handlePaste"
        @keydown.enter.ctrl.prevent="$emit('run')"
      />
    </label>

    <div v-if="attachments.length" class="v2-attachment-strip" aria-label="本次使用的图片">
      <article v-for="(item, index) in attachments" :key="item.id" class="v2-attachment">
        <img :src="item.url" :alt="item.name" />
        <div>
          <strong>{{ index === 0 ? '模板图' : `图标 ${index}` }}</strong>
          <span>{{ item.name }}</span>
        </div>
        <button type="button" :aria-label="`移除 ${item.name}`" @click="$emit('remove-attachment', item.id)">×</button>
      </article>
    </div>

    <div class="v2-composer-footer">
      <div class="v2-composer-tools">
        <input ref="fileInput" class="sr-only" type="file" accept="image/png,image/jpeg,image/gif" multiple @change="selectFiles" />
        <button type="button" @click="fileInput?.click()">添加图片</button>
        <button type="button" :disabled="!prompt && !attachments.length" @click="$emit('clear')">清空</button>
        <button type="button" :disabled="!canRetry" @click="$emit('retry')">重试</button>
      </div>

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
      </div>

      <button class="primary v2-send-button" type="button" :disabled="!canRun" @click="$emit('run')">
        {{ running || uploading ? '处理中' : '生成图片' }}
      </button>
    </div>

    <p v-if="errorMessage" class="v2-composer-error">{{ errorMessage }}</p>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { ModelSelection } from '../../types'

export interface ComposerAttachment {
  id: string
  name: string
  url: string
}

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
  attachments: ComposerAttachment[]
}>()

const emit = defineEmits<{
  'update:textModelConfigId': [value: number]
  'update:imageModelConfigId': [value: number]
  'update:candidateCount': [value: number]
  'update:disableClarification': [value: boolean]
  'update:prompt': [value: string]
  'attach-files': [files: File[]]
  'remove-attachment': [id: string]
  run: []
  clear: []
  retry: []
}>()

const fileInput = ref<HTMLInputElement | null>(null)
const dragging = ref(false)

function selectFiles(event: Event) {
  const input = event.target as HTMLInputElement
  emitImageFiles(Array.from(input.files || []))
  input.value = ''
}

function handlePaste(event: ClipboardEvent) {
  emitImageFiles(Array.from(event.clipboardData?.files || []))
}

function handleDrop(event: DragEvent) {
  dragging.value = false
  emitImageFiles(Array.from(event.dataTransfer?.files || []))
}

function emitImageFiles(files: File[]) {
  const images = files.filter(file => file.type.startsWith('image/'))
  if (images.length) {
    emit('attach-files', images)
  }
}
</script>
