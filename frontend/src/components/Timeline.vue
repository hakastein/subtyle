<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NText } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import type { SubtitleEvent } from '@/services/types'
import { durationToMs } from '@/services/types'

interface Props {
  events: SubtitleEvent[]
  currentEventIndex: number
  loading?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'seek', timeMs: number, eventIndex: number): void
  (e: 'prev'): void
  (e: 'next'): void
}>()

const { t } = useI18n()

const totalDurationMs = computed(() => {
  if (props.events.length === 0) return 0
  return Math.max(...props.events.map(e => durationToMs(e.endTime)))
})

function markerLeft(event: SubtitleEvent): string {
  const total = totalDurationMs.value
  if (total === 0) return '0%'
  const pct = (durationToMs(event.startTime) / total) * 100
  return `${Math.min(100, Math.max(0, pct)).toFixed(2)}%`
}

function markerWidth(event: SubtitleEvent): string {
  const total = totalDurationMs.value
  if (total === 0) return '2px'
  const startMs = durationToMs(event.startTime)
  const endMs = durationToMs(event.endTime)
  const pct = ((endMs - startMs) / total) * 100
  return `max(2px, ${Math.min(100, Math.max(0, pct)).toFixed(2)}%)`
}

function handleMarkerClick(index: number) {
  const event = props.events[index]
  if (!event) return
  emit('seek', durationToMs(event.startTime), index)
}
</script>

<template>
  <div class="timeline">
    <div class="timeline-controls">
      <NButton size="tiny" @click="emit('prev')" :disabled="currentEventIndex <= 0">◀</NButton>
      <NText depth="3" style="font-size: 11px; margin: 0 6px">
        {{ events.length > 0 ? `${currentEventIndex + 1} / ${events.length}` : t('timeline.noEvents') }}
      </NText>
      <NButton size="tiny" @click="emit('next')" :disabled="currentEventIndex >= events.length - 1">▶</NButton>
    </div>

    <div class="timeline-track" :class="{ loading }" v-if="events.length > 0 || loading">
      <div
        v-for="(event, index) in events"
        :key="index"
        class="timeline-marker"
        :class="{ active: index === currentEventIndex }"
        :style="{ left: markerLeft(event), width: markerWidth(event) }"
        :title="event.text"
        @click="handleMarkerClick(index)"
      />
      <div v-if="loading" class="timeline-shimmer-label">Scanning events...</div>
    </div>
    <div v-else class="timeline-empty">
      <NText depth="3" style="font-size: 11px">{{ t('timeline.noEvents') }}</NText>
    </div>
  </div>
</template>

<style scoped>
.timeline {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px 0;
}

.timeline-controls {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.timeline-track {
  position: relative;
  height: 20px;
  background: var(--n-color-target, #f5f5f5);
  border-radius: 4px;
  overflow: hidden;
  cursor: pointer;
}

.timeline-marker {
  position: absolute;
  top: 2px;
  bottom: 2px;
  background: var(--n-primary-color, #18a058);
  opacity: 0.6;
  border-radius: 2px;
  transition: opacity 0.15s;
}

.timeline-marker:hover {
  opacity: 0.9;
}

.timeline-marker.active {
  opacity: 1;
  background: var(--n-primary-color-hover, #0c7a43);
  outline: 1px solid currentColor;
}

.timeline-empty {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 20px;
}

/* Shimmer animation for loading state. Sliding gradient looks like typical
 * indeterminate progress bars. */
.timeline-track.loading {
  position: relative;
  background: linear-gradient(
    90deg,
    rgba(24, 160, 88, 0.08) 0%,
    rgba(24, 160, 88, 0.22) 50%,
    rgba(24, 160, 88, 0.08) 100%
  );
  background-size: 200% 100%;
  animation: timeline-shimmer 1.4s linear infinite;
}

.timeline-shimmer-label {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  color: rgba(0, 0, 0, 0.5);
  pointer-events: none;
  user-select: none;
}

@keyframes timeline-shimmer {
  0%   { background-position: 100% 0; }
  100% { background-position: -100% 0; }
}
</style>
