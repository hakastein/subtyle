<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import { NButton, NScrollbar } from 'naive-ui'
import { useDebugStore } from '@/stores/debug'
import { usePreviewStore } from '@/stores/preview'
import { useProjectStore } from '@/stores/project'
import { useUndoStore } from '@/stores/undo'

const debug = useDebugStore()
const previewStore = usePreviewStore()
const projectStore = useProjectStore()
const undoStore = useUndoStore()

const scrollRef = ref<InstanceType<typeof NScrollbar> | null>(null)

// Auto-scroll to bottom on new logs
watch(() => debug.logs.length, () => {
  nextTick(() => {
    scrollRef.value?.scrollTo({ top: 999999 })
  })
})

function levelColor(level: string): string {
  switch (level) {
    case 'error': return '#ff6b6b'
    case 'warn': return '#ffc107'
    default: return '#90caf9'
  }
}
</script>

<template>
  <div v-if="debug.visible" class="debug-panel">
    <div class="debug-header">
      <span class="debug-title">Debug Log</span>
      <div class="debug-status">
        <span :style="{ color: previewStore.ffmpegReady ? '#4caf50' : '#ff6b6b' }">
          ffmpeg: {{ previewStore.ffmpegReady ? 'READY' : 'NOT READY' }}
        </span>
        <span>| files: {{ projectStore.loadedFiles.size }}</span>
        <span>| selected: {{ projectStore.selectedStyleKeys.length }}</span>
        <span>| undo: {{ undoStore.undoStack.length }}</span>
        <span>| dirty: {{ projectStore.dirty }}</span>
        <span>| frame: {{ previewStore.frameBase64 ? 'YES' : 'NO' }}</span>
        <span>| loading: {{ previewStore.loading }}</span>
      </div>
      <div class="debug-actions">
        <NButton size="tiny" @click="debug.clear()">Clear</NButton>
        <NButton size="tiny" @click="debug.toggle()">Close</NButton>
      </div>
    </div>
    <NScrollbar ref="scrollRef" style="max-height: 200px">
      <div class="debug-logs">
        <div
          v-for="(entry, i) in debug.logs"
          :key="i"
          class="debug-log-entry"
        >
          <span class="log-time">{{ entry.time }}</span>
          <span class="log-level" :style="{ color: levelColor(entry.level) }">
            [{{ entry.level.toUpperCase() }}]
          </span>
          <span class="log-msg">{{ entry.message }}</span>
        </div>
        <div v-if="debug.logs.length === 0" class="debug-empty">No logs yet</div>
      </div>
    </NScrollbar>
  </div>
</template>

<style scoped>
.debug-panel {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 11px;
  z-index: 9999;
  border-top: 2px solid #007acc;
}

.debug-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 8px;
  background: #252526;
  border-bottom: 1px solid #333;
}

.debug-title {
  font-weight: bold;
  color: #007acc;
}

.debug-status {
  display: flex;
  gap: 8px;
  flex: 1;
  color: #888;
}

.debug-actions {
  display: flex;
  gap: 4px;
}

.debug-logs {
  padding: 4px 8px;
}

.debug-log-entry {
  display: flex;
  gap: 6px;
  line-height: 1.6;
  white-space: nowrap;
}

.log-time {
  color: #666;
}

.log-level {
  min-width: 50px;
}

.log-msg {
  white-space: pre-wrap;
  word-break: break-all;
}

.debug-empty {
  color: #666;
  padding: 8px;
}
</style>
