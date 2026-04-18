export interface Color {
  r: number
  g: number
  b: number
  a: number // 0-255, 255 = opaque
}

export interface SubtitleStyle {
  name: string
  fontName: string
  fontSize: number
  bold: boolean
  italic: boolean
  underline: boolean
  strikeout: boolean
  primaryColour: Color
  secondaryColour: Color
  outlineColour: Color
  backColour: Color
  outline: number
  shadow: number
  scaleX: number
  scaleY: number
  spacing: number
  angle: number
  alignment: number // 1-9
  marginL: number
  marginR: number
  marginV: number
}

export interface SubtitleEvent {
  styleName: string
  startTime: number // nanoseconds (Go time.Duration)
  endTime: number
  text: string
}

export interface SubtitleFile {
  id: string
  path: string
  source: 'external' | 'embedded'
  trackId: number
  styles: SubtitleStyle[]
  events: SubtitleEvent[]
}

export interface TrackInfo {
  index: number
  language: string
  title: string
}

export interface ScannedFile {
  path: string
  videoPath: string
  type: 'external' | 'embedded'
  tracks: TrackInfo[]
}

export interface FolderScanResult {
  files: ScannedFile[]
}

export interface FrameResult {
  base64Png: string
  timecode: string
}

export interface UndoChange {
  fileId: string
  styleName: string
  field: string
  oldValue: unknown
  newValue: unknown
}

export interface UndoEntry {
  id: number
  description: string
  changes: UndoChange[]
}

export interface FileState {
  id: string
  path: string
  source: string
  trackId: number
  videoPath: string
  originalStyles: SubtitleStyle[]
  modifiedStyles: SubtitleStyle[]
  events: SubtitleEvent[]
}

export interface ProjectState {
  folderPath: string
  savedAt: string
  dirty: boolean
  files: FileState[]
  undoStack: UndoEntry[]
  redoStack: UndoEntry[]
  activeFileId: string
  selectedStyles: string[]
}

export interface TranslationInstance {
  videoPath: string
  subtitlePath?: string
  trackIndex?: number
  trackTitle?: string
}

export interface Translation {
  key: string
  label: string
  kind: 'external' | 'embedded'
  perEpisode: Record<string, TranslationInstance> // videoPath → instance
  coverageCount: number
  totalEpisodes: number
}

/** Convert Go time.Duration (nanoseconds) to milliseconds */
export function durationToMs(ns: number): number {
  return Math.round(ns / 1_000_000)
}

/** Convert milliseconds to display string H:MM:SS */
export function msToTimecode(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000)
  const h = Math.floor(totalSeconds / 3600)
  const m = Math.floor((totalSeconds % 3600) / 60)
  const s = totalSeconds % 60
  return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}
