<script setup lang="ts">
import { computed, h, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { NTree, NEmpty, NText } from 'naive-ui'
import type { TreeOption } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import { useDebugStore } from '@/stores/debug'
import type { ScannedFile, SubtitleStyle } from '@/services/types'

const { t } = useI18n()
const projectStore = useProjectStore()
const debug = useDebugStore()

const expandedKeys = ref<string[]>([])

/** Get basename from a path (handles both / and \) */
function basename(path: string): string {
  return path.replace(/^.*[/\\]/, '')
}

/** Find loaded file by its ID */
function findLoaded(id: string) {
  return projectStore.loadedFiles.get(id) ?? null
}

/** Build a style suffix showing font info */
function styleSuffix(style: SubtitleStyle) {
  return h(NText, { depth: 3, style: 'font-size: 11px; margin-left: 6px' }, {
    default: () => `${style.fontName} ${style.fontSize}`,
  })
}

/** Build style leaf nodes for a loaded file */
function styleNodes(fileId: string, styles: SubtitleStyle[]): TreeOption[] {
  return styles.map((style) => ({
    key: `${fileId}::${style.name}`,
    label: style.name,
    isLeaf: true,
    suffix: () => styleSuffix(style),
  }))
}

const treeData = computed<TreeOption[]>(() => {
  return projectStore.scannedFiles.map((scanned) => {
    if (scanned.type === 'embedded') {
      return buildEmbeddedNode(scanned)
    }
    return buildExternalNode(scanned)
  })
})

function buildExternalNode(scanned: ScannedFile): TreeOption {
  const fileId = basename(scanned.path)
  const loaded = findLoaded(fileId)

  return {
    key: `file:${scanned.path}`,
    label: basename(scanned.path),
    isLeaf: false,
    children: loaded ? styleNodes(loaded.id, loaded.modifiedStyles) : [],
  }
}

function buildEmbeddedNode(scanned: ScannedFile): TreeOption {
  const children: TreeOption[] = scanned.tracks.map((track) => {
    const trackId = `${basename(scanned.videoPath)}:track:${track.index}`
    const loaded = findLoaded(trackId)
    const label = track.title
      ? `${track.title} (${track.language || '?'})`
      : `Track ${track.index} (${track.language || '?'})`

    if (loaded) {
      return {
        key: `track:${scanned.videoPath}:${track.index}`,
        label,
        isLeaf: false,
        children: styleNodes(loaded.id, loaded.modifiedStyles),
      }
    }

    return {
      key: `track:${scanned.videoPath}:${track.index}`,
      label,
      isLeaf: false,
      children: [], // will be populated after extraction
    }
  })

  return {
    key: `video:${scanned.videoPath}`,
    label: `🎬 ${basename(scanned.videoPath)}`,
    children,
  }
}

async function handleExpand(keys: string[]) {
  expandedKeys.value = keys

  for (const key of keys) {
    // External file expand → load & parse
    if (key.startsWith('file:')) {
      const path = key.slice('file:'.length)
      const fileId = basename(path)
      if (!findLoaded(fileId)) {
        const scanned = projectStore.scannedFiles.find(f => f.path === path)
        if (scanned) {
          debug.info(`FileTree: loading external ${basename(path)}`)
          await projectStore.loadFile(scanned)
        }
      }
    }

    // Track expand → extract from video
    if (key.startsWith('track:')) {
      const rest = key.slice('track:'.length)
      // Parse from the end: last segment is trackIndex
      const lastColon = rest.lastIndexOf(':')
      const videoPath = rest.slice(0, lastColon)
      const trackIndex = parseInt(rest.slice(lastColon + 1))
      const trackId = `${basename(videoPath)}:track:${trackIndex}`

      if (!findLoaded(trackId) && !isNaN(trackIndex)) {
        debug.info(`FileTree: extracting track ${trackIndex} from ${basename(videoPath)}`)
        try {
          await projectStore.extractTrack(videoPath, trackIndex)
        } catch (err) {
          debug.error(`FileTree: extract failed — ${err}`)
        }
      }
    }
  }
}

function handleSelect(keys: string[]) {
  // Only propagate style keys (contain :: with a style name)
  const styleKeys = keys.filter(k =>
    !k.startsWith('file:') && !k.startsWith('video:') && !k.startsWith('track:')
  )
  if (styleKeys.length > 0) {
    debug.info(`FileTree: selected ${styleKeys.join(', ')}`)
  }
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
      :selected-keys="projectStore.selectedStyleKeys"
      :expanded-keys="expandedKeys"
      multiple
      block-line
      expand-on-click
      @update:selected-keys="handleSelect"
      @update:expanded-keys="handleExpand"
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
