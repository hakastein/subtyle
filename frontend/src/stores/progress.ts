import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface ProgressStream {
  active: boolean
  message: string
  current: number
  total: number
}

function emptyStream(): ProgressStream {
  return { active: false, message: '', current: 0, total: 0 }
}

export const useProgressStore = defineStore('progress', () => {
  const scan = ref<ProgressStream>(emptyStream())
  const load = ref<ProgressStream>(emptyStream())
  const preview = ref<{ busy: boolean }>({ busy: false })

  function startScan(message: string, total: number = 0): void {
    scan.value = { active: true, message, current: 0, total }
  }

  function updateScan(current: number, total: number, message: string): void {
    scan.value = { active: true, message, current, total }
  }

  function finishScan(): void {
    scan.value = emptyStream()
  }

  function startLoad(message: string, total: number = 0): void {
    load.value = { active: true, message, current: 0, total }
  }

  function updateLoad(current: number, total: number, message: string): void {
    load.value = { active: true, message, current, total }
  }

  function finishLoad(): void {
    load.value = emptyStream()
  }

  function startPreview(): void {
    preview.value = { busy: true }
  }

  function finishPreview(): void {
    preview.value = { busy: false }
  }

  return {
    scan,
    load,
    preview,
    startScan,
    updateScan,
    finishScan,
    startLoad,
    updateLoad,
    finishLoad,
    startPreview,
    finishPreview,
  }
})
