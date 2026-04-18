<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import { NButton, NScrollbar } from 'naive-ui'
import { useDebugStore } from '@/stores/debug'

const debug = useDebugStore()
const scrollRef = ref<InstanceType<typeof NScrollbar> | null>(null)

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
  <div v-if="debug.visible" class="debug-log">
    <div class="debug-log-header">
      <span class="log-title">Debug Log</span>
      <div class="log-actions">
        <NButton size="tiny" @click="debug.clear()">Clear</NButton>
      </div>
    </div>
    <NScrollbar ref="scrollRef" style="max-height: 180px">
      <div class="log-entries">
        <div
          v-for="(entry, i) in debug.logs"
          :key="i"
          class="log-entry"
        >
          <span class="log-time">{{ entry.time }}</span>
          <span class="log-level" :style="{ color: levelColor(entry.level) }">
            [{{ entry.level.toUpperCase() }}]
          </span>
          <span class="log-msg">{{ entry.message }}</span>
        </div>
        <div v-if="debug.logs.length === 0" class="log-empty">No logs yet</div>
      </div>
    </NScrollbar>
  </div>
</template>

<style scoped>
.debug-log {
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 11px;
  border-top: 1px solid #333;
}

.debug-log-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 10px;
  background: #252526;
  border-bottom: 1px solid #333;
}

.log-title {
  font-weight: bold;
  color: #007acc;
}

.log-entries {
  padding: 4px 10px;
}

.log-entry {
  display: flex;
  gap: 6px;
  line-height: 1.6;
}

.log-time { color: #666; }
.log-level { min-width: 50px; }
.log-msg { white-space: pre-wrap; word-break: break-all; }
.log-empty { color: #666; padding: 8px; }
</style>
