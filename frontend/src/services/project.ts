import type { ProjectState } from './types'
export async function getLocale(): Promise<string> { return window.go.main.App.GetLocale() }
export async function checkAutosave(): Promise<ProjectState | null> { return window.go.main.App.CheckAutosave() }
export async function restoreProject(): Promise<ProjectState> { return window.go.main.App.RestoreProject() }
export async function autosave(state: ProjectState): Promise<void> { return window.go.main.App.Autosave(state) }
export async function deleteAutosave(): Promise<void> { return window.go.main.App.DeleteAutosave() }
