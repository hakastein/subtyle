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
          ExtractTrack(videoPath: string, trackIndex: number, trackTitle: string): Promise<SubtitleFile>
          GeneratePreviewFrame(fileId: string, videoPath: string, styles: SubtitleStyle[], atMs: number, widthPx: number): Promise<FrameResult>
          SaveFile(req: { fileId: string; videoPath: string; styles: SubtitleStyle[] }): Promise<string>
          SaveAll(requests: Array<{ fileId: string; videoPath: string; styles: SubtitleStyle[] }>): Promise<string[]>
          CheckAutosave(): Promise<ProjectState | null>
          RestoreProject(): Promise<ProjectState>
          Autosave(state: ProjectState): Promise<void>
          DeleteAutosave(): Promise<void>
          GetVideoDuration(videoPath: string): Promise<number>
          IsFfmpegReady(): Promise<boolean>
          GetFfmpegDiag(): Promise<{
            path: string
            version: string
            hasSubtitlesFilter: boolean
            hasLibass: boolean
            filters: string
          }>
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
