export interface FFmpegProgress {
  received: number
  total: number
}

declare const window: Window & {
  runtime: {
    EventsOn(eventName: string, callback: (...args: unknown[]) => void): void
  }
}

export function onFFmpegReady(callback: () => void): void {
  window.runtime.EventsOn('ffmpeg:ready', callback)
}
export function onFFmpegDownloading(callback: () => void): void {
  window.runtime.EventsOn('ffmpeg:downloading', callback)
}
export function onFFmpegProgress(callback: (received: number, total: number) => void): void {
  window.runtime.EventsOn('ffmpeg:progress', (received: unknown, total: unknown) => {
    callback(received as number, total as number)
  })
}
export function onFFmpegError(callback: (error: string) => void): void {
  window.runtime.EventsOn('ffmpeg:error', (error: unknown) => callback(error as string))
}
