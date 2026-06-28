<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '../api/client'
import { typeBadgeClass } from '../composables/useBadges'
import Modal from '../components/Modal.vue'
import FolderBrowser from '../components/FolderBrowser.vue'
import HostKeyModal from '../components/HostKeyModal.vue'

const sources = ref<any[]>([])
const sshKeys = ref<any[]>([])
const search = ref('')
const showModal = ref(false)
const showBrowse = ref(false)
const showHostKey = ref(false)
const sshHost = ref('')
const testResult = ref('')
const editing = ref<any>(null)
const form = ref<any>({
  name: '', type: 'local', path: '', excludes: '', notes: '',
  ssh_key_path: '', ssh_host_key_checking: false,
  s3_bucket: '', s3_prefix: '', s3_endpoint: '', s3_region: '', s3_provider: '',
  s3_access_key: '', s3_secret_key: '',
})

const filtered = computed(() =>
  sources.value.filter((s) => s.name.toLowerCase().includes(search.value.toLowerCase()))
)

async function load() {
  const [s, k] = await Promise.all([
    api<{ sources: any[] }>('/sources'),
    api<{ keys: any[] }>('/ssh/keys'),
  ])
  sources.value = s.sources
  sshKeys.value = k.keys
}

function openCreate() {
  editing.value = null
  form.value = {
    name: '', type: 'local', path: '', excludes: '', notes: '',
    ssh_key_path: sshKeys.value[0]?.name || '', ssh_host_key_checking: false,
    s3_bucket: '', s3_prefix: '', s3_endpoint: '', s3_region: '', s3_provider: '',
    s3_access_key: '', s3_secret_key: '',
  }
  showModal.value = true
}

function openEdit(s: any) {
  editing.value = s
  form.value = { ...s, s3_access_key: '', s3_secret_key: '' }
  showModal.value = true
}

async function save() {
  const body = { ...form.value }
  if (editing.value) {
    await api(`/sources/${editing.value.id}`, { method: 'PUT', body: JSON.stringify(body) })
  } else {
    await api('/sources', { method: 'POST', body: JSON.stringify(body) })
  }
  showModal.value = false
  await load()
}

async function remove(id: number) {
  if (!confirm('Delete this source?')) return
  await api(`/sources/${id}`, { method: 'DELETE' })
  await load()
}

function onBrowseSelect(path: string) {
  form.value.path = path
}

async function testSSH() {
  if (!form.value.path?.startsWith('ssh://')) return
  const host = form.value.path.slice(6).split('/')[0]
  sshHost.value = host
  const res = await api<{ success: boolean; output: string }>('/ssh/test-connection', {
    method: 'POST', body: JSON.stringify({ host }),
  })
  testResult.value = res.success ? 'Connection OK' : (res.output || 'Failed')
  if (!res.success) showHostKey.value = true
}

onMounted(load)
</script>

<template>
  <div class="page-content">
    <div class="page-header">
      <h1>Sources</h1>
      <div class="page-actions">
        <div class="search-box"><input v-model="search" type="text" class="text-input" placeholder="Search sources..." /></div>
        <button class="btn btn-primary" @click="openCreate">Add Source</button>
      </div>
    </div>

    <div class="data-table-wrapper">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Type</th><th>Path / Bucket</th><th class="hide-mobile">Notes</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="s in filtered" :key="s.id">
            <td>{{ s.name }}</td>
            <td><span class="type-badge" :class="typeBadgeClass(s.type)">{{ s.type }}</span></td>
            <td>{{ s.path || s.s3_bucket || '—' }}</td>
            <td class="hide-mobile">{{ s.notes || '—' }}</td>
            <td class="actions-cell">
              <button class="btn btn-sm btn-outline" @click="openEdit(s)">Edit</button>
              <button class="btn btn-sm btn-danger" @click="remove(s.id)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Modal v-model="showModal" :title="editing ? 'Edit Source' : 'Add Source'" wide @save="save">
      <div class="form-group"><label>Name</label><input v-model="form.name" type="text" class="text-input" /></div>
      <div class="form-group">
        <label>Type</label>
        <select v-model="form.type" class="select-input">
          <option value="local">Local</option><option value="ssh">SSH</option><option value="s3">S3</option>
        </select>
      </div>
      <div v-if="form.type === 'local' || form.type === 'ssh'" class="form-group">
        <label>Path</label>
        <div class="path-input-group">
          <input v-model="form.path" type="text" class="text-input" :placeholder="form.type === 'ssh' ? 'ssh://user@host/path' : '/data'" />
          <button v-if="form.type === 'local'" type="button" class="btn btn-outline btn-browse" @click="showBrowse = true">Browse</button>
        </div>
      </div>
      <div v-if="form.type === 'ssh'" class="form-group">
        <label>SSH Key</label>
        <select v-model="form.ssh_key_path" class="select-input">
          <option v-for="k in sshKeys" :key="k.id" :value="k.name">{{ k.name }}</option>
        </select>
      </div>
      <div v-if="form.type === 'ssh'" class="form-group">
        <label class="checkbox-label"><input v-model="form.ssh_host_key_checking" type="checkbox" /> Strict host key checking</label>
        <button type="button" class="btn btn-outline btn-sm" @click="testSSH">Test connection</button>
        <span v-if="testResult" class="text-muted">{{ testResult }}</span>
      </div>
      <template v-if="form.type === 's3'">
        <div class="form-group"><label>S3 Bucket</label><input v-model="form.s3_bucket" type="text" class="text-input" /></div>
        <div class="form-group"><label>Prefix</label><input v-model="form.s3_prefix" type="text" class="text-input" /></div>
        <div class="form-group"><label>Endpoint</label><input v-model="form.s3_endpoint" type="text" class="text-input" placeholder="https://..." /></div>
        <div class="form-group"><label>Region</label><input v-model="form.s3_region" type="text" class="text-input" /></div>
        <div class="form-group"><label>Access Key</label><input v-model="form.s3_access_key" type="password" class="text-input" autocomplete="off" /></div>
        <div class="form-group"><label>Secret Key</label><input v-model="form.s3_secret_key" type="password" class="text-input" autocomplete="off" /></div>
      </template>
      <div class="form-group"><label>Exclude patterns</label><textarea v-model="form.excludes" class="text-input" rows="3" /></div>
      <div class="form-group"><label>Notes</label><input v-model="form.notes" type="text" class="text-input" /></div>
    </Modal>

    <FolderBrowser v-model="showBrowse" :path="form.path" @select="onBrowseSelect" />
    <HostKeyModal v-model="showHostKey" :host="sshHost" />
  </div>
</template>
