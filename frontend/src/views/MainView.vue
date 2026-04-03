<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { usePreviewStore } from '@/stores/preview'
import {
  onFFmpegReady,
  onFFmpegDownloading,
  onFFmpegProgress,
  onFFmpegError,
} from '@/services/ffmpeg'
import Toolbar from '@/components/Toolbar.vue'
import FileTree from '@/components/FileTree.vue'
import StyleEditor from '@/components/StyleEditor.vue'
import PreviewArea from '@/components/PreviewArea.vue'

const { t } = useI18n()
const message = useMessage()
const projectStore = useProjectStore()
const previewStore = usePreviewStore()

// Keyboard shortcuts
function handleKeydown(e: KeyboardEvent) {
  const isMac = navigator.platform.toLowerCase().includes('mac')
  const ctrl = isMac ? e.metaKey : e.ctrlKey

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

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)

  // Register ffmpeg event listeners
  onFFmpegReady(() => {
    previewStore.ffmpegReady = true
    previewStore.ffmpegDownloading = false
    message.success(t('ffmpeg.ready'))
  })

  onFFmpegDownloading(() => {
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
    previewStore.ffmpegDownloading = false
    message.error(t('ffmpeg.error', { message: error }))
  })
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
