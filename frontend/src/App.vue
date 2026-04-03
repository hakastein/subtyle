<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NConfigProvider, NMessageProvider, NDialogProvider } from 'naive-ui'
import { useProjectStore } from '@/stores/project'
import * as projectService from '@/services/project'
import { setLocale, getSavedLocale } from '@/i18n'
import type { ProjectState } from '@/services/types'
import MainView from '@/views/MainView.vue'
import RestoreDialog from '@/views/RestoreDialog.vue'

const projectStore = useProjectStore()

const initialized = ref(false)
const showRestoreDialog = ref(false)
const autosaveState = ref<ProjectState | null>(null)

onMounted(async () => {
  // Detect locale
  const savedLocale = getSavedLocale()
  if (savedLocale) {
    setLocale(savedLocale)
  } else {
    try {
      const backendLocale = await projectService.getLocale()
      setLocale(backendLocale)
    } catch {
      // fallback to default 'en'
    }
  }

  // Check for autosave
  try {
    const state = await projectService.checkAutosave()
    if (state && state.dirty) {
      autosaveState.value = state
      showRestoreDialog.value = true
      return
    }
  } catch {
    // no autosave
  }

  initialized.value = true
})

async function handleRestore() {
  if (autosaveState.value) {
    projectStore.restoreFromAutosave(autosaveState.value)
  }
  showRestoreDialog.value = false
  initialized.value = true
}

async function handleDiscard() {
  try {
    await projectService.deleteAutosave()
  } catch {
    // ignore
  }
  showRestoreDialog.value = false
  initialized.value = true
}
</script>

<template>
  <NConfigProvider>
    <NMessageProvider>
      <NDialogProvider>
        <RestoreDialog
          :show="showRestoreDialog"
          :state="autosaveState"
          @restore="handleRestore"
          @discard="handleDiscard"
        />
        <MainView v-if="initialized" />
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>

<style>
* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

html, body, #app {
  height: 100%;
  overflow: hidden;
}
</style>
