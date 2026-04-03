<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { NModal, NCard, NButton, NSpace, NText } from 'naive-ui'
import type { ProjectState } from '@/services/types'

interface Props {
  show: boolean
  state: ProjectState | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'restore'): void
  (e: 'discard'): void
}>()

const { t } = useI18n()

const formattedDate = computed(() => {
  if (!props.state?.savedAt) return ''
  try {
    return new Date(props.state.savedAt).toLocaleString()
  } catch {
    return props.state.savedAt
  }
})
</script>

<template>
  <NModal :show="show" :mask-closable="false">
    <NCard
      :title="t('project.restoreTitle')"
      style="max-width: 420px; width: 90vw"
      :bordered="true"
    >
      <NText>
        {{ t('project.restoreMessage', { date: formattedDate }) }}
      </NText>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="emit('discard')">
            {{ t('project.restoreNo') }}
          </NButton>
          <NButton type="primary" @click="emit('restore')">
            {{ t('project.restoreYes') }}
          </NButton>
        </NSpace>
      </template>
    </NCard>
  </NModal>
</template>
