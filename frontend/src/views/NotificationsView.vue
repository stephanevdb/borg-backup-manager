<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api/client'
import Modal from '../components/Modal.vue'

const channels = ref<any[]>([])
const showModal = ref(false)
const editing = ref<any>(null)
const testStatus = ref('')
const form = ref<any>({
  name: '', type: 'smtp', enabled: true, on_success: true, on_failure: true, on_storage_warning: false, config: {},
})

async function load() {
  const data = await api<{ channels: any[] }>('/notification-channels')
  channels.value = data.channels
}

function openCreate() {
  editing.value = null
  form.value = { name: '', type: 'smtp', enabled: true, on_success: true, on_failure: true, on_storage_warning: false, config: {} }
  showModal.value = true
}

function openEdit(c: any) {
  editing.value = c
  form.value = { ...c, config: { ...c.config } }
  showModal.value = true
}

function applyWebhookPreset(preset: string) {
  if (preset === 'teams') form.value.config = { preset: 'teams', url: form.value.config.url || '' }
  if (preset === 'discord') form.value.config = { preset: 'discord', url: form.value.config.url || '' }
}

async function save() {
  const body = JSON.stringify(form.value)
  if (editing.value) {
    await api(`/notification-channels/${editing.value.id}`, { method: 'PUT', body })
  } else {
    await api('/notification-channels', { method: 'POST', body })
  }
  showModal.value = false
  await load()
}

async function test(id: number) {
  testStatus.value = 'Sending...'
  const res = await api<{ success: boolean }>(`/notification-channels/${id}/test`, { method: 'POST' })
  testStatus.value = res.success ? 'Test sent successfully' : 'Test failed'
}

async function remove(id: number) {
  if (!confirm('Delete channel?')) return
  await api(`/notification-channels/${id}`, { method: 'DELETE' })
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-content">
    <div class="page-header">
      <h1>Notifications</h1>
      <button class="btn btn-primary" @click="openCreate">Add Channel</button>
    </div>
    <p v-if="testStatus" class="text-muted">{{ testStatus }}</p>

    <div class="data-table-wrapper">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Type</th><th>Enabled</th><th>Events</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="c in channels" :key="c.id">
            <td>{{ c.name }}</td>
            <td><span class="type-badge type-local">{{ c.type }}</span></td>
            <td>{{ c.enabled ? 'Yes' : 'No' }}</td>
            <td>
              <span v-if="c.on_success">success </span>
              <span v-if="c.on_failure">failure </span>
              <span v-if="c.on_storage_warning">storage</span>
            </td>
            <td class="actions-cell">
              <button class="btn btn-sm btn-outline" @click="openEdit(c)">Edit</button>
              <button class="btn btn-sm btn-outline" @click="test(c.id)">Test</button>
              <button class="btn btn-sm btn-danger" @click="remove(c.id)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Modal v-model="showModal" :title="editing ? 'Edit Channel' : 'Add Channel'" wide @save="save">
      <div class="form-group"><label>Name</label><input v-model="form.name" type="text" class="text-input" /></div>
      <div class="form-group">
        <label>Type</label>
        <select v-model="form.type" class="select-input"><option value="smtp">SMTP</option><option value="webhook">Webhook</option></select>
      </div>
      <div class="form-group"><label class="checkbox-label"><input v-model="form.enabled" type="checkbox" /> Enabled</label></div>
      <div class="form-group">
        <label class="checkbox-label"><input v-model="form.on_success" type="checkbox" /> On success</label>
        <label class="checkbox-label"><input v-model="form.on_failure" type="checkbox" /> On failure</label>
        <label class="checkbox-label"><input v-model="form.on_storage_warning" type="checkbox" /> Storage warning</label>
      </div>
      <template v-if="form.type === 'smtp'">
        <div class="form-group"><label>Host</label><input v-model="form.config.host" type="text" class="text-input" /></div>
        <div class="form-group"><label>Port</label><input v-model="form.config.port" type="text" class="text-input" placeholder="587" /></div>
        <div class="form-group"><label>From</label><input v-model="form.config.from" type="text" class="text-input" /></div>
        <div class="form-group"><label>To</label><input v-model="form.config.to" type="text" class="text-input" /></div>
      </template>
      <template v-else>
        <div class="form-group">
          <label>Preset</label>
          <button type="button" class="btn btn-outline btn-sm" @click="applyWebhookPreset('teams')">Teams</button>
          <button type="button" class="btn btn-outline btn-sm" @click="applyWebhookPreset('discord')">Discord</button>
        </div>
        <div class="form-group"><label>Webhook URL</label><input v-model="form.config.url" type="text" class="text-input" /></div>
      </template>
    </Modal>
  </div>
</template>
