<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api/client'
import Modal from '../components/Modal.vue'

const keys = ref<any[]>([])
const showModal = ref(false)
const name = ref('')
const secret = ref('')

async function load() {
  const data = await api<{ keys: any[] }>('/api-keys')
  keys.value = data.keys
}

async function create() {
  const data = await api<any>('/api-keys', { method: 'POST', body: JSON.stringify({ name: name.value }) })
  secret.value = data.secret
  name.value = ''
  await load()
}

async function revoke(id: number) {
  if (!confirm('Revoke this API key?')) return
  await api(`/api-keys/${id}`, { method: 'DELETE' })
  await load()
}

function copySecret() {
  navigator.clipboard.writeText(secret.value)
}

onMounted(load)
</script>

<template>
  <div class="page-content">
    <div class="page-header">
      <h1>API Keys</h1>
      <button class="btn btn-primary" @click="showModal = true">Generate Key</button>
    </div>

    <p class="text-muted">Use the <code>X-API-Key</code> header for programmatic access.</p>

    <div class="data-table-wrapper">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Prefix</th><th>Created</th><th>Last Used</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id">
            <td>{{ k.name }}</td>
            <td class="mono-text">{{ k.prefix }}...</td>
            <td>{{ k.created_at }}</td>
            <td>{{ k.last_used_at || '—' }}</td>
            <td class="actions-cell">
              <button class="btn btn-sm btn-danger" @click="revoke(k.id)">Revoke</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Modal v-model="showModal" title="Generate API Key" @save="create">
      <div class="form-group"><label>Name</label><input v-model="name" type="text" class="text-input" /></div>
      <div v-if="secret" class="alert alert-warning">
        <p>Copy this key now — it won't be shown again:</p>
        <code class="mono-text">{{ secret }}</code>
        <button class="btn btn-sm btn-outline" @click="copySecret">Copy</button>
      </div>
    </Modal>
  </div>
</template>
