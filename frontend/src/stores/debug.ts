import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface DebugLogEntry {
  time: string
  level: 'info' | 'warn' | 'error'
  message: string
}

export const useDebugStore = defineStore('debug', () => {
  const visible = ref(false)
  const logs = ref<DebugLogEntry[]>([])
  const maxLogs = 200

  function log(level: DebugLogEntry['level'], message: string): void {
    const entry: DebugLogEntry = {
      time: new Date().toISOString().slice(11, 23),
      level,
      message,
    }
    logs.value.push(entry)
    if (logs.value.length > maxLogs) {
      logs.value.splice(0, logs.value.length - maxLogs)
    }

    // Also console log
    const consoleFn = level === 'error' ? console.error : level === 'warn' ? console.warn : console.log
    consoleFn(`[${entry.time}] ${message}`)
  }

  function info(message: string): void { log('info', message) }
  function warn(message: string): void { log('warn', message) }
  function error(message: string): void { log('error', message) }
  function toggle(): void { visible.value = !visible.value }
  function clear(): void { logs.value = [] }

  return { visible, logs, info, warn, error, toggle, clear }
})
