<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NCheckbox, NEmpty, NScrollbar, NProgress } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { useDebugStore } from '@/stores/debug'
import { useProgressStore } from '@/stores/progress'

const { t } = useI18n()
const projectStore = useProjectStore()
const debug = useDebugStore()
const progressStore = useProgressStore()

// Translations: visible list (not dropdown). Ctrl/Cmd+click for multi-select.
function handleTranslationClick(key: string, event: MouseEvent) {
  const additive = event.ctrlKey || event.metaKey
  projectStore.selectTranslation(key, additive)
  debug.info(`FilePanel: translation ${key} ${additive ? '+add' : 'replace'}`)
}

function isTranslationSelected(key: string): boolean {
  return projectStore.selectedTranslationKeys.includes(key)
}

// Episodes: checkbox list.
function episodeIsChecked(videoPath: string): boolean {
  return projectStore.episodeChecks.get(videoPath) ?? false
}

function toggleEpisode(videoPath: string, value: boolean) {
  projectStore.toggleEpisode(videoPath, value)
}

const checkedCount = computed(() => {
  let n = 0
  for (const [, v] of projectStore.episodeChecks) {
    if (v) n++
  }
  return n
})

// Styles: grouped list with header progress bar.
function isStyleSelected(styleName: string): boolean {
  const group = projectStore.groupedStyles.find(g => g.styleName === styleName)
  if (!group) return false
  return group.instances.every(i =>
    projectStore.selectedStyleKeys.includes(`${i.fileId}::${i.styleName}`),
  )
}

function handleStyleClick(styleName: string, event: MouseEvent) {
  const additive = event.ctrlKey || event.metaKey
  projectStore.selectGroupedStyle(styleName, additive)
}

function styleInfo(style: { fontName: string; fontSize: number; bold: boolean; italic: boolean }) {
  const parts = [style.fontName, String(style.fontSize)]
  if (style.bold) parts.push('B')
  if (style.italic) parts.push('I')
  return parts.join(' · ')
}

const loadPercentage = computed(() => {
  const p = progressStore.load
  return p.total > 0 ? Math.round((p.current / p.total) * 100) : 0
})
</script>

<template>
  <div class="file-panel">
    <!-- Left sub-column: translations (top) + episodes (bottom) -->
    <div class="left-col">
      <!-- Translations -->
      <div class="translations-section">
        <div class="section-header">Translations</div>
        <NEmpty
          v-if="projectStore.translations.length === 0"
          description="No translations"
          size="small"
          style="padding: 12px"
        />
        <NScrollbar v-else style="flex: 1">
          <div
            v-for="trans in projectStore.translations"
            :key="trans.key"
            class="trans-row"
            :class="{ active: isTranslationSelected(trans.key) }"
            @click="handleTranslationClick(trans.key, $event)"
          >
            <span class="trans-label" :title="trans.label">{{ trans.label }}</span>
            <span class="trans-coverage">{{ trans.coverageCount }}/{{ trans.totalEpisodes }}</span>
          </div>
        </NScrollbar>
      </div>

      <!-- Episodes -->
      <div class="episodes-section">
        <div class="section-header">
          <span>Episodes</span>
          <span class="header-muted">{{ checkedCount }}/{{ projectStore.videoEntries.length }}</span>
        </div>
        <NEmpty
          v-if="projectStore.videoEntries.length === 0"
          :description="t('fileTree.noFiles')"
          size="small"
          style="padding: 12px"
        />
        <NScrollbar v-else style="flex: 1">
          <label
            v-for="entry in projectStore.videoEntries"
            :key="entry.videoPath"
            class="ep-row"
            :class="{ disabled: !episodeIsChecked(entry.videoPath) }"
          >
            <NCheckbox
              :checked="episodeIsChecked(entry.videoPath)"
              @update:checked="(v: boolean) => toggleEpisode(entry.videoPath, v)"
            />
            <span v-if="entry.episode !== null" class="ep-badge">
              {{ String(entry.episode).padStart(2, '0') }}
            </span>
            <span class="ep-name" :title="entry.videoPath">{{ entry.videoName }}</span>
          </label>
        </NScrollbar>
      </div>
    </div>

    <!-- Right sub-column: styles -->
    <div class="styles-col">
      <div class="section-header styles-header">
        <span>Styles</span>
        <NProgress
          v-if="progressStore.load.active"
          type="line"
          :percentage="loadPercentage"
          :show-indicator="false"
          :height="4"
          style="flex: 1"
        />
        <span v-else class="header-muted">
          {{ projectStore.groupedStyles.length }}
        </span>
      </div>

      <div v-if="progressStore.load.active" class="load-message">
        {{ progressStore.load.message }}
      </div>

      <NEmpty
        v-else-if="projectStore.selectedTranslationKeys.length === 0"
        description="Pick a translation to see styles"
        size="small"
        style="padding: 20px 12px"
      />

      <NEmpty
        v-else-if="projectStore.groupedStyles.length === 0"
        description="No styles loaded"
        size="small"
        style="padding: 20px 12px"
      />

      <NScrollbar v-else style="flex: 1">
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

.left-col {
  width: 260px;
  border-right: 1px solid var(--n-border-color, #e0e0e6);
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex-shrink: 0;
}

.translations-section {
  flex: 0 0 40%;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.episodes-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.styles-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}

.section-header {
  padding: 6px 10px;
  font-weight: 600;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  background: var(--n-color, #fff);
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.styles-header {
  gap: 10px;
}

.header-muted {
  font-weight: 400;
  color: var(--n-text-color-3, #888);
}

.load-message {
  padding: 6px 10px;
  font-size: 11px;
  color: var(--n-text-color-3, #888);
}

/* Translations */
.trans-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 5px 10px;
  cursor: pointer;
  font-size: 12px;
  border-left: 3px solid transparent;
}
.trans-row:hover {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.04));
}
.trans-row.active {
  background: var(--n-color-target-hover, rgba(32, 128, 240, 0.12));
  border-left-color: var(--n-color-target, #2080f0);
}
.trans-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}
.trans-coverage {
  color: var(--n-text-color-3, #888);
  font-size: 11px;
  white-space: nowrap;
}

/* Episodes */
.ep-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 10px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1.3;
}
.ep-row:hover {
  background: var(--n-color-hover, rgba(0, 0, 0, 0.04));
}
.ep-row.disabled {
  opacity: 0.5;
}
.ep-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ep-badge {
  display: inline-block;
  background: var(--n-color-target, #2080f0);
  color: white;
  font-size: 10px;
  font-weight: 600;
  padding: 1px 4px;
  border-radius: 3px;
}

/* Styles */
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
  color: var(--n-text-color-3, #888);
  white-space: nowrap;
}
.style-info {
  font-size: 11px;
  color: var(--n-text-color-3, #888);
  margin-top: 2px;
}
.instance-count {
  margin-left: 6px;
  opacity: 0.7;
}
</style>
