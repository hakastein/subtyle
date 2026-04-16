<script setup lang="ts">
import { computed, h, ref, onMounted, onUnmounted } from 'vue'
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
        // Look up the track title from scanned files
        const scanned = projectStore.scannedFiles.find(
          f => f.videoPath === videoPath && f.type === 'embedded',
        )
        const track = scanned?.tracks.find(t => t.index === trackIndex)
        const title = track?.title ?? ''
        debug.info(`FileTree: extracting track ${trackIndex} (${title || 'no title'}) from ${basename(videoPath)}`)
        try {
          await projectStore.extractTrack(videoPath, trackIndex, title)
        } catch (err) {
          debug.error(`FileTree: extract failed — ${err}`)
        }
      }
    }
  }
}

// Track modifier keys for click selection logic
const modifiers = { ctrl: false, shift: false }

function handleMouseDown(e: MouseEvent) {
  modifiers.ctrl = e.ctrlKey || e.metaKey
  modifiers.shift = e.shiftKey
}

onMounted(() => {
  document.addEventListener('mousedown', handleMouseDown, true)
})

onUnmounted(() => {
  document.removeEventListener('mousedown', handleMouseDown, true)
})

/** Flatten tree to ordered list of visible style keys (for range selection) */
function flattenStyleKeys(): string[] {
  const result: string[] = []
  function walk(nodes: TreeOption[]) {
    for (const node of nodes) {
      const key = String(node.key ?? '')
      if (key && !key.startsWith('file:') && !key.startsWith('video:') && !key.startsWith('track:')) {
        result.push(key)
      }
      if (node.children && expandedKeys.value.includes(String(node.key ?? ''))) {
        walk(node.children)
      }
    }
  }
  walk(treeData.value)
  return result
}

function handleSelect(
  keys: string[],
  _option: Array<TreeOption | null>,
  meta: { node: TreeOption | null; action: 'select' | 'unselect' },
) {
  const clickedKey = String(meta.node?.key ?? '')
  const isStyleKey = clickedKey &&
    !clickedKey.startsWith('file:') &&
    !clickedKey.startsWith('video:') &&
    !clickedKey.startsWith('track:')

  // If a non-style node was clicked, ignore selection change (keep current)
  if (!isStyleKey) {
    return
  }

  const prev = projectStore.selectedStyleKeys

  // Shift+click: range select from last selected to clicked
  if (modifiers.shift && prev.length > 0) {
    const visible = flattenStyleKeys()
    const lastSelected = prev[prev.length - 1]
    const startIdx = visible.indexOf(lastSelected)
    const endIdx = visible.indexOf(clickedKey)
    if (startIdx >= 0 && endIdx >= 0) {
      const [lo, hi] = startIdx < endIdx ? [startIdx, endIdx] : [endIdx, startIdx]
      projectStore.selectedStyleKeys = visible.slice(lo, hi + 1)
      debug.info(`FileTree: shift-select range ${lo}..${hi}`)
      return
    }
  }

  // Ctrl+click: toggle add/remove from selection
  if (modifiers.ctrl) {
    if (meta.action === 'unselect') {
      projectStore.selectedStyleKeys = prev.filter(k => k !== clickedKey)
    } else {
      projectStore.selectedStyleKeys = prev.includes(clickedKey)
        ? prev.filter(k => k !== clickedKey)
        : [...prev, clickedKey]
    }
    debug.info(`FileTree: ctrl-toggle ${clickedKey}`)
    return
  }

  // Plain click: single select (replace)
  const styleKeys = keys.filter(k =>
    !k.startsWith('file:') && !k.startsWith('video:') && !k.startsWith('track:')
  )
  // If NTree tried to add multiple or toggle, force single selection to clicked
  projectStore.selectedStyleKeys = [clickedKey]
  debug.info(`FileTree: single-select ${clickedKey}`)
  void styleKeys
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
