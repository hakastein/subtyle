<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { usePreviewStore } from '@/stores/preview'
import { useDebugStore } from '@/stores/debug'
import { useProgressStore } from '@/stores/progress'
import {
  onFFmpegReady,
  onFFmpegDownloading,
  onFFmpegProgress,
  onFFmpegError,
} from '@/services/ffmpeg'
import Toolbar from '@/components/Toolbar.vue'
import FilePanel from '@/components/FilePanel.vue'
import StyleEditor from '@/components/StyleEditor.vue'
import PreviewArea from '@/components/PreviewArea.vue'
import DebugLog from '@/components/DebugLog.vue'
import StatusBar from '@/components/StatusBar.vue'

const { t } = useI18n()
const message = useMessage()
const projectStore = useProjectStore()
const previewStore = usePreviewStore()
const debug = useDebugStore()
const progressStore = useProgressStore()

// Keyboard shortcuts
function handleKeydown(e: KeyboardEvent) {
  const isMac = navigator.platform.toLowerCase().includes('mac')
  const ctrl = isMac ? e.metaKey : e.ctrlKey

  // F12 toggles debug panel (no ctrl needed)
  if (e.key === 'F12') {
    e.preventDefault()
    debug.toggle()
    return
  }

  if (!ctrl) return

  if (e.key === 'z' && !e.shiftKey) {
    e.preventDefault()
    projectStore.applyUndo()
  } else if (e.key === 'y' || (e.key === 'z' && e.shiftKey)) {
    e.preventDefault()
    projectStore.applyRedo()
  } else if (e.key === 's') {
    e.preventDefault()
    if (projectStore.dirty) {
      projectStore.save().then(() => {
        message.success(t('project.saved'))
      }).catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : String(err)
        message.error(t('project.saveError', { message: msg }))
      })
    }
  }
}

async function syncFfmpegState() {
  // One-shot query to recover state in case events fired before we mounted.
  // After this we rely on events only — no polling spam.
  try {
    const state = await window.go.main.App.GetFfmpegState()
    debug.info(`ffmpeg state: ${state.status} (${Math.round((state.progress || 0) * 100)}%)`)
    if (state.status === 'ready') {
      previewStore.ffmpegReady = true
      previewStore.ffmpegDownloading = false
      previewStore.ffmpegProgress = 1
      void logFfmpegDiag()
    } else if (state.status === 'downloading') {
      previewStore.ffmpegDownloading = true
      previewStore.ffmpegReady = false
      previewStore.ffmpegProgress = state.progress || 0
    } else if (state.status === 'error') {
      debug.error(`ffmpeg error: ${state.error}`)
    }
  } catch (err) {
    debug.error(`ffmpeg state query failed: ${err}`)
  }
}

async function logFfmpegDiag() {
  try {
    const diag = await window.go.main.App.GetFfmpegDiag()
    debug.info(`ffmpeg path: ${diag.path}`)
    debug.info(`ffmpeg version: ${diag.version}`)
    debug.info(`ffmpeg subtitles filter: ${diag.hasSubtitlesFilter}`)
    debug.info(`ffmpeg libass: ${diag.hasLibass}`)
    if (!diag.hasSubtitlesFilter) {
      debug.error('ffmpeg does NOT support subtitles filter — subtitle overlay will not work!')
      message.warning('ffmpeg does not support subtitle rendering (missing libass)', { duration: 10000 })
    }
  } catch (err) {
    debug.error(`ffmpeg diag failed: ${err}`)
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)

  debug.info('MainView mounted, registering ffmpeg events')

  // Register ffmpeg event listeners
  onFFmpegReady(() => {
    debug.info('ffmpeg:ready event received')
    previewStore.ffmpegReady = true
    previewStore.ffmpegDownloading = false
    previewStore.ffmpegProgress = 1
    message.success(t('ffmpeg.ready'))
    void logFfmpegDiag()
  })

  onFFmpegDownloading(() => {
    debug.info('ffmpeg:downloading event received')
    previewStore.ffmpegDownloading = true
    previewStore.ffmpegReady = false
    previewStore.ffmpegProgress = 0
    message.info(t('ffmpeg.downloading'))
  })

  onFFmpegProgress((received: number, total: number) => {
    if (total > 0) {
      previewStore.ffmpegProgress = received / total
      previewStore.ffmpegDownloading = true
    }
  })

  onFFmpegError((error: string) => {
    debug.error(`ffmpeg:error event: ${error}`)
    previewStore.ffmpegDownloading = false
    message.error(t('ffmpeg.error', { message: error }))
  })

  // Backend debug logs
  window.runtime.EventsOn('debug:log', (msg: unknown) => {
    debug.info(`[backend] ${msg}`)
  })

  window.runtime.EventsOn('progress:scan', (data: unknown) => {
    const d = data as { stage: string; current?: number; total?: number; message?: string }
    if (d.stage === 'done') {
      progressStore.finishScan()
    } else {
      progressStore.updateScan(d.current ?? 0, d.total ?? 0, d.message ?? '')
      progressStore.scan.active = true
    }
  })

  // One-shot state sync — in case events fired before this mount.
  syncFfmpegState()
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="main-view">
    <Toolbar />
    <div class="main-body">
      <aside class="panel-left">
        <FilePanel />
      </aside>
      <main class="panel-center">
        <PreviewArea />
      </main>
      <aside class="panel-right">
        <StyleEditor />
      </aside>
    </div>
    <DebugLog />
    <StatusBar />
  </div>
</template>

<style scoped>
.main-view {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.main-body {
  display: flex;
  flex: 1;
  overflow: hidden;
  min-height: 0;
}

.panel-left {
  width: 520px;
  min-width: 360px;
  border-right: 1px solid var(--n-border-color, #e0e0e6);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.panel-center {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}

.panel-right {
  width: 280px;
  min-width: 220px;
  border-left: 1px solid var(--n-border-color, #e0e0e6);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}
</style>
