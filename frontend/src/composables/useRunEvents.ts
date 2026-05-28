import { ref } from 'vue'

export function useRunEvents(refresh: () => Promise<void>) {
  const runPollTimer = ref<ReturnType<typeof window.setInterval> | null>(null)

  function startRunPolling(runID: number, activeRunId: () => number, onError: (message: string) => void) {
    clearRunPolling()
    runPollTimer.value = window.setInterval(async () => {
      if (activeRunId() !== runID) {
        clearRunPolling()
        return
      }
      try {
        await refresh()
      } catch (error) {
        onError(error instanceof Error ? error.message : '刷新运行状态失败')
        clearRunPolling()
      }
    }, 2000)
  }

  function clearRunPolling() {
    if (!runPollTimer.value) return
    window.clearInterval(runPollTimer.value)
    runPollTimer.value = null
  }

  return {
    runPollTimer,
    startRunPolling,
    clearRunPolling
  }
}
