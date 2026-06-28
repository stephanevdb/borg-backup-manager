<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '../api/client'
import { typeBadgeClass } from '../composables/useBadges'
import Modal from '../components/Modal.vue'
import FolderBrowser from '../components/FolderBrowser.vue'
import HostKeyModal from '../components/HostKeyModal.vue'

const destinations = ref<any[]>([])
const jobs = ref<any[]>([])
const storage = ref<Record<string, any>>({})
const sshKeys = ref<any[]>([])
const search = ref('')
const showModal = ref(false)
const showBrowse = ref(false)
const showHostKey = ref(false)
const sshHost = ref('')
const maintenanceJobId = ref<number | null>(null)
const editing = ref<any>(null)
const form = ref<any>({
  name: '', type: 'local', repo_url: '', passphrase_env: '', notes: '',
  ssh_key_path: '', ssh_host_key_checking: false,
  s3_bucket: '', s3_endpoint: '', s3_region: '', s3_access_key: '', s3_secret_key: '',
})

const filtered = computed(() =>
  destinations.value.filter((d) => d.name.toLowerCase().includes(search.value.toLowerCase()))
)

function storageLabel(id: number) {
  const st = storage.value[String(id)]
  if (!st || st.status === 'unavailable') return '—'
  if (st.percent_used != null) return `${st.percent_used.toFixed(1)}% free`
  return st.status
}

function jobsForDest(destId: number) {
  return jobs.value.filter(j => j.destination_id === destId)
}

async function load() {
  const [d, j, k, st] = await Promise.all([
    api<{ destinations: any[] }>('/destinations'),
    api<{ jobs: any[] }>('/jobs'),
    api<{ keys: any[] }>('/ssh/keys'),
    api<{ storage: Record<string, any> }>('/destinations/storage'),
  ])
  destinations.value = d.destinations
  jobs.value = j.jobs
  sshKeys.value = k.keys
  storage.value = st.storage || {}
}

function openCreate() {
  editing.value = null
  form.value = {
    name: '', type: 'local', repo_url: '', passphrase_env: '', notes: '',
    ssh_key_path: sshKeys.value[0]?.name || '', ssh_host_key_checking: false,
    s3_bucket: '', s3_endpoint: '', s3_region: '', s3_access_key: '', s3_secret_key: '',
  }
  showModal.value = true
}

function openEdit(d: any) {
  editing.value = d
  form.value = { ...d, s3_access_key: '', s3_secret_key: '' }
  showModal.value = true
}

async function save() {
  const body = { ...form.value }
  if (editing.value) {
    await api(`/destinations/${editing.value.id}`, { method: 'PUT', body: JSON.stringify(body) })
  } else {
    await api('/destinations', { method: 'POST', body: JSON.stringify(body) })
  }
  showModal.value = false
  await load()
}

async function remove(id: number) {
  if (!confirm('Delete this destination?')) return
  await api(`/destinations/${id}`, { method: 'DELETE' })
  await load()
}

function pickJobId(destId: number) {
  const list = jobsForDest(destId)
  return list[0]?.id || maintenanceJobId.value
}

async function breakLock(d: any) {
  const jobId = pickJobId(d.id)
  if (!jobId) return alert('No job uses this destination')
  await api(`/destinations/${d.id}/break-lock?job_id=${jobId}`, { method: 'POST' })
  alert('Lock broken')
}

async function compact(d: any) {
  const jobId = pickJobId(d.id)
  if (!jobId) return alert('No job uses this destination')
  await api(`/destinations/${d.id}/compact?job_id=${jobId}`, { method: 'POST' })
  alert('Compact started')
}

function onBrowseSelect(path: string) {
  form.value.repo_url = path
}

async function testSSH() {
  const url = form.value.repo_url || ''
  if (!url.startsWith('ssh://')) return
  sshHost.value = url.slice(6).split('/')[0]
  showHostKey.value = true
}

onMounted(load)
</script>

<template>
  <div class="page-content">
    <div class="page-header">
      <h1>Destinations</h1>
      <div class="page-actions">
        <div class="search-box"><input v-model="search" type="text" class="text-input" placeholder="Search destinations..." /></div>
        <button class="btn btn-primary" @click="openCreate">Add Destination</button>
      </div>
    </div>

    <div class="data-table-wrapper">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Type</th><th>Repository</th><th>Storage</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="d in filtered" :key="d.id">
            <td>{{ d.name }}</td>
            <td><span class="type-badge" :class="typeBadgeClass(d.type)">{{ d.type }}</span></td>
            <td>{{ d.repo_url || d.s3_bucket || '—' }}</td>
            <td>{{ storageLabel(d.id) }}</td>
            <td class="actions-cell">
              <button class="btn btn-sm btn-outline" @click="openEdit(d)">Edit</button>
              <button class="btn btn-sm btn-outline" @click="breakLock(d)">Break Lock</button>
              <button class="btn btn-sm btn-outline" @click="compact(d)">Compact</button>
              <button class="btn btn-sm btn-danger" @click="remove(d.id)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Modal v-model="showModal" :title="editing ? 'Edit Destination' : 'Add Destination'" wide @save="save">
      <div class="form-group"><label>Name</label><input v-model="form.name" type="text" class="text-input" /></div>
      <div class="form-group">
        <label>Type</label>
        <select v-model="form.type" class="select-input">
          <option value="local">Local</option><option value="ssh">SSH</option><option value="s3">S3</option>
        </select>
      </div>
      <div class="form-group">
        <label>Repository URL / Path</label>
        <div class="path-input-group">
          <input v-model="form.repo_url" type="text" class="text-input" />
          <button v-if="form.type === 'local'" type="button" class="btn btn-outline btn-browse" @click="showBrowse = true">Browse</button>
        </div>
      </div>
      <div v-if="form.type === 'ssh'" class="form-group">
        <label>SSH Key</label>
        <select v-model="form.ssh_key_path" class="select-input">
          <option v-for="k in sshKeys" :key="k.id" :value="k.name">{{ k.name }}</option>
        </select>
        <button type="button" class="btn btn-outline btn-sm" @click="testSSH">Test / confirm host key</button>
      </div>
      <template v-if="form.type === 's3'">
        <div class="form-group"><label>S3 Bucket</label><input v-model="form.s3_bucket" type="text" class="text-input" /></div>
        <div class="form-group"><label>Endpoint</label><input v-model="form.s3_endpoint" type="text" class="text-input" /></div>
        <div class="form-group"><label>Region</label><input v-model="form.s3_region" type="text" class="text-input" /></div>
        <div class="form-group"><label>Access Key</label><input v-model="form.s3_access_key" type="password" class="text-input" /></div>
        <div class="form-group"><label>Secret Key</label><input v-model="form.s3_secret_key" type="password" class="text-input" /></div>
      </template>
      <div class="form-group"><label>Passphrase env var</label><input v-model="form.passphrase_env" type="text" class="text-input" /></div>
      <div class="form-group"><label>Notes</label><input v-model="form.notes" type="text" class="text-input" /></div>
    </Modal>

    <FolderBrowser v-model="showBrowse" :path="form.repo_url" @select="onBrowseSelect" />
    <HostKeyModal v-model="showHostKey" :host="sshHost" />
  </div>
</template>
