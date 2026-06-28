<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { api } from '../api/client'

const props = defineProps<{ modelValue: boolean; runId: number | null; jobId?: number | null }>()
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()

const logs = ref<string[]>([])
const progress = ref<Record<string, unknown>>({})
let es: EventSource | null = null

function closeStream() {
  es?.close()
  es = null
}

watch(
  () => [props.modelValue, props.runId] as const,
  ([open, id]) => {
    closeStream()
    if (!open || !id) return

    logs.value = []
    progress.value = {}
    es = new EventSource(`/api/v1/runs/${id}/stream`, { withCredentials: true })

    const onLog = (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        if (data?.line) logs.value.push(data.line)
      } catch { /* ignore */ }
    }
    const onProgress = (e: MessageEvent) => {
      try {
        progress.value = JSON.parse(e.data)
      } catch { /* ignore */ }
    }

    es.addEventListener('log', onLog)
    es.addEventListener('progress', onProgress)
  },
  { immediate: true },
)

onUnmounted(closeStream)

function close() {
  emit('update:modelValue', false)
}

async function cancelRun() {
  if (props.jobId) {
    await api(`/jobs/${props.jobId}/cancel`, { method: 'POST' })
  }
  close()
}
</script>

<template>
  <div v-if="modelValue" class="modal">
    <div class="modal-backdrop" @click="close" />
    <div class="modal-content modal-wide">
      <div class="modal-header">
        <h3>Backup Run Output</h3>
        <button type="button" class="modal-close" @click="close">&times;</button>
      </div>
      <div class="modal-body">
        <div v-if="progress.percent != null" class="progress-bar-container">
          <div class="progress-fill" :style="{ width: progress.percent + '%' }" />
        </div>
        <div class="run-output-wrapper">
          <pre class="run-output-log">{{ logs.join('\n') }}</pre>
        </div>
      </div>
      <div class="modal-footer">
        <button v-if="jobId" type="button" class="btn btn-danger" @click="cancelRun">Cancel backup</button>
        <button type="button" class="btn btn-outline" @click="close">Close</button>
      </div>
    </div>
  </div>
</template>
