<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NSpin, NText } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { usePreviewStore } from '@/stores/preview'
import * as editorService from '@/services/editor'
import { durationToMs } from '@/services/types'
import Timeline from './Timeline.vue'
import CssSubtitleOverlay from './CssSubtitleOverlay.vue'

const { t } = useI18n()
const projectStore = useProjectStore()
const previewStore = usePreviewStore()

const currentEventIndex = ref(0)

// Current style events for timeline
const currentEvents = computed(() => {
  const selected = projectStore.selectedStyles
  if (selected.length === 0) return []
  const { fileId, style } = selected[0]
  return projectStore.getEventsForStyle(fileId, style.name)
})

const currentEvent = computed(() => currentEvents.value[currentEventIndex.value] ?? null)

const currentStyle = computed(() => {
  const selected = projectStore.selectedStyles
  return selected.length > 0 ? selected[0].style : null
})

const activeFile = computed(() => projectStore.activeFile)

// Debounce timer
let previewTimer: ReturnType<typeof setTimeout> | null = null

function schedulePreview() {
  if (previewTimer) clearTimeout(previewTimer)
  previewTimer = setTimeout(() => {
    previewTimer = null
    generatePreview()
  }, 400)
}

async function generatePreview() {
  const file = activeFile.value
  const style = currentStyle.value
  if (!file || !style || !file.videoPath || !previewStore.ffmpegReady) return

  const atMs = currentEvent.value
    ? durationToMs(currentEvent.value.startTime)
    : previewStore.currentTimeMs

  previewStore.setLoading(true)
  try {
    const result = await editorService.generatePreviewFrame(
      file.id,
      file.videoPath,
      file.modifiedStyles,
      atMs,
    )
    previewStore.setFrame(result.base64Png, result.timecode)
  } catch {
    previewStore.setLoading(false)
  }
}

// Watch for style changes to trigger preview
watch(
  () => [projectStore.selectedStyles, currentEventIndex.value],
  () => schedulePreview(),
  { deep: true },
)

// Timeline navigation
function handleSeek(timeMs: number, index: number) {
  currentEventIndex.value = index
  previewStore.currentTimeMs = timeMs
  schedulePreview()
}

function handlePrev() {
  if (currentEventIndex.value > 0) {
    currentEventIndex.value--
    const ev = currentEvents.value[currentEventIndex.value]
    if (ev) previewStore.currentTimeMs = durationToMs(ev.startTime)
    schedulePreview()
  }
}

function handleNext() {
  if (currentEventIndex.value < currentEvents.value.length - 1) {
    currentEventIndex.value++
    const ev = currentEvents.value[currentEventIndex.value]
    if (ev) previewStore.currentTimeMs = durationToMs(ev.startTime)
    schedulePreview()
  }
}

// Reset event index when selection changes
watch(
  () => projectStore.selectedStyleKeys,
  () => {
    currentEventIndex.value = 0
  },
)
</script>

<template>
  <div class="preview-area">
    <div class="preview-header">{{ t('preview.title') }}</div>

    <!-- ffmpeg status messages -->
    <div v-if="previewStore.ffmpegDownloading" class="status-bar">
      <NText depth="3">
        {{ t('ffmpeg.progress', { percent: Math.round(previewStore.ffmpegProgress * 100) }) }}
      </NText>
    </div>

    <div class="preview-frame-container">
      <!-- Loading spinner -->
      <div v-if="previewStore.loading" class="preview-loading">
        <NSpin :size="40" />
        <NText depth="3" style="margin-top: 8px">{{ t('preview.loading') }}</NText>
      </div>

      <!-- FFmpeg frame -->
      <div v-else-if="previewStore.frameBase64" class="frame-wrapper">
        <img
          :src="`data:image/png;base64,${previewStore.frameBase64}`"
          class="preview-image"
          alt="preview frame"
        />
        <div v-if="previewStore.timecode" class="timecode-badge">
          {{ previewStore.timecode }}
        </div>
      </div>

      <!-- CSS overlay fallback -->
      <div v-else-if="currentStyle" class="css-preview-wrapper">
        <div class="css-preview-stage">
          <CssSubtitleOverlay
            :style="currentStyle"
            :text="currentEvent?.text ?? 'Preview Text'"
          />
        </div>
      </div>

      <!-- No selection — gradient placeholder -->
      <div v-else class="css-preview-wrapper">
        <div class="css-preview-stage" />
      </div>
    </div>

    <!-- Timeline -->
    <div class="timeline-container">
      <Timeline
        :events="currentEvents"
        :current-event-index="currentEventIndex"
        @seek="handleSeek"
        @prev="handlePrev"
        @next="handleNext"
      />
    </div>
  </div>
</template>

<style scoped>
.preview-area {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.preview-header {
  padding: 8px 12px;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  flex-shrink: 0;
}

.status-bar {
  padding: 4px 12px;
  background: var(--n-warning-color-suppl, #fffbe6);
  font-size: 12px;
  flex-shrink: 0;
}

.preview-frame-container {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: #1a1a1a;
  position: relative;
}

.preview-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: white;
}

.frame-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.timecode-badge {
  position: absolute;
  bottom: 8px;
  right: 8px;
  background: rgba(0, 0, 0, 0.6);
  color: white;
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 3px;
  font-family: monospace;
}

.css-preview-wrapper {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: stretch;
}

.css-preview-stage {
  position: relative;
  flex: 1;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
}

.preview-empty {
  color: #aaa;
  text-align: center;
  padding: 20px;
}

.timeline-container {
  padding: 4px 12px;
  border-top: 1px solid var(--n-border-color, #e0e0e6);
  flex-shrink: 0;
  background: var(--n-color, #fff);
}
</style>
