import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { UndoEntry, UndoChange } from '@/services/types'

export const useUndoStore = defineStore('undo', () => {
  const undoStack = ref<UndoEntry[]>([])
  const redoStack = ref<UndoEntry[]>([])
  let nextId = 1

  function push(description: string, changes: UndoChange[]): UndoEntry {
    const entry: UndoEntry = { id: nextId++, description, changes }
    undoStack.value.push(entry)
    redoStack.value = [] // clear redo on new action
    return entry
  }

  function undo(): UndoEntry | null {
    const entry = undoStack.value.pop()
    if (!entry) return null
    redoStack.value.push(entry)
    return entry
  }

  function redo(): UndoEntry | null {
    const entry = redoStack.value.pop()
    if (!entry) return null
    undoStack.value.push(entry)
    return entry
  }

  function canUndo(): boolean { return undoStack.value.length > 0 }
  function canRedo(): boolean { return redoStack.value.length > 0 }

  function clear(): void {
    undoStack.value = []
    redoStack.value = []
    nextId = 1
  }

  function restore(undo: UndoEntry[], redo: UndoEntry[]): void {
    undoStack.value = undo
    redoStack.value = redo
    const maxId = Math.max(0, ...undo.map(e => e.id), ...redo.map(e => e.id))
    nextId = maxId + 1
  }

  return { undoStack, redoStack, push, undo, redo, canUndo, canRedo, clear, restore }
})
