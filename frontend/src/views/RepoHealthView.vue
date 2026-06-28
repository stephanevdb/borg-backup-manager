<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api/client'
import { statusBadgeClass } from '../composables/useBadges'
import { timeAgo, formatDuration } from '../composables/useFormatters'
import Modal from '../components/Modal.vue'

const repos = ref<any[]>([])
const jobs = ref<any[]>([])
const destinations = ref<any[]>([])
const checks = ref<any[]>([])
const showChecks = ref(false)
const checkDestId = ref<number | null>(null)
const checkJobId = ref<number | null>(null)
const showDiff = ref(false)
const diffResult = ref<any>({})
const diffA1 = ref('')
const diffA2 = ref('')
const diffDestId = ref<number | null>(null)
const diffJobId = ref<number | null>(null)
const logCheck = ref<any>(null)
const showLogModal = ref(false)

async function load() {
  const [o, j, d] = await Promise.all([
    api<{ repos: any[] }>('/health/overview'),
    api<{ jobs: any[] }>('/jobs'),
    api<{ destinations: any[] }>('/destinations'),
  ])
  repos.value = o.repos
  jobs.value = j.jobs
  destinations.value = d.destinations
}

async function verify(repo: any) {
  await api(`/destinations/${repo.destination_id}/verify?job_id=${repo.job_id}`, { method: 'POST' })
  await load()
}

async function openChecks(destId: number, jobId: number) {
  checkDestId.value = destId
  checkJobId.value = jobId
  const data = await api<{ checks: any[] }>(`/destinations/${destId}/checks?job_id=${jobId}`)
  checks.value = data.checks
  showChecks.value = true
}

async function runDiff() {
  const data = await api<any>(
    `/archives/diff?destination_id=${diffDestId.value}&job_id=${diffJobId.value}&archive1=${encodeURIComponent(diffA1.value)}&archive2=${encodeURIComponent(diffA2.value)}`,
  )
  diffResult.value = data
}

onMounted(load)
</script>

<template>
  <div class="page-content">
    <div class="page-header">
      <h1>Repository Health</h1>
      <button class="btn btn-outline btn-sm" @click="showDiff = true">Archive diff</button>
    </div>

    <div class="dashboard-grid">
      <div v-for="r in repos" :key="`${r.destination_id}-${r.job_id}`" class="dash-card">
        <div class="dash-card-header"><span>{{ r.destination_name }}</span></div>
        <p class="text-muted">{{ r.job_name }}</p>
        <p>Last check: <span class="status-badge" :class="statusBadgeClass(r.last_check_status)">{{ r.last_check_status }}</span></p>
        <div class="actions-cell">
          <button class="btn btn-primary btn-sm" @click="verify(r)">Run Check</button>
          <button class="btn btn-outline btn-sm" @click="openChecks(r.destination_id, r.job_id)">History</button>
        </div>
      </div>
    </div>

    <Modal v-model="showChecks" title="Verification History" wide @save="showChecks = false">
      <table class="data-table">
        <thead><tr><th>Started</th><th>Status</th><th>Duration</th><th>Log</th></tr></thead>
        <tbody>
          <tr v-for="c in checks" :key="c.id">
            <td>{{ timeAgo(c.started_at) }}</td>
            <td><span class="status-badge" :class="statusBadgeClass(c.status)">{{ c.status }}</span></td>
            <td>{{ formatDuration(c.duration_seconds) }}</td>
            <td><button class="btn btn-sm btn-outline" @click="logCheck = c; showLogModal = true">View</button></td>
          </tr>
        </tbody>
      </table>
    </Modal>

    <Modal v-model="showDiff" title="Archive Diff" wide @save="runDiff">
      <div class="diff-controls filter-toolbar">
        <select v-model="diffDestId" class="select-input"><option v-for="d in destinations" :key="d.id" :value="d.id">{{ d.name }}</option></select>
        <select v-model="diffJobId" class="select-input"><option v-for="j in jobs" :key="j.id" :value="j.id">{{ j.name }}</option></select>
        <input v-model="diffA1" class="text-input" placeholder="Archive 1" />
        <input v-model="diffA2" class="text-input" placeholder="Archive 2" />
      </div>
      <div v-if="diffResult.added" class="diff-summary">
        <div class="diff-summary-item diff-summary-added"><span class="diff-summary-count">{{ diffResult.added?.length || 0 }}</span><span class="diff-summary-label">added</span></div>
        <div class="diff-summary-item diff-summary-modified"><span class="diff-summary-count">{{ diffResult.changed?.length || 0 }}</span><span class="diff-summary-label">changed</span></div>
        <div class="diff-summary-item diff-summary-removed"><span class="diff-summary-count">{{ diffResult.deleted?.length || 0 }}</span><span class="diff-summary-label">removed</span></div>
      </div>
      <div v-if="diffResult.added?.length" class="diff-file-list">
        <div class="diff-group"><div class="diff-group-header diff-group-added">Added</div><div v-for="f in diffResult.added" :key="f" class="diff-file diff-file-added">{{ f }}</div></div>
      </div>
    </Modal>

    <Modal v-model="showLogModal" title="Check log" wide @save="showLogModal = false">
      <pre v-if="logCheck" class="run-output-log">{{ logCheck.log_output }}</pre>
    </Modal>
  </div>
</template>
