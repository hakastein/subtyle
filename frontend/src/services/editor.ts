import type { FrameResult, SubtitleStyle } from './types'

export async function generatePreviewFrame(
  fileId: string,
  videoPath: string,
  styles: SubtitleStyle[],
  atMs: number,
  widthPx: number,
): Promise<FrameResult> {
  return window.go.main.App.GeneratePreviewFrame(fileId, videoPath, styles, atMs, widthPx)
}

export async function getVideoDuration(videoPath: string): Promise<number> { return window.go.main.App.GetVideoDuration(videoPath) }
