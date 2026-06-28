<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { api } from '../api/client'

const props = defineProps<{ modelValue: boolean; restoreId: string | null }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; done: [] }>()

const logs = ref<string[]>([])
const status = ref('running')
let es: EventSource | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null

function closeStream() {
  es?.close()
  es = null
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(
  () => [props.modelValue, props.restoreId] as const,
  ([open, id]) => {
    closeStream()
    if (!open || !id) return
    logs.value = []
    status.value = 'running'
    es = new EventSource(`/api/v1/restores/${id}/stream`, { withCredentials: true })
    es.addEventListener('log', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        if (data?.line) logs.value.push(data.line)
      } catch { /* ignore */ }
    })
    pollTimer = setInterval(async () => {
      try {
        const data = await api<{ restore: { status: string; done: boolean } }>(`/restores/${id}/status`)
        status.value = data.restore?.status || status.value
        if (data.restore?.done) {
          closeStream()
          emit('done')
        }
      } catch { /* ignore */ }
    }, 2000)
  },
  { immediate: true },
)

onUnmounted(closeStream)

function close() {
  emit('update:modelValue', false)
}
</script>

<template>
  <div v-if="modelValue" class="modal">
    <div class="modal-backdrop" @click="close" />
    <div class="modal-content modal-wide">
      <div class="modal-header">
        <h3>Restore Output</h3>
        <span class="status-badge" :class="status === 'success' ? 'status-success' : status === 'failed' ? 'status-error' : 'status-running'">{{ status }}</span>
        <button type="button" class="modal-close" @click="close">&times;</button>
      </div>
      <div class="modal-body">
        <div class="run-output-wrapper">
          <pre class="run-output-log">{{ logs.join('\n') }}</pre>
        </div>
      </div>
      <div class="modal-footer">
        <button type="button" class="btn btn-outline" @click="close">Close</button>
      </div>
    </div>
  </div>
</template>
