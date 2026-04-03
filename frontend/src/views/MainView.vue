<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { usePreviewStore } from '@/stores/preview'
import { useDebugStore } from '@/stores/debug'
import {
  onFFmpegReady,
  onFFmpegDownloading,
  onFFmpegProgress,
  onFFmpegError,
} from '@/services/ffmpeg'
import * as projectService from '@/services/project'
import Toolbar from '@/components/Toolbar.vue'
import FileTree from '@/components/FileTree.vue'
import StyleEditor from '@/components/StyleEditor.vue'
import PreviewArea from '@/components/PreviewArea.vue'
import DebugPanel from '@/components/DebugPanel.vue'

const { t } = useI18n()
const message = useMessage()
const projectStore = useProjectStore()
const previewStore = usePreviewStore()
const debug = useDebugStore()

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

async function pollFfmpegReady() {
  // Poll backend directly in case event was emitted before we mounted
  try {
    const ready = await projectService.isFfmpegReady()
    debug.info(`ffmpeg poll: ready=${ready}`)
    if (ready) {
      previewStore.ffmpegReady = true
      previewStore.ffmpegDownloading = false
      return
    }
  } catch (err) {
    debug.error(`ffmpeg poll failed: ${err}`)
  }

  // Not ready yet — keep polling every 500ms
  setTimeout(pollFfmpegReady, 500)
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)

  debug.info('MainView mounted, registering ffmpeg events')

  // Register ffmpeg event listeners
  onFFmpegReady(() => {
    debug.info('ffmpeg:ready event received')
    previewStore.ffmpegReady = true
    previewStore.ffmpegDownloading = false
    message.success(t('ffmpeg.ready'))
  })

  onFFmpegDownloading(() => {
    debug.info('ffmpeg:downloading event received')
    previewStore.ffmpegDownloading = true
    previewStore.ffmpegProgress = 0
    message.info(t('ffmpeg.downloading'))
  })

  onFFmpegProgress((received: number, total: number) => {
    if (total > 0) {
      previewStore.ffmpegProgress = received / total
    }
  })

  onFFmpegError((error: string) => {
    debug.error(`ffmpeg:error event: ${error}`)
    previewStore.ffmpegDownloading = false
    message.error(t('ffmpeg.error', { message: error }))
  })

  // Poll ffmpeg status to catch race condition where event fired before mount
  pollFfmpegReady()
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
        <FileTree />
      </aside>
      <main class="panel-center">
        <PreviewArea />
      </main>
      <aside class="panel-right">
        <StyleEditor />
      </aside>
    </div>
    <DebugPanel />
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
  width: 260px;
  min-width: 180px;
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
