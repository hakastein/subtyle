import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ScannedFile, SubtitleStyle, SubtitleEvent, UndoChange, ProjectState, FileState } from '@/services/types'
import { useUndoStore } from './undo'
import { usePreviewStore } from './preview'
import * as scanService from '@/services/scan'
import * as parserService from '@/services/parser'
import * as editorService from '@/services/editor'
import * as projectService from '@/services/project'
import { durationToMs } from '@/services/types'

interface LoadedFile {
  id: string
  path: string
  videoPath: string
  source: 'external' | 'embedded'
  trackId: number
  originalStyles: SubtitleStyle[]
  modifiedStyles: SubtitleStyle[]
  events: SubtitleEvent[]
}

let autosaveTimer: ReturnType<typeof setTimeout> | null = null

export const useProjectStore = defineStore('project', () => {
  const undoStore = useUndoStore()
  const previewStore = usePreviewStore()

  const folderPath = ref('')
  const scannedFiles = ref<ScannedFile[]>([])
  const loadedFiles = ref<Map<string, LoadedFile>>(new Map())
  const selectedStyleKeys = ref<string[]>([]) // format: "fileId::styleName"
  const dirty = ref(false)

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

  // Actions

  async function openFolder(): Promise<void> {
    const path = await scanService.openFolder()
    if (!path) return

    folderPath.value = path
    scannedFiles.value = []
    loadedFiles.value = new Map()
    selectedStyleKeys.value = []
    dirty.value = false
    undoStore.clear()
    previewStore.clearFrame()

    const result = await scanService.scanFolder(path)
    scannedFiles.value = result.files
  }

  async function loadFile(scannedFile: ScannedFile): Promise<LoadedFile> {
    const parsed = await parserService.parseFile(scannedFile.path)
    const loaded: LoadedFile = {
      id: parsed.id,
      path: parsed.path,
      videoPath: scannedFile.videoPath,
      source: parsed.source,
      trackId: parsed.trackId,
      originalStyles: structuredClone(parsed.styles),
      modifiedStyles: structuredClone(parsed.styles),
      events: parsed.events,
    }
    loadedFiles.value.set(loaded.id, loaded)

    if (scannedFile.videoPath) {
      try {
        const durationNs = await editorService.getVideoDuration(scannedFile.videoPath)
        previewStore.videoDurationMs = durationToMs(durationNs)
      } catch {
        // ignore if video duration unavailable
      }
    }

    return loaded
  }

  async function extractTrack(videoPath: string, trackIndex: number): Promise<LoadedFile> {
    const parsed = await parserService.extractTrack(videoPath, trackIndex)
    const loaded: LoadedFile = {
      id: parsed.id,
      path: parsed.path,
      videoPath,
      source: parsed.source,
      trackId: parsed.trackId,
      originalStyles: structuredClone(parsed.styles),
      modifiedStyles: structuredClone(parsed.styles),
      events: parsed.events,
    }
    loadedFiles.value.set(loaded.id, loaded)

    try {
      const durationNs = await editorService.getVideoDuration(videoPath)
      previewStore.videoDurationMs = durationToMs(durationNs)
    } catch {
      // ignore if video duration unavailable
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

  async function save(): Promise<void> {
    const fileStyles: Record<string, SubtitleStyle[]> = {}
    for (const [id, file] of loadedFiles.value) {
      fileStyles[id] = file.modifiedStyles
    }
    await parserService.saveAll(fileStyles)
    dirty.value = false
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
