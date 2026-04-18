<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NSelect, NSpace, NProgress } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { useUndoStore } from '@/stores/undo'
import { useProgressStore } from '@/stores/progress'
import { setLocale } from '@/i18n'

const { t, locale } = useI18n()
const projectStore = useProjectStore()
const undoStore = useUndoStore()
const progressStore = useProgressStore()

const langOptions = [
  { label: 'EN', value: 'en' },
  { label: 'RU', value: 'ru' },
]

const currentLocale = computed({
  get: () => locale.value,
  set: (val: string) => setLocale(val),
})

const canSave = computed(() => projectStore.dirty)
const canUndo = computed(() => undoStore.canUndo())
const canRedo = computed(() => undoStore.canRedo())

function handleOpen() {
  projectStore.openFolder()
}

function handleSave() {
  projectStore.save()
}

function handleUndo() {
  projectStore.applyUndo()
}

function handleRedo() {
  projectStore.applyRedo()
}
</script>

<template>
  <div class="toolbar">
    <NSpace align="center">
      <NButton @click="handleOpen" type="primary" size="small">
        {{ t('toolbar.open') }}
      </NButton>
      <NButton @click="handleSave" :disabled="!canSave" size="small">
        {{ t('toolbar.save') }}
      </NButton>
      <NButton @click="handleUndo" :disabled="!canUndo" size="small">
        {{ t('toolbar.undo') }}
      </NButton>
      <NButton @click="handleRedo" :disabled="!canRedo" size="small">
        {{ t('toolbar.redo') }}
      </NButton>
    </NSpace>
    <div v-if="progressStore.scan.active" class="scan-progress">
      <NProgress
        type="line"
        :percentage="progressStore.scan.total > 0
          ? Math.round((progressStore.scan.current / progressStore.scan.total) * 100)
          : 0"
        :indicator-placement="'inside'"
        :height="14"
        style="width: 200px"
      />
      <span class="scan-message">{{ progressStore.scan.message }}</span>
    </div>
    <NSelect
      v-model:value="currentLocale"
      :options="langOptions"
      size="small"
      style="width: 80px"
    />
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  padding: 6px 12px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  background: var(--n-color, #fff);
  flex-shrink: 0;
}
.scan-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: 16px;
}
.scan-message {
  font-size: 11px;
  color: var(--n-text-color-3, #888);
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
