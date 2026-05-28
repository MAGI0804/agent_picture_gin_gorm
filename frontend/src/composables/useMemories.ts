import { computed, ref } from 'vue'
import type { ContextMemory } from '../types'

export function useMemories() {
  const memories = ref<ContextMemory[]>([])
  const memoryNamespace = ref('')
  const memoryStatusFilter = ref('')
  const memoryLoading = ref(false)
  const promotingMemoryId = ref(0)

  const displayedMemories = computed(() => {
    if (memoryStatusFilter.value === 'proposal') {
      return memories.value.filter(memory => isMemoryProposal(memory))
    }
    if (memoryStatusFilter.value === 'stable') {
      return memories.value.filter(memory => !isMemoryProposal(memory))
    }
    return memories.value
  })

  return {
    memories,
    memoryNamespace,
    memoryStatusFilter,
    memoryLoading,
    promotingMemoryId,
    displayedMemories,
    isMemoryProposal
  }
}

export function isMemoryProposal(memory: ContextMemory) {
  return memory.kind === 'memory_proposal'
}
