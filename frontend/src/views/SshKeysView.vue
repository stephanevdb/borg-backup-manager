<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api/client'

const keys = ref<any[]>([])
const name = ref('')
const newKey = ref<{ public_key?: string; install_command?: string } | null>(null)

async function load() {
  const data = await api<{ keys: any[] }>('/ssh/keys')
  keys.value = data.keys
}

async function generate() {
  const data = await api<any>('/ssh/generate', { method: 'POST', body: JSON.stringify({ name: name.value }) })
  newKey.value = { public_key: data.key?.public_key, install_command: data.install_command }
  name.value = ''
  await load()
}

async function remove(id: number) {
  if (!confirm('Delete this key?')) return
  await api(`/ssh/keys/${id}`, { method: 'DELETE' })
  await load()
}

function copy(text: string) {
  navigator.clipboard.writeText(text)
}

onMounted(load)
</script>

<template>
  <div class="page-content">
    <div class="page-header">
      <h1>SSH Keys</h1>
    </div>

    <div class="filter-toolbar">
      <div class="form-group">
        <input v-model="name" type="text" class="text-input" placeholder="Key name" />
      </div>
      <button class="btn btn-primary" :disabled="!name" @click="generate">Generate Key</button>
    </div>

    <div v-if="newKey" class="alert alert-info">
      <p>Copy public key:</p>
      <pre class="mono-text">{{ newKey.public_key }}</pre>
      <button class="btn btn-sm btn-outline" @click="copy(newKey.install_command || '')">Copy install command</button>
    </div>

    <div class="data-table-wrapper">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Type</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id">
            <td>{{ k.name }}</td>
            <td>{{ k.key_type }}</td>
            <td class="actions-cell">
              <button class="btn btn-sm btn-outline" @click="copy(k.public_key)">Copy public key</button>
              <button class="btn btn-sm btn-danger" @click="remove(k.id)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
