import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type { ScannedFile, SubtitleStyle, SubtitleEvent, UndoChange, ProjectState, FileState } from '@/services/types'
import { useUndoStore } from './undo'
import { usePreviewStore } from './preview'
import { useDebugStore } from './debug'
import * as scanService from '@/services/scan'
import * as parserService from '@/services/parser'
import * as editorService from '@/services/editor'
import * as projectService from '@/services/project'
import { durationToMs } from '@/services/types'
import { basename, stripExt, detectEpisodeNumbers, collapseRanges, subtitleSuffix } from '@/services/episodes'

interface LoadedFile {
  id: string
  path: string
  videoPath: string
  source: 'external' | 'embedded'
  trackId: number
  trackTitle: string
  originalStyles: SubtitleStyle[]
  modifiedStyles: SubtitleStyle[]
  events: SubtitleEvent[]
}

export interface VideoEntry {
  videoPath: string
  videoName: string
  episode: number | null
}

export interface SourceTypeInstance {
  videoPath: string
  subtitlePath?: string  // for external
  trackIndex?: number    // for embedded
  trackTitle?: string    // for embedded
}

export interface SourceType {
  key: string
  label: string
  kind: 'external' | 'embedded'
  perVideo: SourceTypeInstance[]
}

export interface GroupedStyleInstance {
  videoPath: string
  episode: number | null
  fileId: string
  styleName: string
}

export interface GroupedStyle {
  styleName: string
  representative: SubtitleStyle
  instances: GroupedStyleInstance[]
  episodesLabel: string
}

let autosaveTimer: ReturnType<typeof setTimeout> | null = null

