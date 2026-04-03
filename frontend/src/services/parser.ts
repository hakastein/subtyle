import type { SubtitleFile, SubtitleStyle } from './types'
export async function parseFile(path: string): Promise<SubtitleFile> { return window.go.main.App.ParseFile(path) }
export async function extractTrack(videoPath: string, trackIndex: number): Promise<SubtitleFile> { return window.go.main.App.ExtractTrack(videoPath, trackIndex) }
export async function saveFile(fileId: string, styles: SubtitleStyle[]): Promise<void> { return window.go.main.App.SaveFile(fileId, styles) }
export async function saveAll(fileStyles: Record<string, SubtitleStyle[]>): Promise<void> { return window.go.main.App.SaveAll(fileStyles) }
