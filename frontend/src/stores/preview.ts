import { defineStore } from 'pinia'
import { ref } from 'vue'

export const usePreviewStore = defineStore('preview', () => {
  const frameBase64 = ref<string | null>(null)
  const timecode = ref('')
  const loading = ref(false)
  const ffmpegReady = ref(false)
  const ffmpegDownloading = ref(false)
  const ffmpegProgress = ref(0)
  const currentTimeMs = ref(0)
  const videoDurationMs = ref(0)

  function setFrame(base64: string, tc: string): void {
    frameBase64.value = base64
    timecode.value = tc
    loading.value = false
  }

  function setLoading(isLoading: boolean): void { loading.value = isLoading }
  function clearFrame(): void { frameBase64.value = null; timecode.value = '' }

  return { frameBase64, timecode, loading, ffmpegReady, ffmpegDownloading, ffmpegProgress, currentTimeMs, videoDurationMs, setFrame, setLoading, clearFrame }
})