export const useProjectStore = defineStore('project', () => {
  const undoStore = useUndoStore()
  const previewStore = usePreviewStore()
  const debug = useDebugStore()

  const folderPath = ref('')
  const scannedFiles = ref<ScannedFile[]>([])
  const loadedFiles = ref<Map<string, LoadedFile>>(new Map())
  const selectedStyleKeys = ref<string[]>([]) // format: "fileId::styleName"
  const dirty = ref(false)

  // New video-centric state for the split FileTree UI
  const fileChecks = ref<Map<string, boolean>>(new Map()) // videoPath → checked
  const selectedSourceKey = ref<string | null>(null)
  const sourceLoadingState = ref<'idle' | 'loading'>('idle')

  // Computed

  const activeFile = computed<LoadedFile | null>(() => {
    if (selectedStyleKeys.value.length === 0) return null
    const [fileId] = selectedStyleKeys.value[0].split('::')
    return loadedFiles.value.get(fileId) ?? null
  })

  const selectedStyles = computed<Array<{ fileId: string; style: SubtitleStyle }>>(() => {
    const result: Array<{ fileId: string; style: SubtitleStyle }> = []
    for (const key of selectedStyleKeys.value) {
      const [fileId, styleName] = key.split('::')
      const file = loadedFiles.value.get(fileId)
      if (!file) continue
      const style = file.modifiedStyles.find(s => s.name === styleName)
      if (style) result.push({ fileId, style })
    }
    return result
  })

  // Unique video files from scan results, with detected episode numbers.
  const videoEntries = computed<VideoEntry[]>(() => {
    const videoPaths = new Set<string>()
    for (const sf of scannedFiles.value) {
      if (sf.videoPath) videoPaths.add(sf.videoPath)
      else if (sf.type === 'embedded') videoPaths.add(sf.path)
    }
    const sorted = Array.from(videoPaths).sort()
    const episodes = detectEpisodeNumbers(sorted)
    return sorted.map(vp => ({
      videoPath: vp,
      videoName: basename(vp),
      episode: episodes.get(vp) ?? null,
    }))
  })

  // All available subtitle source types across all videos in the folder.
  // External sources are grouped by filename suffix (after stripping video base name).
  // Embedded sources are grouped by track title.
  const sourceTypes = computed<SourceType[]>(() => {
    const byKey = new Map<string, SourceType>()

    for (const sf of scannedFiles.value) {
      if (sf.type === 'external' && sf.videoPath) {
        const suffix = subtitleSuffix(sf.videoPath, sf.path)
        const key = `ext:${suffix}`
        if (!byKey.has(key)) {
          byKey.set(key, { key, label: suffix || sf.path, kind: 'external', perVideo: [] })
        }
        byKey.get(key)!.perVideo.push({ videoPath: sf.videoPath, subtitlePath: sf.path })
      } else if (sf.type === 'embedded') {
        for (const track of sf.tracks) {
          const title = track.title || `Track ${track.index}`
          const key = `emb:${title}:${track.language}`
          if (!byKey.has(key)) {
            const label = track.language ? `${title} (${track.language})` : title
            byKey.set(key, { key, label, kind: 'embedded', perVideo: [] })
          }
          byKey.get(key)!.perVideo.push({
            videoPath: sf.videoPath,
            trackIndex: track.index,
            trackTitle: track.title,
          })
        }
      }
    }

    return Array.from(byKey.values()).sort((a, b) => a.label.localeCompare(b.label))
  })

  /** Get loaded file for a given video path matching the selected source. */
  function findLoadedForSource(videoPath: string, source: SourceType): LoadedFile | null {
    const inst = source.perVideo.find(i => i.videoPath === videoPath)
    if (!inst) return null

    if (source.kind === 'external' && inst.subtitlePath) {
      const fileId = basename(inst.subtitlePath)
      return loadedFiles.value.get(fileId) ?? null
    }
    if (source.kind === 'embedded' && inst.trackIndex !== undefined) {
      const fileId = `${basename(videoPath)}:track:${inst.trackIndex}`
      return loadedFiles.value.get(fileId) ?? null
    }
    return null
  }

  // Styles grouped by name for the currently selected source, across checked files.
  const groupedStyles = computed<GroupedStyle[]>(() => {
    if (!selectedSourceKey.value) return []
    const source = sourceTypes.value.find(s => s.key === selectedSourceKey.value)
    if (!source) return []

    const groups = new Map<string, GroupedStyle>()

    for (const entry of videoEntries.value) {
      const checked = fileChecks.value.get(entry.videoPath) ?? true
      if (!checked) continue

      const loaded = findLoadedForSource(entry.videoPath, source)
      if (!loaded) continue

      for (const style of loaded.modifiedStyles) {
        if (!groups.has(style.name)) {
          groups.set(style.name, {
            styleName: style.name,
            representative: style,
            instances: [],
            episodesLabel: '',
          })
        }
        groups.get(style.name)!.instances.push({
          videoPath: entry.videoPath,
          episode: entry.episode,
          fileId: loaded.id,
          styleName: style.name,
        })
      }
    }

    // Compute episode ranges for each group
    for (const group of groups.values()) {
      const eps = group.instances.map(i => i.episode).filter((e): e is number => e !== null)
      group.episodesLabel = eps.length > 0 ? collapseRanges(eps) : ''
    }

    return Array.from(groups.values()).sort((a, b) => a.styleName.localeCompare(b.styleName))
  })

  /** Set selection to all instances of the given grouped style. */
  function selectGroupedStyle(styleName: string, additive: boolean = false): void {
    const group = groupedStyles.value.find(g => g.styleName === styleName)
    if (!group) return
    const keys = group.instances.map(i => `${i.fileId}::${i.styleName}`)
    if (additive) {
      const existing = new Set(selectedStyleKeys.value)
      for (const k of keys) existing.add(k)
      selectedStyleKeys.value = Array.from(existing)
    } else {
      selectedStyleKeys.value = keys
    }
  }

  /** Auto-load all subtitles for the selected source type across checked videos. */
  async function loadSourceStyles(): Promise<void> {
    if (!selectedSourceKey.value) return
    const source = sourceTypes.value.find(s => s.key === selectedSourceKey.value)
    if (!source) return

    sourceLoadingState.value = 'loading'
    try {
      for (const inst of source.perVideo) {
        const checked = fileChecks.value.get(inst.videoPath) ?? true
        if (!checked) continue

        if (source.kind === 'external' && inst.subtitlePath) {
          const fileId = basename(inst.subtitlePath)
          if (loadedFiles.value.has(fileId)) continue
          const scanned = scannedFiles.value.find(f => f.path === inst.subtitlePath)
          if (scanned) await loadFile(scanned)
        } else if (source.kind === 'embedded' && inst.trackIndex !== undefined) {
          const fileId = `${basename(inst.videoPath)}:track:${inst.trackIndex}`
          if (loadedFiles.value.has(fileId)) continue
          await extractTrack(inst.videoPath, inst.trackIndex, inst.trackTitle || '')
        }
      }
    } finally {
      sourceLoadingState.value = 'idle'
    }
  }

  // Watch source / file-checks changes to auto-load.
  watch([selectedSourceKey, fileChecks], () => {
    if (selectedSourceKey.value) {
      loadSourceStyles().catch((err: unknown) => {
        debug.error(`loadSourceStyles failed: ${err}`)
      })
    }
  }, { deep: true })

  // Actions

  async function openFolder(): Promise<void> {
    const path = await scanService.openFolder()
    if (!path) return

    debug.info(`openFolder: ${path}`)
    folderPath.value = path
    scannedFiles.value = []
    loadedFiles.value = new Map()
    selectedStyleKeys.value = []
    dirty.value = false
    fileChecks.value = new Map()
    selectedSourceKey.value = null
    undoStore.clear()
    previewStore.clearFrame()

    const result = await scanService.scanFolder(path)
    scannedFiles.value = result.files
    debug.info(`scanFolder: found ${result.files.length} files`)
    for (const f of result.files) {
      debug.info(`  ${f.type}: ${f.path} → video: ${f.videoPath || 'none'}`)
    }

    // Initialize all video checkboxes as checked
    const checks = new Map<string, boolean>()
    for (const entry of videoEntries.value) {
      checks.set(entry.videoPath, true)
    }
    fileChecks.value = checks
  }

  async function loadFile(scannedFile: ScannedFile): Promise<LoadedFile> {
    debug.info(`loadFile: ${scannedFile.path} (video: ${scannedFile.videoPath || 'none'})`)
    const parsed = await parserService.parseFile(scannedFile.path)
    debug.info(`  parsed: ${parsed.styles.length} styles, ${parsed.events.length} events`)
    const loaded: LoadedFile = {
      id: parsed.id,
      path: parsed.path,
      videoPath: scannedFile.videoPath,
      source: parsed.source,
      trackId: parsed.trackId,
      trackTitle: '',
      originalStyles: structuredClone(parsed.styles),
      modifiedStyles: structuredClone(parsed.styles),
      events: parsed.events,
    }
    loadedFiles.value.set(loaded.id, loaded)

    if (scannedFile.videoPath) {
      try {
        const durationMs = await editorService.getVideoDuration(scannedFile.videoPath)
        previewStore.videoDurationMs = durationMs
        debug.info(`  video duration: ${previewStore.videoDurationMs}ms`)
      } catch (err) {
        debug.error(`  video duration failed: ${err}`)
      }
    }

    return loaded
  }

  async function extractTrack(videoPath: string, trackIndex: number, trackTitle: string = ''): Promise<LoadedFile> {
    debug.info(`extractTrack: video=${videoPath} track=${trackIndex} title=${trackTitle}`)
    const parsed = await parserService.extractTrack(videoPath, trackIndex, trackTitle)
    debug.info(`  extracted: ${parsed.styles.length} styles, ${parsed.events.length} events, id=${parsed.id}`)
    const loaded: LoadedFile = {
      id: parsed.id,
      path: parsed.path,
      videoPath,
      source: parsed.source as 'external' | 'embedded',
      trackId: parsed.trackId,
      trackTitle,
      originalStyles: structuredClone(parsed.styles),
      modifiedStyles: structuredClone(parsed.styles),
      events: parsed.events,
    }
    loadedFiles.value.set(loaded.id, loaded)

    try {
      const durationMs = await editorService.getVideoDuration(videoPath)
      previewStore.videoDurationMs = durationMs
      debug.info(`  video duration: ${durationMs}ms`)
    } catch (err) {
      debug.error(`  video duration failed: ${err}`)
    }

    return loaded
  }

  function selectStyle(fileId: string, styleName: string, multi: boolean): void {
    const key = `${fileId}::${styleName}`
    if (multi) {
      const idx = selectedStyleKeys.value.indexOf(key)
      if (idx >= 0) {
        selectedStyleKeys.value.splice(idx, 1)
      } else {
        selectedStyleKeys.value.push(key)
      }
    } else {
      selectedStyleKeys.value = [key]
    }
  }

  function updateStyle(fileId: string, styleName: string, field: string, value: unknown): void {
    // Gather all selected keys that match the field update scope.
    // The primary target is fileId::styleName, but we apply to ALL selected styles.
    const keysToUpdate = selectedStyleKeys.value.length > 0
      ? selectedStyleKeys.value
      : [`${fileId}::${styleName}`]

    const changes: UndoChange[] = []

    for (const key of keysToUpdate) {
      const [fId, sName] = key.split('::')
      const file = loadedFiles.value.get(fId)
      if (!file) continue
      const style = file.modifiedStyles.find(s => s.name === sName)
      if (!style) continue

      const oldValue = structuredClone((style as Record<string, unknown>)[field])
      ;(style as Record<string, unknown>)[field] = value
      changes.push({ fileId: fId, styleName: sName, field, oldValue, newValue: value })
    }

    if (changes.length > 0) {
      undoStore.push(`Update ${field}`, changes)
      dirty.value = true
      scheduleAutosave()
    }
  }

  function applyChanges(changes: UndoChange[], useOld: boolean): void {
    for (const change of changes) {
      const file = loadedFiles.value.get(change.fileId)
      if (!file) continue
      const style = file.modifiedStyles.find(s => s.name === change.styleName)
      if (!style) continue
      ;(style as Record<string, unknown>)[change.field] = useOld ? change.oldValue : change.newValue
    }
  }

  function applyUndo(): void {
    const entry = undoStore.undo()
    if (!entry) return
    applyChanges(entry.changes, true)
    dirty.value = true
    scheduleAutosave()
  }

  function applyRedo(): void {
    const entry = undoStore.redo()
    if (!entry) return
    applyChanges(entry.changes, false)
    dirty.value = true
    scheduleAutosave()
  }

  async function save(): Promise<string[]> {
    const requests: parserService.SaveRequest[] = []
    for (const [id, file] of loadedFiles.value) {
      requests.push({
        fileId: id,
        videoPath: file.videoPath,
        styles: file.modifiedStyles,
      })
    }
    debug.info(`save: ${requests.length} files`)
    const paths = await parserService.saveAll(requests)
    debug.info(`save: wrote ${paths.length} files: ${paths.join(', ')}`)
    dirty.value = false
    // Update originals to match saved state
    for (const file of loadedFiles.value.values()) {
      file.originalStyles = structuredClone(file.modifiedStyles)
    }
    return paths
  }

  function scheduleAutosave(): void {
    if (autosaveTimer !== null) {
      clearTimeout(autosaveTimer)
    }
    autosaveTimer = setTimeout(() => {
      autosaveTimer = null
      doAutosave()
    }, 2000)
  }

  async function doAutosave(): Promise<void> {
    const files: FileState[] = []
    for (const file of loadedFiles.value.values()) {
      files.push({
        id: file.id,
        path: file.path,
        source: file.source,
        trackId: file.trackId,
        videoPath: file.videoPath,
        originalStyles: structuredClone(file.originalStyles),
        modifiedStyles: structuredClone(file.modifiedStyles),
        events: file.events,
      })
    }

    const state: ProjectState = {
      folderPath: folderPath.value,
      savedAt: new Date().toISOString(),
      dirty: dirty.value,
      files,
      undoStack: structuredClone(undoStore.undoStack),
      redoStack: structuredClone(undoStore.redoStack),
      activeFileId: activeFile.value?.id ?? '',
      selectedStyles: selectedStyleKeys.value.slice(),
    }

    await projectService.autosave(state)
  }

  function restoreFromAutosave(state: ProjectState): void {
    folderPath.value = state.folderPath
    dirty.value = state.dirty
    selectedStyleKeys.value = state.selectedStyles.slice()

    const map = new Map<string, LoadedFile>()
    for (const fs of state.files) {
      map.set(fs.id, {
        id: fs.id,
        path: fs.path,
        videoPath: fs.videoPath,
        source: fs.source as 'external' | 'embedded',
        trackId: fs.trackId,
        trackTitle: '',
        originalStyles: structuredClone(fs.originalStyles),
        modifiedStyles: structuredClone(fs.modifiedStyles),
        events: fs.events,
      })
    }
    loadedFiles.value = map

    undoStore.restore(
      structuredClone(state.undoStack),
      structuredClone(state.redoStack),
    )
  }

  function getEventsForStyle(fileId: string, styleName: string): SubtitleEvent[] {
    const file = loadedFiles.value.get(fileId)
    if (!file) return []
    return file.events.filter(e => e.styleName === styleName)
  }

  return {
    folderPath,
    scannedFiles,
    loadedFiles,
    selectedStyleKeys,
    dirty,
    activeFile,
    selectedStyles,
    // Video-centric state
    fileChecks,
    selectedSourceKey,
    sourceLoadingState,
    videoEntries,
    sourceTypes,
    groupedStyles,
    selectGroupedStyle,
    // Actions
    openFolder,
    loadFile,
    extractTrack,
    selectStyle,
    updateStyle,
    applyUndo,
    applyRedo,
    save,
    scheduleAutosave,
    doAutosave,
    restoreFromAutosave,
    getEventsForStyle,
  }
})
