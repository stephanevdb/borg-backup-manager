<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '../api/client'
import { statusBadgeClass } from '../composables/useBadges'
import { formatDuration, timeAgo } from '../composables/useFormatters'
import Modal from '../components/Modal.vue'
import RunOutputModal from '../components/RunOutputModal.vue'

const jobs = ref<any[]>([])
const sources = ref<any[]>([])
const destinations = ref<any[]>([])
const channels = ref<any[]>([])
const search = ref('')
const showModal = ref(false)
const showRun = ref(false)
const showHistory = ref(false)
const historyRuns = ref<any[]>([])
const historyJob = ref<any>(null)
const activeRunId = ref<number | null>(null)
const activeJobId = ref<number | null>(null)
const editing = ref<any>(null)
const form = ref<any>({
  name: '', source_id: null, destination_id: null, schedule: 'manual',
  compression: 'lz4', retention_daily: 7, retention_weekly: 4, retention_monthly: 6,
  enabled: true, daily_verification_enabled: true, notification_channel_ids: [] as number[],
  ssh_before_command: '', ssh_after_command: '', restore_before_command: '', restore_after_command: '',
})

const filtered = computed(() =>
  jobs.value.filter((j) => j.name.toLowerCase().includes(search.value.toLowerCase()))
)

async function load() {
  const [j, s, d, c] = await Promise.all([
    api<{ jobs: any[] }>('/jobs'),
    api<{ sources: any[] }>('/sources'),
    api<{ destinations: any[] }>('/destinations'),
    api<{ channels: any[] }>('/notification-channels'),
  ])
  jobs.value = j.jobs
  sources.value = s.sources
  destinations.value = d.destinations
  channels.value = c.channels
}

function openCreate() {
  editing.value = null
  form.value = {
    name: '', source_id: sources.value[0]?.id, destination_id: destinations.value[0]?.id,
    schedule: 'manual', compression: 'lz4', retention_daily: 7, retention_weekly: 4, retention_monthly: 6,
    enabled: true, daily_verification_enabled: true, notification_channel_ids: [],
    ssh_before_command: '', ssh_after_command: '', restore_before_command: '', restore_after_command: '',
  }
  showModal.value = true
}

function openEdit(j: any) {
  editing.value = j
  form.value = { ...j, notification_channel_ids: [...(j.notification_channel_ids || [])] }
  showModal.value = true
}

async function save() {
  const body = JSON.stringify(form.value)
  if (editing.value) {
    await api(`/jobs/${editing.value.id}`, { method: 'PUT', body })
  } else {
    await api('/jobs', { method: 'POST', body })
  }
  showModal.value = false
  await load()
}

async function toggle(j: any) {
  await api(`/jobs/${j.id}/toggle`, { method: 'POST' })
  await load()
}

async function runNow(j: any) {
  const res = await api<{ run_id: number }>(`/jobs/${j.id}/run`, { method: 'POST' })
  activeRunId.value = res.run_id
  activeJobId.value = j.id
  showRun.value = true
}

async function openHistory(j: any) {
  historyJob.value = j
  const data = await api<{ runs: any[] }>(`/jobs/${j.id}/runs`)
  historyRuns.value = data.runs
  showHistory.value = true
}

