import type { SubtitleFile, SubtitleStyle } from './types'

export interface SaveRequest {
  fileId: string
  videoPath: string
  styles: SubtitleStyle[]
}

export async function parseFile(path: string): Promise<SubtitleFile> { return window.go.main.App.ParseFile(path) }
export async function extractTrack(videoPath: string, trackIndex: number): Promise<SubtitleFile> { return window.go.main.App.ExtractTrack(videoPath, trackIndex) }
export async function saveFile(req: SaveRequest): Promise<string> { return window.go.main.App.SaveFile(req) }
export async function saveAll(requests: SaveRequest[]): Promise<string[]> { return window.go.main.App.SaveAll(requests) }
