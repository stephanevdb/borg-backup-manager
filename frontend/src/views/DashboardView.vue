<script setup lang="ts">
import { ref, onMounted, shallowRef, computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend } from 'chart.js'
import { api } from '../api/client'
import { formatBytes, timeAgo, formatDuration } from '../composables/useFormatters'
import { statusBadgeClass } from '../composables/useBadges'
import { useSSE } from '../composables/useSSE'
import RunOutputModal from '../components/RunOutputModal.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

const stats = shallowRef<Record<string, any>>({})
const storageHistory = shallowRef<Record<string, { date: string; size: number }[]>>({})
const ready = ref(false)
const page = ref(1)
const showRun = ref(false)
const activeRunId = ref<number | null>(null)
const activeJobId = ref<number | null>(null)

const s30 = computed(() => stats.value.stats_30d || { success: 0, failed: 0 })
const total30 = computed(() => (s30.value.success || 0) + (s30.value.failed || 0))
const successPct = computed(() =>
  total30.value > 0 ? Math.round((s30.value.success / total30.value) * 100) : 0,
)

const donutStyle = computed(() => {
  const success = s30.value.success || 0
  const failed = s30.value.failed || 0
  const total = success + failed
  if (total === 0) return { background: 'conic-gradient(var(--bg-tertiary) 0deg 360deg)' }
  const successDeg = (success / total) * 360
  return { background: `conic-gradient(var(--success) 0deg ${successDeg}deg, var(--error) ${successDeg}deg 360deg)` }
})

const statusBarClass = computed(() => {
  if ((stats.value.failed_unacknowledged || []).length > 0) return 'status-warning'
  if ((stats.value.jobs || 0) > 0) return 'status-ok'
  return 'status-neutral'
})

const statusText = computed(() => {
  const failed = stats.value.failed_unacknowledged || []
  if (failed.length > 0) return `${failed.length} failed backup(s) need attention`
  if ((stats.value.jobs || 0) === 0) return 'No backup jobs configured'
  return 'All systems operational'
})

const chartData = computed(() => {
  const labels: string[] = []
  const datasets: any[] = []
  const colors = ['#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#3b82f6']
  let i = 0
  for (const [name, points] of Object.entries(storageHistory.value)) {
    for (const p of points) {
      if (!labels.includes(p.date)) labels.push(p.date)
    }
    datasets.push({
      label: name,
      data: points.map(p => p.size),
      borderColor: colors[i % colors.length],
      tension: 0.3,
    })
    i++
  }
  return { labels, datasets }
})

const chartOptions = { responsive: true, animation: false as const, plugins: { legend: { position: 'bottom' as const } } }

async function load() {
  const [data, hist] = await Promise.all([
    api<Record<string, any>>(`/dashboard/stats?page=${page.value}`),
    api<Record<string, { date: string; size: number }[]>>('/stats/storage-history').catch(() => ({})),
  ])
  stats.value = data
  storageHistory.value = hist
  ready.value = true
}

async function refreshQuietly() {
  try {
    stats.value = await api<Record<string, any>>(`/dashboard/stats?page=${page.value}`)
  } catch { /* ignore */ }
}

async function acknowledgeFailed() {
  await api('/runs/acknowledge-failed', { method: 'POST' })
  await refreshQuietly()
}

async function runJob(id: number) {
  const res = await api<{ run_id: number }>(`/jobs/${id}/run`, { method: 'POST' })
  activeRunId.value = res.run_id
  activeJobId.value = id
  showRun.value = true
}

function viewLog(runId: number, jobId: number) {
  activeRunId.value = runId
  activeJobId.value = jobId
  showRun.value = true
}

function changePage(delta: number) {
  const maxPage = Math.ceil((stats.value.total_runs || 0) / (stats.value.per_page || 20))
  page.value = Math.min(maxPage, Math.max(1, page.value + delta))
  load()
}

onMounted(() => { load().catch(() => { ready.value = true }) })

useSSE('/api/v1/dashboard/progress', () => { refreshQuietly() })
</script>

