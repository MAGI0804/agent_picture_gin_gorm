<template>
  <section v-if="artifact" class="v2-edit-panel">
    <label>
      编辑 Prompt
      <textarea
        :value="editPrompt"
        placeholder="描述要基于当前版本继续修改的内容。"
        @input="$emit('update:editPrompt', ($event.target as HTMLTextAreaElement).value)"
        @keydown.enter.ctrl.prevent="$emit('edit')"
      />
    </label>
    <button type="button" :disabled="!canEdit" @click="$emit('edit')">
      {{ editing ? '编辑中...' : '继续编辑' }}
    </button>
  </section>

  <section v-if="artifact?.kind === 'image'" class="v2-edit-panel">
    <label>
      标题
      <input :value="renderTitle" type="text" placeholder="中文标题由渲染层排版" @input="$emit('update:renderTitle', ($event.target as HTMLInputElement).value)" />
    </label>
    <label>
      副标题
      <input :value="renderSubtitle" type="text" placeholder="可选副标题" @input="$emit('update:renderSubtitle', ($event.target as HTMLInputElement).value)" />
    </label>
    <label>
      品牌
      <input :value="renderBrand" type="text" placeholder="可选品牌文案" @input="$emit('update:renderBrand', ($event.target as HTMLInputElement).value)" />
    </label>
    <button type="button" :disabled="!canRender" @click="$emit('render')">
      {{ rendering ? '渲染中...' : '生成文字分层' }}
    </button>
  </section>
</template>

<script setup lang="ts">
import type { Artifact } from '../../types'

defineProps<{
  artifact: Artifact | null
  editPrompt: string
  editing: boolean
  canEdit: boolean
  renderTitle: string
  renderSubtitle: string
  renderBrand: string
  rendering: boolean
  canRender: boolean
}>()

defineEmits<{
  'update:editPrompt': [value: string]
  'update:renderTitle': [value: string]
  'update:renderSubtitle': [value: string]
  'update:renderBrand': [value: string]
  edit: []
  render: []
}>()
</script>
