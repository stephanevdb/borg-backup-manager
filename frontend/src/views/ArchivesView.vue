<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '../api/client'
import RestoreOutputModal from '../components/RestoreOutputModal.vue'
import Modal from '../components/Modal.vue'

const jobs = ref<any[]>([])
const destinations = ref<any[]>([])
const jobId = ref<number | null>(null)
const destId = ref<number | null>(null)
const archives = ref<any[]>([])
const selected = ref<string[]>([])
const browseArchive = ref<string | null>(null)
const browsePath = ref('')
const browseFiles = ref<{ name: string; type: string }[]>([])
const showRestoreConfirm = ref(false)
const restoreTarget = ref<string | null>(null)
const restoreConfirmText = ref('')
const restoreId = ref<string | null>(null)
const showRestoreLog = ref(false)

const breadcrumbs = computed(() => {
  const parts = browsePath.value.split('/').filter(Boolean)
  const crumbs = [{ label: browseArchive.value || '', path: '' }]
  let acc = ''
  for (const p of parts) {
    acc += (acc ? '/' : '') + p
    crumbs.push({ label: p, path: acc })
  }
  return crumbs
})

async function loadMeta() {
  const [j, d] = await Promise.all([
    api<{ jobs: any[] }>('/jobs'),
    api<{ destinations: any[] }>('/destinations'),
  ])
  jobs.value = j.jobs
  destinations.value = d.destinations
  if (j.jobs.length) jobId.value = j.jobs[0].id
  if (d.destinations.length) destId.value = d.destinations[0].id
}

async function loadArchives() {
  if (!jobId.value || !destId.value) return
  const data = await api<{ archives: any[] }>(`/archives?job_id=${jobId.value}&destination_id=${destId.value}`)
  archives.value = data.archives
}

async function browse(name: string, path = '') {
  browseArchive.value = name
  browsePath.value = path
  const data = await api<{ files: { name: string; type: string }[] }>(
    `/archives/${encodeURIComponent(name)}/files?job_id=${jobId.value}&destination_id=${destId.value}&path=${encodeURIComponent(path)}`,
  )
  browseFiles.value = data.files || []
}

function downloadFile(name: string) {
  if (!browseArchive.value || !jobId.value || !destId.value) return
  const full = [browsePath.value, name].filter(Boolean).join('/')
  window.open(`/api/v1/archives/${encodeURIComponent(browseArchive.value)}/download?job_id=${jobId.value}&destination_id=${destId.value}&path=${encodeURIComponent(full)}`)
}

function promptRestore(name: string) {
  restoreTarget.value = name
  showRestoreConfirm.value = true
}

async function confirmRestore() {
  if (!restoreTarget.value || !jobId.value || !destId.value) return
  if (restoreConfirmText.value !== 'CONFIRM') return
  showRestoreConfirm.value = false
  restoreConfirmText.value = ''
  const res = await api<{ restore_id: string }>(
    `/archives/${encodeURIComponent(restoreTarget.value)}/restore?job_id=${jobId.value}&destination_id=${destId.value}`,
    { method: 'POST' },
  )
  restoreId.value = res.restore_id
  showRestoreLog.value = true
}

async function deleteSelected() {
  if (!selected.value.length || !confirm('Delete selected archives?')) return
  await api('/archives/delete-batch', {
    method: 'POST',
    body: JSON.stringify({ job_id: jobId.value, destination_id: destId.value, names: selected.value }),
  })
  selected.value = []
  await loadArchives()
}

onMounted(async () => { await loadMeta(); await loadArchives() })
</script>

<template>
  <div class="page-content">
    <div class="page-header"><h1>Backups</h1></div>

    <div class="archive-controls filter-toolbar">
      <div class="form-group"><label>Destination</label><select v-model="destId" class="select-input" @change="loadArchives"><option v-for="d in destinations" :key="d.id" :value="d.id">{{ d.name }}</option></select></div>
      <div class="form-group"><label>Job</label><select v-model="jobId" class="select-input" @change="loadArchives"><option v-for="j in jobs" :key="j.id" :value="j.id">{{ j.name }}</option></select></div>
      <button class="btn btn-danger" :disabled="!selected.length" @click="deleteSelected">Delete Selected</button>
    </div>

    <div class="data-table-wrapper">
      <table class="data-table">
        <thead><tr><th></th><th>Archive</th><th>Time</th><th>Size</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="a in archives" :key="a.name">
            <td><input type="checkbox" :value="a.name" v-model="selected" /></td>
            <td class="mono-text">{{ a.name }}</td>
            <td>{{ a.time || '—' }}</td>
            <td>{{ a.size_bytes ? (a.size_bytes / 1024 / 1024).toFixed(1) + ' MB' : '—' }}</td>
            <td class="actions-cell">
              <button class="btn btn-sm btn-outline" @click="browse(a.name)">Browse</button>
              <button class="btn btn-sm btn-outline" @click="promptRestore(a.name)">Restore</button>
              <a class="btn btn-sm btn-outline" :href="`/api/v1/archives/${encodeURIComponent(a.name)}/download-tar?job_id=${jobId}&destination_id=${destId}`">Download tar</a>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="browseArchive" class="dash-card" style="margin-top: 1rem;">
      <div class="archive-browser-header">
        <nav class="archive-breadcrumb">
          <button v-for="(c, i) in breadcrumbs" :key="i" class="btn btn-sm btn-outline" @click="browse(browseArchive!, c.path)">{{ c.label || 'root' }}</button>
        </nav>
      </div>
      <table class="data-table">
        <tbody>
          <tr v-if="browsePath" @click="browse(browseArchive!, browsePath.split('/').slice(0,-1).join('/'))"><td>..</td><td></td></tr>
          <tr v-for="f in browseFiles" :key="f.name">
            <td>{{ f.name }}</td>
            <td class="actions-cell">
              <button v-if="f.type !== 'd'" class="btn btn-sm btn-outline" @click="downloadFile(f.name)">Download</button>
              <button v-else class="btn btn-sm btn-outline" @click="browse(browseArchive!, [browsePath, f.name].filter(Boolean).join('/'))">Open</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Modal v-model="showRestoreConfirm" title="Confirm Restore" @save="confirmRestore">
      <p>This will extract archive <strong>{{ restoreTarget }}</strong> to the source path.</p>
      <div class="form-group">
        <label>Type CONFIRM to proceed</label>
        <input v-model="restoreConfirmText" type="text" class="text-input" placeholder="CONFIRM" />
      </div>
    </Modal>

    <RestoreOutputModal v-model="showRestoreLog" :restore-id="restoreId" />
  </div>
</template>
