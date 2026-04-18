<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NSpin, NText } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { usePreviewStore } from '@/stores/preview'
import { useDebugStore } from '@/stores/debug'
import { useProgressStore } from '@/stores/progress'
import * as editorService from '@/services/editor'
import { durationToMs } from '@/services/types'
import Timeline from './Timeline.vue'
import CssSubtitleOverlay from './CssSubtitleOverlay.vue'

const { t } = useI18n()
const debug = useDebugStore()
const projectStore = useProjectStore()
const previewStore = usePreviewStore()
const progressStore = useProgressStore()

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

// Lazily fetch video duration for the active file (cache to avoid re-query)
const durationCache = new Map<string, number>()
watch(
  () => activeFile.value?.videoPath,
  async (videoPath) => {
    if (!videoPath) return
    if (durationCache.has(videoPath)) {
      previewStore.videoDurationMs = durationCache.get(videoPath)!
      return
    }
    try {
      const ms = await editorService.getVideoDuration(videoPath)
      durationCache.set(videoPath, ms)
      previewStore.videoDurationMs = ms
    } catch (err) {
      debug.error(`video duration failed: ${err}`)
    }
  },
  { immediate: true },
)

// Background full-track prefetch when active file has styles-only loaded.
// Shows a shimmer on the timeline while events are being extracted.
const prefetchInFlight = new Set<string>()
watch(
  () => activeFile.value,
  async (file) => {
    if (!file) {
      previewStore.eventsLoading = false
      return
    }
    if (file.events.length > 0) {
      previewStore.eventsLoading = false
      return
    }
    if (file.source !== 'embedded' || !file.videoPath) {
      previewStore.eventsLoading = false
      return
    }
    if (prefetchInFlight.has(file.id)) return
    prefetchInFlight.add(file.id)
    previewStore.eventsLoading = true
    try {
      debug.info(`prefetch full events for ${file.id}`)
      const events = await window.go.main.App.EnsureFullTrack(file.id, file.videoPath)
      projectStore.setEventsFor(file.id, events ?? [])
      debug.info(`prefetch done for ${file.id}: ${events?.length ?? 0} events`)
    } catch (err) {
      debug.error(`prefetch failed: ${err}`)
    } finally {
      prefetchInFlight.delete(file.id)
      previewStore.eventsLoading = false
    }
  },
  { immediate: true },
)

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

  if (!file) { debug.info('preview: skip — no active file'); return }
  if (!style) { debug.info('preview: skip — no style selected'); return }
  if (!file.videoPath) { debug.warn('preview: skip — no videoPath on file ' + file.id); return }
  if (!previewStore.ffmpegReady) { debug.warn('preview: skip — ffmpeg not ready'); return }

  const atMs = currentEvent.value
    ? durationToMs(currentEvent.value.startTime)
    : previewStore.currentTimeMs

  debug.info(`preview: requesting frame file=${file.id} video=${file.videoPath} at=${atMs}ms styles=${file.modifiedStyles.length}`)
  previewStore.setLoading(true)
  progressStore.startPreview()
  try {
    const result = await editorService.generatePreviewFrame(
      file.id,
      file.videoPath,
      file.modifiedStyles,
      atMs,
    )
    debug.info(`preview: frame received, base64 length=${result.base64Png.length} tc=${result.timecode}`)
    previewStore.setFrame(result.base64Png, result.timecode)
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err)
    debug.error(`preview: frame generation failed — ${msg}`)
    previewStore.setLoading(false)
  } finally {
    progressStore.finishPreview()
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
      <div v-if="progressStore.preview.busy" class="preview-progress-bar">
        <div class="preview-progress-stripe"></div>
      </div>
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
        :loading="previewStore.eventsLoading"
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

.preview-progress-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: #0a2540;
  overflow: hidden;
  z-index: 5;
}

.preview-progress-stripe {
  width: 35%;
  height: 100%;
  background: linear-gradient(90deg, transparent, #2080f0, transparent);
  animation: preview-stripe 1.2s linear infinite;
}

@keyframes preview-stripe {
  from { transform: translateX(-100%); }
  to { transform: translateX(285%); }
}
</style>
