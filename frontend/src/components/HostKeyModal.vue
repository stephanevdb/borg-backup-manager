<script setup lang="ts">
import { ref } from 'vue'
import { api } from '../api/client'

const props = defineProps<{ modelValue: boolean; host: string }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; confirmed: [] }>()

const keys = ref('')
const loading = ref(false)

async function scan() {
  loading.value = true
  try {
    const data = await api<{ keys: string }>(`/ssh/scan-host?host=${encodeURIComponent(props.host)}`)
    keys.value = data.keys || ''
  } finally {
    loading.value = false
  }
}

async function confirm() {
  const line = keys.value.trim().split('\n')[0]
  if (!line) return
  await api('/ssh/confirm-key', { method: 'POST', body: JSON.stringify({ host: props.host, key_line: line }) })
  emit('confirmed')
  emit('update:modelValue', false)
}

function close() {
  emit('update:modelValue', false)
}
</script>

<template>
  <div v-if="modelValue" class="modal">
    <div class="modal-backdrop" @click="close" />
    <div class="modal-content">
      <div class="modal-header">
        <h3>Confirm Host Key</h3>
        <button type="button" class="modal-close" @click="close">&times;</button>
      </div>
      <div class="modal-body">
        <p class="text-muted">Host: {{ host }}</p>
        <button class="btn btn-outline btn-sm" :disabled="loading" @click="scan">Scan host key</button>
        <pre v-if="keys" class="mono-text" style="margin-top: 1rem;">{{ keys }}</pre>
      </div>
      <div class="modal-footer">
        <button type="button" class="btn btn-outline" @click="close">Cancel</button>
        <button type="button" class="btn btn-primary" :disabled="!keys" @click="confirm">Trust key</button>
      </div>
    </div>
  </div>
</template>
