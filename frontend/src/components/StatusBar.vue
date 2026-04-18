<script setup lang="ts">
import { computed } from 'vue'
import { useProjectStore } from '@/stores/project'
import { useUndoStore } from '@/stores/undo'
import { usePreviewStore } from '@/stores/preview'
import { useDebugStore } from '@/stores/debug'

const projectStore = useProjectStore()
const undoStore = useUndoStore()
const previewStore = usePreviewStore()
const debug = useDebugStore()

const ffmpegStatus = computed(() => {
  if (previewStore.ffmpegReady) {
    return { text: 'ffmpeg ready', color: '#4caf50' }
  }
  if (previewStore.ffmpegDownloading) {
    const pct = Math.round(previewStore.ffmpegProgress * 100)
    return { text: `downloading ${pct}%`, color: '#ffc107' }
  }
  return { text: 'not ready', color: '#ff6b6b' }
})

const episodesCount = computed(() => {
  const total = projectStore.videoEntries.length
  let checked = 0
  for (const [, v] of projectStore.fileChecks) {
    if (v) checked++
  }
  return `${checked}/${total}`
})

const translationsCount = computed(() => {
  // Placeholder until Task 9 lands: show sourceTypes length
  return projectStore.sourceTypes?.length ?? 0
})

const stylesGroupsCount = computed(() => projectStore.groupedStyles.length)
</script>

<template>
  <div class="status-bar" @click="debug.toggle()">
    <span :style="{ color: ffmpegStatus.color }">● {{ ffmpegStatus.text }}</span>
    <span class="sep">|</span>
    <span>episodes: {{ episodesCount }}</span>
    <span class="sep">|</span>
    <span>translations: {{ translationsCount }}</span>
    <span class="sep">|</span>
    <span>styles: {{ stylesGroupsCount }} groups</span>
    <span class="sep">|</span>
    <span>undo: {{ undoStore.undoStack.length }}</span>
    <span v-if="projectStore.dirty" class="sep">|</span>
    <span v-if="projectStore.dirty" style="color: #ffc107">● unsaved</span>
    <span class="spacer"></span>
    <span class="toggle-hint">⌃ debug ({{ debug.visible ? 'hide' : 'show' }})</span>
  </div>
</template>

<style scoped>
.status-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 10px;
  background: #252526;
  color: #d4d4d4;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 11px;
  border-top: 2px solid #007acc;
  cursor: pointer;
  flex-shrink: 0;
  user-select: none;
}

.status-bar:hover {
  background: #2a2a2b;
}

.sep {
  color: #555;
}

.spacer {
  flex: 1;
}

.toggle-hint {
  color: #888;
}
</style>
