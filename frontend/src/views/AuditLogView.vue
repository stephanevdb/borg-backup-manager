<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api/client'
import Modal from '../components/Modal.vue'

const entries = ref<any[]>([])
const users = ref<string[]>([])
const total = ref(0)
const page = ref(1)
const showDetail = ref(false)
const detail = ref<any>(null)
const filters = ref({ action: '', target_type: '', user: '', q: '', from: '', to: '' })

async function load() {
  const params = new URLSearchParams()
  Object.entries(filters.value).forEach(([k, v]) => { if (v) params.set(k, v) })
  params.set('page', String(page.value))
  params.set('limit', '100')
  const data = await api<{ entries: any[]; total?: number }>(`/audit-log?${params}`)
  entries.value = data.entries
  total.value = data.total || data.entries.length
}

async function loadUsers() {
  const data = await api<{ users: string[] }>('/audit-log/users')
  users.value = data.users
}

function openDetail(e: any) {
  detail.value = e
  showDetail.value = true
}

function changePage(delta: number) {
  page.value = Math.max(1, page.value + delta)
  load()
}

onMounted(async () => { await loadUsers(); await load() })
</script>

<template>
  <div class="page-content">
    <div class="page-header"><h1>Audit Log</h1></div>

    <div class="filter-toolbar">
      <div class="form-group"><input v-model="filters.q" type="text" class="text-input" placeholder="Search..." @keyup.enter="load" /></div>
      <div class="form-group">
        <select v-model="filters.action" class="select-input" @change="load">
          <option value="">All actions</option>
          <option value="create">create</option><option value="update">update</option><option value="delete">delete</option><option value="run">run</option><option value="restore">restore</option>
        </select>
      </div>
      <div class="form-group">
        <select v-model="filters.target_type" class="select-input" @change="load">
          <option value="">All types</option>
          <option value="source">source</option><option value="destination">destination</option><option value="job">job</option><option value="archive">archive</option>
        </select>
      </div>
      <div class="form-group">
        <select v-model="filters.user" class="select-input" @change="load">
          <option value="">All users</option>
          <option v-for="u in users" :key="u" :value="u">{{ u }}</option>
        </select>
      </div>
      <div class="form-group"><input v-model="filters.from" type="date" class="text-input" @change="load" /></div>
      <div class="form-group"><input v-model="filters.to" type="date" class="text-input" @change="load" /></div>
      <button class="btn btn-outline" @click="load">Filter</button>
    </div>

    <div class="data-table-wrapper">
      <table class="data-table">
        <thead><tr><th>Time</th><th>User</th><th>Action</th><th>Target</th><th>Name</th><th class="hide-mobile">IP</th><th></th></tr></thead>
        <tbody>
          <tr v-for="e in entries" :key="e.id">
            <td>{{ e.timestamp }}</td>
            <td>{{ e.user }}</td>
            <td>{{ e.action }}</td>
            <td>{{ e.target_type }}</td>
            <td>{{ e.target_name }}</td>
            <td class="hide-mobile">{{ e.ip_address }}</td>
            <td><button class="btn btn-sm btn-outline" @click="openDetail(e)">Details</button></td>
          </tr>
        </tbody>
      </table>
      <div class="data-table-footer">
        <div class="pagination-info text-muted">Page {{ page }}</div>
        <div class="pagination-controls">
          <button class="btn btn-outline btn-sm" :disabled="page <= 1" @click="changePage(-1)">Previous</button>
          <button class="btn btn-outline btn-sm" @click="changePage(1)">Next</button>
        </div>
      </div>
    </div>

    <Modal v-model="showDetail" title="Audit entry" wide @save="showDetail = false">
      <pre v-if="detail" class="run-output-log">{{ JSON.stringify(detail, null, 2) }}</pre>
    </Modal>
  </div>
</template>