<template>
  <div class="page-content dashboard-page">
    <div class="status-bar" :class="statusBarClass">
      <div class="status-indicator">
        <span class="status-dot" />
        <div class="status-indicator-copy">
          <span class="status-text">{{ ready ? statusText : 'Loading...' }}</span>
        </div>
      </div>
      <div class="status-stats">
        <div class="status-stat"><span class="status-stat-label">SOURCES</span><span class="status-stat-value">{{ ready ? stats.sources : '—' }}</span></div>
        <div class="status-stat"><span class="status-stat-label">DESTINATIONS</span><span class="status-stat-value">{{ ready ? stats.destinations : '—' }}</span></div>
        <div class="status-stat"><span class="status-stat-label">LAST BACKUP</span><span class="status-stat-value">{{ ready && stats.last_backup ? timeAgo(stats.last_backup) : '—' }}</span></div>
        <div class="status-stat"><span class="status-stat-label">JOBS</span><span class="status-stat-value">{{ ready ? stats.jobs : '—' }}</span></div>
        <div class="status-stat"><span class="status-stat-label">STORAGE</span><span class="status-stat-value">{{ ready ? formatBytes(stats.total_storage) : '—' }}</span></div>
      </div>
      <nav class="status-quick-links" aria-label="Quick links">
        <RouterLink to="/jobs">Jobs</RouterLink>
        <RouterLink to="/repo-health">Repo health</RouterLink>
        <RouterLink to="/audit-log">Audit log</RouterLink>
      </nav>
      <div class="status-bar-actions">
        <button v-if="(stats.failed_unacknowledged || []).length" class="btn btn-danger btn-sm" style="margin-right: 0.5rem" @click="acknowledgeFailed">Acknowledge</button>
        <button class="btn btn-outline btn-sm" @click="load">Refresh</button>
      </div>
    </div>

    <div v-if="(stats.storage_warnings || []).length" class="storage-warning-chips">
      <RouterLink v-for="w in stats.storage_warnings" :key="w.destination_id" :to="'/destinations'" class="storage-warning-chip">
        {{ w.destination_name }}: {{ w.percent_used?.toFixed?.(1) || w.percent_used }}% used
      </RouterLink>
    </div>

    <div v-if="(stats.failed_unacknowledged || []).length" class="dash-attention">
      <div class="dash-attention-header">Needs attention</div>
      <ul class="dash-attention-list">
        <li v-for="run in stats.failed_unacknowledged" :key="run.id" class="dash-attention-item">
          <span class="dash-attention-job">{{ run.job_name }}</span>
          <span class="dash-attention-time">{{ timeAgo(run.started_at) }}</span>
          <button class="btn btn-outline btn-sm" @click="viewLog(run.id, run.job_id)">View log</button>
        </li>
      </ul>
    </div>

    <div class="dashboard-grid">
      <div class="dash-card">
        <div class="dash-card-header">
          <span>30-DAY SUCCESS</span>
        </div>
        <div class="donut-chart-container">
          <div class="donut-chart" :style="donutStyle">
            <div class="donut-center">
              <span class="donut-value">{{ ready ? successPct + '%' : '—' }}</span>
              <span class="donut-label">{{ total30 }} runs</span>
            </div>
          </div>
        </div>
        <div class="donut-stats">
          <div class="donut-stat"><span class="donut-stat-value donut-stat-success">{{ s30.success || 0 }}</span><span class="donut-stat-label">passed</span></div>
          <div class="donut-stat"><span class="donut-stat-value donut-stat-error">{{ s30.failed || 0 }}</span><span class="donut-stat-label">failed</span></div>
        </div>
      </div>

      <div class="dash-card dash-card-wide">
        <div class="dash-card-header"><span>BACKUP JOBS OVERVIEW</span></div>
        <div class="jobs-overview-grid">
          <div v-if="!(stats.jobs_overview || []).length" class="empty-state">
            <p>No backup jobs configured yet.</p>
            <RouterLink to="/jobs" class="btn btn-primary btn-sm">Create Job</RouterLink>
          </div>
          <div v-for="j in stats.jobs_overview || []" :key="j.id" class="job-overview-card">
            <div class="job-overview-name">{{ j.name }}</div>
            <span class="status-badge" :class="statusBadgeClass(j.last_status)">{{ j.last_status || '—' }}</span>
            <div v-if="j.progress?.message" class="text-muted">{{ j.progress.message }}</div>
            <button class="btn btn-sm btn-primary btn-run" :disabled="!j.enabled" @click="runJob(j.id)">Run Now</button>
          </div>
        </div>
      </div>

      <div class="dash-card dash-card-full">
        <div class="dash-card-header"><span>STORAGE GROWTH</span></div>
        <div class="chart-container dash-chart-container">
          <Line v-if="chartData.datasets.length" :data="chartData" :options="chartOptions" />
          <p v-else class="text-muted">No storage history yet.</p>
        </div>
      </div>

      <div class="dash-card dash-card-full">
        <div class="dash-card-header"><span>RECENT ACTIVITY</span></div>
        <div class="data-table-wrapper">
          <table class="data-table">
            <thead>
              <tr>
                <th>Job</th><th>Status</th><th class="hide-mobile">Started</th><th class="hide-mobile">Duration</th><th class="hide-mobile">Repo</th><th>Log</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="run in stats.recent_runs || []" :key="run.id">
                <td>{{ run.job_name }}</td>
                <td><span class="status-badge" :class="statusBadgeClass(run.status)">{{ run.status }}</span></td>
                <td class="hide-mobile">{{ timeAgo(run.started_at) }}</td>
                <td class="hide-mobile">{{ formatDuration(run.duration_seconds) }}</td>
                <td class="hide-mobile">{{ formatBytes(run.archive_size_bytes) }}</td>
                <td><button class="btn btn-sm btn-outline" @click="viewLog(run.id, run.job_id)">View</button></td>
              </tr>
            </tbody>
          </table>
          <div class="data-table-footer">
            <div class="pagination-info text-muted">Page {{ page }} — {{ stats.total_runs || 0 }} runs</div>
            <div class="pagination-controls">
              <button class="btn btn-outline btn-sm" :disabled="page <= 1" @click="changePage(-1)">Previous</button>
              <button class="btn btn-outline btn-sm" @click="changePage(1)">Next</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <RunOutputModal v-model="showRun" :run-id="activeRunId" :job-id="activeJobId" />
  </div>
</template>

<style scoped>
.job-overview-card { padding: 0.75rem; border: 1px solid var(--border); border-radius: var(--radius-md); display: flex; flex-direction: column; gap: 0.35rem; }
.job-overview-name { font-weight: 600; }
.jobs-overview-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 0.75rem; }
</style>
