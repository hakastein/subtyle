import { createApp } from 'vue'
import { createPinia } from 'pinia'
import i18n from './i18n'
import App from './App.vue'
import { useDebugStore } from './stores/debug'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(i18n)

// Global Vue error handler
app.config.errorHandler = (err, instance, info) => {
  const msg = err instanceof Error ? `${err.message}\n${err.stack}` : String(err)
  console.error('[Vue error]', err, info)
  try {
    useDebugStore().error(`[Vue] ${info}: ${msg}`)
  } catch {
    // store may not be ready yet
  }
}

// Uncaught JS errors
window.addEventListener('error', (event) => {
  const msg = event.error instanceof Error
    ? `${event.error.message}\n${event.error.stack}`
    : event.message
  console.error('[window.error]', event.error)
  try {
    useDebugStore().error(`[window] ${msg}`)
  } catch {
    // ignore
  }
})

// Unhandled promise rejections
window.addEventListener('unhandledrejection', (event) => {
  const msg = event.reason instanceof Error
    ? `${event.reason.message}\n${event.reason.stack}`
    : String(event.reason)
  console.error('[unhandledrejection]', event.reason)
  try {
    useDebugStore().error(`[promise] ${msg}`)
  } catch {
    // ignore
  }
})

// Backend panics surfaced via Wails event
if (typeof window !== 'undefined' && (window as unknown as { runtime?: { EventsOn: (event: string, cb: (msg: unknown) => void) => void } }).runtime) {
  (window as unknown as { runtime: { EventsOn: (event: string, cb: (msg: unknown) => void) => void } }).runtime.EventsOn('app:error', (msg: unknown) => {
    try {
      useDebugStore().error(`[backend panic] ${msg}`)
    } catch {
      // ignore
    }
  })
}

app.mount('#app')