async function remove(id: number) {
  if (!confirm('Delete this job?')) return
  await api(`/jobs/${id}`, { method: 'DELETE' })
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-content">
    <div class="page-header">
      <h1>Backup Jobs</h1>
      <div class="page-actions">
        <div class="search-box"><input v-model="search" type="text" class="text-input" placeholder="Search jobs..." /></div>
        <button class="btn btn-primary" @click="openCreate">Add Job</button>
      </div>
    </div>

    <div class="data-table-wrapper">
      <table class="data-table">
        <thead><tr><th>Name</th><th>Source</th><th>Destination</th><th>Schedule</th><th>Status</th><th>Actions</th></tr></thead>
        <tbody>
          <tr v-for="j in filtered" :key="j.id">
            <td>{{ j.name }}</td>
            <td>{{ j.source_name }}</td>
            <td>{{ j.destination_name }}</td>
            <td>{{ j.schedule }}</td>
            <td><span class="status-badge" :class="statusBadgeClass(j.last_status)">{{ j.last_status || '—' }}</span></td>
            <td class="actions-cell">
              <button class="btn btn-sm btn-primary btn-run" @click="runNow(j)">Run Now</button>
              <button class="btn btn-sm btn-outline" @click="openHistory(j)">History</button>
              <button class="btn btn-sm btn-outline" @click="toggle(j)">{{ j.enabled ? 'Disable' : 'Enable' }}</button>
              <button class="btn btn-sm btn-outline" @click="openEdit(j)">Edit</button>
              <button class="btn btn-sm btn-danger" @click="remove(j.id)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Modal v-model="showModal" :title="editing ? 'Edit Job' : 'Add Job'" wide @save="save">
      <div class="form-group"><label>Name</label><input v-model="form.name" type="text" class="text-input" /></div>
      <div class="form-group"><label>Source</label><select v-model="form.source_id" class="select-input"><option v-for="s in sources" :key="s.id" :value="s.id">{{ s.name }}</option></select></div>
      <div class="form-group"><label>Destination</label><select v-model="form.destination_id" class="select-input"><option v-for="d in destinations" :key="d.id" :value="d.id">{{ d.name }}</option></select></div>
      <div class="form-group"><label>Schedule (cron or manual)</label><input v-model="form.schedule" type="text" class="text-input" /></div>
      <div class="form-group"><label>Compression</label><select v-model="form.compression" class="select-input"><option value="lz4">LZ4</option><option value="zstd">ZSTD</option><option value="zlib">ZLIB</option><option value="none">None</option></select></div>
      <div class="form-row">
        <div class="form-group"><label>Daily</label><input v-model.number="form.retention_daily" type="number" class="text-input" /></div>
        <div class="form-group"><label>Weekly</label><input v-model.number="form.retention_weekly" type="number" class="text-input" /></div>
        <div class="form-group"><label>Monthly</label><input v-model.number="form.retention_monthly" type="number" class="text-input" /></div>
      </div>
      <div class="form-group"><label class="checkbox-label"><input v-model="form.enabled" type="checkbox" /> Enabled</label></div>
      <div class="form-group"><label class="checkbox-label"><input v-model="form.daily_verification_enabled" type="checkbox" /> Daily verification</label></div>
      <div class="form-group"><label v-for="c in channels" :key="c.id" class="checkbox-label"><input type="checkbox" :value="c.id" v-model="form.notification_channel_ids" /> {{ c.name }}</label></div>
      <div class="form-group"><label>SSH before command</label><input v-model="form.ssh_before_command" type="text" class="text-input" /></div>
      <div class="form-group"><label>SSH after command</label><input v-model="form.ssh_after_command" type="text" class="text-input" /></div>
      <div class="form-group"><label>Restore before command</label><input v-model="form.restore_before_command" type="text" class="text-input" /></div>
      <div class="form-group"><label>Restore after command</label><input v-model="form.restore_after_command" type="text" class="text-input" /></div>
    </Modal>

    <Modal v-model="showHistory" :title="`Run history — ${historyJob?.name || ''}`" wide @save="showHistory = false">
      <table class="data-table">
        <thead><tr><th>Started</th><th>Status</th><th>Duration</th><th>Log</th></tr></thead>
        <tbody>
          <tr v-for="r in historyRuns" :key="r.id">
            <td>{{ timeAgo(r.started_at) }}</td>
            <td><span class="status-badge" :class="statusBadgeClass(r.status)">{{ r.status }}</span></td>
            <td>{{ formatDuration(r.duration_seconds) }}</td>
            <td><button class="btn btn-sm btn-outline" @click="activeRunId = r.id; activeJobId = historyJob?.id; showRun = true">View</button></td>
          </tr>
        </tbody>
      </table>
    </Modal>

    <RunOutputModal v-model="showRun" :run-id="activeRunId" :job-id="activeJobId" />
  </div>
</template>
