import type { FolderScanResult } from './types'
export async function openFolder(): Promise<string> { return window.go.main.App.OpenFolder() }
export async function scanFolder(dir: string): Promise<FolderScanResult> { return window.go.main.App.ScanFolder(dir) }
