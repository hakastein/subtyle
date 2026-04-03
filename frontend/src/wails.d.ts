import type { FolderScanResult, SubtitleFile, SubtitleStyle, FrameResult, ProjectState } from './services/types'

declare global {
  interface Window {
    go: {
      main: {
        App: {
          GetLocale(): Promise<string>
          OpenFolder(): Promise<string>
          ScanFolder(dir: string): Promise<FolderScanResult>
          ParseFile(path: string): Promise<SubtitleFile>
          ExtractTrack(videoPath: string, trackIndex: number): Promise<SubtitleFile>
          GeneratePreviewFrame(fileId: string, videoPath: string, styles: SubtitleStyle[], atMs: number): Promise<FrameResult>
          SaveFile(fileId: string, styles: SubtitleStyle[]): Promise<void>
          SaveAll(fileStyles: Record<string, SubtitleStyle[]>): Promise<void>
          CheckAutosave(): Promise<ProjectState | null>
          RestoreProject(): Promise<ProjectState>
          Autosave(state: ProjectState): Promise<void>
          DeleteAutosave(): Promise<void>
          GetVideoDuration(videoPath: string): Promise<number>
        }
      }
    }
    runtime: {
      EventsOn(eventName: string, callback: (...args: unknown[]) => void): void
      EventsEmit(eventName: string, ...args: unknown[]): void
    }
  }
}

export {}
