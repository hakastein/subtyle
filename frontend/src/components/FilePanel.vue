<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCheckbox, NSelect, NEmpty, NSpin, NText, NScrollbar } from 'naive-ui'
import type { SelectOption } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { useDebugStore } from '@/stores/debug'

const { t } = useI18n()
const projectStore = useProjectStore()
const debug = useDebugStore()

// Left sub-column: videos with checkboxes
const videoEntries = computed(() => projectStore.videoEntries)

function isChecked(videoPath: string): boolean {
  return projectStore.fileChecks.get(videoPath) ?? true
}

function toggleCheck(videoPath: string, value: boolean): void {
  const next = new Map(projectStore.fileChecks)
  next.set(videoPath, value)
  projectStore.fileChecks = next
}

function checkAll(value: boolean): void {
  const next = new Map<string, boolean>()
  for (const entry of videoEntries.value) {
    next.set(entry.videoPath, value)
  }
  projectStore.fileChecks = next
}

const allChecked = computed(() =>
  videoEntries.value.every(e => isChecked(e.videoPath)),
)
const noneChecked = computed(() =>
  videoEntries.value.every(e => !isChecked(e.videoPath)),
)

// Right sub-column: source selector + grouped styles
const sourceOptions = computed<SelectOption[]>(() =>
  projectStore.sourceTypes.map(s => ({
    label: s.label,
    value: s.key,
  })),
)

const selectedSource = computed({
  get: () => projectStore.selectedSourceKey,
  set: (key: string | null) => {
    projectStore.selectedSourceKey = key
    debug.info(`FilePanel: selected source ${key}`)
  },
})

const isStyleSelected = (styleName: string) =>
  projectStore.groupedStyles
    .find(g => g.styleName === styleName)
    ?.instances.every(i =>
      projectStore.selectedStyleKeys.includes(`${i.fileId}::${i.styleName}`),
    ) ?? false

function handleStyleClick(styleName: string, event: MouseEvent) {
  const additive = event.ctrlKey || event.metaKey
  projectStore.selectGroupedStyle(styleName, additive)
  debug.info(`FilePanel: style ${styleName} ${additive ? '(additive)' : '(replace)'}`)
}

// Compact style info for the list row
function styleInfo(style: { fontName: string; fontSize: number; bold: boolean; italic: boolean }) {
  const parts = [style.fontName, String(style.fontSize)]
  if (style.bold) parts.push('B')
  if (style.italic) parts.push('I')
  return parts.join(' · ')
}
</script>

<template>
  <div class="file-panel">
    <!-- Left sub-column: file checkboxes -->
    <div class="files-col">
      <div class="col-header">
        <NCheckbox
          :checked="allChecked"
          :indeterminate="!allChecked && !noneChecked"
          @update:checked="checkAll"
        >
          <span class="header-label">{{ t('fileTree.title') }}</span>
        </NCheckbox>
      </div>

      <NEmpty
        v-if="videoEntries.length === 0"
        :description="t('fileTree.noFiles')"
        size="small"
        style="padding: 20px 12px"
      />

      <NScrollbar v-else style="max-height: 100%; flex: 1">
        <div class="file-list">
          <label
            v-for="entry in videoEntries"
            :key="entry.videoPath"
            class="file-row"
          >
            <NCheckbox
              :checked="isChecked(entry.videoPath)"
              @update:checked="(v: boolean) => toggleCheck(entry.videoPath, v)"
            />
            <span class="file-name" :title="entry.videoPath">
              <span v-if="entry.episode !== null" class="ep-badge">
                {{ String(entry.episode).padStart(2, '0') }}
              </span>
              {{ entry.videoName }}
            </span>
          </label>
        </div>
      </NScrollbar>
    </div>

    <!-- Right sub-column: source selector + styles -->
    <div class="styles-col">
      <div class="col-header">
        <NSelect
          v-model:value="selectedSource"
          :options="sourceOptions"
          placeholder="Select subtitle source"
          size="small"
          clearable
          :disabled="sourceOptions.length === 0"
        />
      </div>

      <div v-if="projectStore.sourceLoadingState === 'loading'" class="loading-row">
        <NSpin size="small" />
        <NText depth="3" style="font-size: 12px; margin-left: 8px">
          Loading styles...
        </NText>
      </div>

      <NEmpty
        v-else-if="!selectedSource"
        description="Choose a subtitle source above"
        size="small"
        style="padding: 20px 12px"
      />

      <NEmpty
        v-else-if="projectStore.groupedStyles.length === 0"
        description="No styles loaded"
        size="small"
        style="padding: 20px 12px"
      />

      <NScrollbar v-else style="max-height: 100%; flex: 1">
        <div class="style-list">
          <div
            v-for="group in projectStore.groupedStyles"
            :key="group.styleName"
            class="style-row"
            :class="{ active: isStyleSelected(group.styleName) }"
            @click="handleStyleClick(group.styleName, $event)"
          >
            <div class="style-main">
              <span class="style-name">{{ group.styleName }}</span>
              <span v-if="group.episodesLabel" class="episodes-label">
                ep {{ group.episodesLabel }}
              </span>
            </div>
            <div class="style-info">
              {{ styleInfo(group.representative) }}
              <span class="instance-count">({{ group.instances.length }})</span>
            </div>
          </div>
        </div>
      </NScrollbar>
    </div>
  </div>
</template>

<style scoped>
.file-panel {
  display: flex;
  height: 100%;
  overflow: hidden;
}

.files-col,
.styles-col {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.files-col {
  width: 45%;
  min-width: 200px;
  border-right: 1px solid var(--n-border-color, #e0e0e6);
}

.styles-col {
  flex: 1;
  min-width: 0;
}

.col-header {
  padding: 6px 10px;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-label {
  font-weight: 600;
}

.file-list {
  padding: 4px 0;
}

.file-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 10px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1.3;
}

.file-row:hover {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.04));
}

.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.ep-badge {
  display: inline-block;
  background: var(--n-color-target, #2080f0);
  color: white;
  font-size: 10px;
  font-weight: 600;
  padding: 1px 4px;
  border-radius: 3px;
  margin-right: 4px;
}

.loading-row {
  display: flex;
  align-items: center;
  padding: 12px;
}

.style-list {
  padding: 4px 0;
}

.style-row {
  display: flex;
  flex-direction: column;
  padding: 6px 10px;
  cursor: pointer;
  border-left: 2px solid transparent;
}

.style-row:hover {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.04));
}

.style-row.active {
  background: var(--n-color-target-hover, rgba(32, 128, 240, 0.12));
  border-left-color: var(--n-color-target, #2080f0);
}

.style-main {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
}

.style-name {
  font-weight: 600;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.episodes-label {
  font-size: 11px;
  color: var(--n-text-color-3, #999);
  white-space: nowrap;
}

.style-info {
  font-size: 11px;
  color: var(--n-text-color-3, #999);
  margin-top: 2px;
}

.instance-count {
  margin-left: 6px;
  opacity: 0.7;
}
</style>
