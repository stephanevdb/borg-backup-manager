<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api/client'

const settings = ref<Record<string, string>>({})

async function load() {
  const data = await api<{ settings: Record<string, string> }>('/settings')
  settings.value = data.settings
}

async function save(key: string) {
  await api('/settings', { method: 'PUT', body: JSON.stringify({ [key]: settings.value[key] }) })
}

async function exportConfig() {
  window.open('/api/v1/config/export', '_blank')
}

async function importConfig(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const text = await file.text()
  const data = JSON.parse(text)
  const res = await api<{ imported: Record<string, number> }>('/config/import', { method: 'POST', body: JSON.stringify(data) })
  alert('Imported: ' + JSON.stringify(res.imported))
  await load()
}

const labels: Record<string, string> = {
  default_compression: 'Default Compression',
  default_retention_daily: 'Retention Daily',
  default_retention_weekly: 'Retention Weekly',
  default_retention_monthly: 'Retention Monthly',
  daily_verification_enabled: 'Daily Verification',
  daily_verification_cron: 'Verification Cron',
  storage_warning_interval: 'Storage Warning Interval (hours)',
  backup_log_retention_days: 'Backup Log Retention (days)',
  audit_log_retention_days: 'Audit Log Retention (days)',
  concurrent_backup_limit: 'Concurrent Backup Limit',
  ssh_address_family: 'SSH Address Family',
}

onMounted(load)
</script>

<template>
  <div class="page-content settings-page">
    <div class="page-header">
      <h1>Settings</h1>
    </div>

    <div class="dash-card">
      <div class="dash-card-header">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
        </svg>
        <span>GENERAL</span>
      </div>
      <div v-for="key in Object.keys(settings)" :key="key" class="settings-item">
        <label class="settings-item-label">{{ labels[key] || key }}</label>
        <div class="settings-item-control">
          <input v-model="settings[key]" type="text" class="text-input" @change="save(key)" />
        </div>
      </div>
    </div>

    <div class="dash-card">
      <div class="dash-card-header">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
          <polyline points="7 10 12 15 17 10" />
          <line x1="12" y1="15" x2="12" y2="3" />
        </svg>
        <span>CONFIGURATION</span>
      </div>
      <div class="page-actions">
        <button class="btn btn-outline" @click="exportConfig">Export Config</button>
        <label class="btn btn-outline">
          Import Config
          <input type="file" accept=".json" hidden @change="importConfig" />
        </label>
      </div>
    </div>
  </div>
</template>
