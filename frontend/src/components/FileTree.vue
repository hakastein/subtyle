<script setup lang="ts">
import { computed, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { NTree, NEmpty, NText } from 'naive-ui'
import type { TreeOption } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import type { ScannedFile } from '@/services/types'

const { t } = useI18n()
const projectStore = useProjectStore()

// Build tree data from scanned and loaded files
const treeData = computed<TreeOption[]>(() => {
  return projectStore.scannedFiles.map((scanned) => {
    const loaded = findLoadedByPath(scanned.path)
    return buildFileNode(scanned, loaded)
  })
})

function findLoadedByPath(path: string) {
  for (const [, file] of projectStore.loadedFiles) {
    if (file.path === path) return file
  }
  return null
}

function findLoadedByVideoAndTrack(videoPath: string, trackIndex: number) {
  for (const [, file] of projectStore.loadedFiles) {
    if (file.videoPath === videoPath && file.trackId === trackIndex) return file
  }
  return null
}

function buildFileNode(scanned: ScannedFile, loaded: ReturnType<typeof findLoadedByPath>): TreeOption {
  const isEmbedded = scanned.type === 'embedded'
  const fileName = scanned.path.split('/').pop() ?? scanned.path

  if (isEmbedded) {
    // Embedded: show video file with tracks as children
    const videoName = scanned.videoPath.split('/').pop() ?? scanned.videoPath
    const children: TreeOption[] = scanned.tracks.map((track) => {
      const trackLoaded = findLoadedByVideoAndTrack(scanned.videoPath, track.index)
      const trackLabel = track.title
        ? `Track ${track.index}: ${track.title} (${track.language})`
        : `Track ${track.index} (${track.language})`

      if (trackLoaded) {
        // Track is loaded, show its styles
        return {
          key: `embedded::${scanned.videoPath}::${track.index}`,
          label: trackLabel,
          children: trackLoaded.modifiedStyles.map((style) => ({
            key: `${trackLoaded.id}::${style.name}`,
            label: style.name,
            isLeaf: true,
            suffix: () => h(NText, { depth: 3, style: 'font-size: 11px; margin-left: 6px' }, { default: () => `${style.fontName} ${style.fontSize}` }),
          })),
        }
      } else {
        // Track not loaded yet
        return {
          key: `track::${scanned.videoPath}::${track.index}`,
          label: trackLabel,
          isLeaf: true,
        }
      }
    })

    return {
      key: `video::${scanned.videoPath}`,
      label: `🎬 ${videoName}`,
      children,
    }
  } else {
    // External subtitle file
    if (loaded) {
      return {
        key: `file::${scanned.path}`,
        label: fileName,
        children: loaded.modifiedStyles.map((style) => ({
          key: `${loaded.id}::${style.name}`,
          label: style.name,
          isLeaf: true,
          suffix: () => h(NText, { depth: 3, style: 'font-size: 11px; margin-left: 6px' }, { default: () => `${style.fontName} ${style.fontSize}` }),
        })),
      }
    } else {
      return {
        key: `file::${scanned.path}`,
        label: fileName,
        isLeaf: false,
        children: [],
      }
    }
  }
}

const selectedKeys = computed({
  get: () => projectStore.selectedStyleKeys,
  set: (keys: string[]) => {
    projectStore.selectedStyleKeys = keys
  },
})

async function handleLoad(keys: string[], option: TreeOption | null) {
  if (!option) return
  const key = String(option.key ?? '')

  if (key.startsWith('file::')) {
    const path = key.slice('file::'.length)
    const scanned = projectStore.scannedFiles.find(f => f.path === path)
    if (scanned && !findLoadedByPath(path)) {
      await projectStore.loadFile(scanned)
    }
  } else if (key.startsWith('track::')) {
    // Format: track::videoPath::trackIndex
    const rest = key.slice('track::'.length)
    const lastColon = rest.lastIndexOf('::')
    const videoPath = rest.slice(0, lastColon)
    const trackIndex = parseInt(rest.slice(lastColon + 2))
    await projectStore.extractTrack(videoPath, trackIndex)
  }
}

function handleUpdateExpandedKeys(
  _keys: Array<string & number>,
  _option: Array<TreeOption | null>,
  meta: { node: TreeOption; action: 'expand' | 'collapse' } | { node: null; action: 'filter' },
) {
  if (meta.action === 'expand' && meta.node) {
    handleLoad([], meta.node)
  }
}

function handleUpdateSelectedKeys(keys: string[]) {
  // Only propagate leaf style keys (format: fileId::styleName)
  const styleKeys = keys.filter(k => {
    return !k.startsWith('file::') && !k.startsWith('video::') &&
           !k.startsWith('embedded::') && !k.startsWith('track::')
  })
  projectStore.selectedStyleKeys = styleKeys
}
</script>

<template>
  <div class="file-tree">
    <div class="file-tree-header">{{ t('fileTree.title') }}</div>
    <NEmpty
      v-if="projectStore.scannedFiles.length === 0"
      :description="t('fileTree.noFiles')"
      style="padding: 20px 12px"
    />
    <NTree
      v-else
      :data="treeData"
      :selected-keys="selectedKeys"
      multiple
      block-line
      expand-on-click
      @update:selected-keys="handleUpdateSelectedKeys"
      @update:expanded-keys="handleUpdateExpandedKeys"
      style="padding: 4px"
    />
  </div>
</template>

<style scoped>
.file-tree {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.file-tree-header {
  padding: 8px 12px;
  font-weight: 600;
  font-size: 13px;
  border-bottom: 1px solid var(--n-border-color, #e0e0e6);
  flex-shrink: 0;
}

:deep(.n-tree) {
  overflow-y: auto;
  flex: 1;
}
</style>
