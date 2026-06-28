<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { api } from '../api/client'

type DirEntry = { name: string; path: string; has_children?: boolean }

const props = defineProps<{ modelValue: boolean; path?: string }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; select: [string] }>()

const DEFAULT_PATH = '/backup'
const currentPath = ref(DEFAULT_PATH)
const manualPath = ref(DEFAULT_PATH)
const directories = ref<DirEntry[]>([])
const parentPath = ref<string | null>(null)
const loading = ref(false)
const error = ref('')
const selectedPath = ref(DEFAULT_PATH)

const breadcrumbs = computed(() => {
  const p = currentPath.value
  const segments = p.split('/').filter(Boolean)
  const crumbs: { label: string; path: string }[] = []
  let acc = ''
  for (const seg of segments) {
    acc += '/' + seg
    crumbs.push({ label: seg, path: acc })
  }
  return crumbs
})

function resolveStartPath(path?: string) {
  const trimmed = (path || '').trim()
  if (!trimmed) return DEFAULT_PATH
  const safe = ['/app/data', '/app/backup', '/backup']
  if (safe.some((p) => trimmed === p || trimmed.startsWith(p + '/'))) return trimmed
  return DEFAULT_PATH
}

async function load(path: string) {
  loading.value = true
  error.value = ''
  try {
    const data = await api<{
      current: string
      parent: string | null
      directories: DirEntry[]
      error?: string
    }>(`/browse?path=${encodeURIComponent(path)}`)
    currentPath.value = data.current
    manualPath.value = data.current
    selectedPath.value = data.current
    parentPath.value = data.parent
    directories.value = data.directories || []
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to browse'
    directories.value = []
  } finally {
    loading.value = false
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) load(resolveStartPath(props.path))
  },
)

function close() {
  emit('update:modelValue', false)
}

function goUp() {
  if (parentPath.value) load(parentPath.value)
}

function enter(dir: DirEntry) {
  load(dir.path)
}

function selectItem(path: string) {
  selectedPath.value = path
  manualPath.value = path
}

function onPathKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter') load(manualPath.value)
}

function confirm() {
  emit('select', selectedPath.value || currentPath.value)
  close()
}
</script>

<template>
  <div v-if="modelValue" class="modal">
    <div class="modal-backdrop" @click="close" />
    <div class="modal-content modal-wide">
      <div class="modal-header">
        <h3>Browse Folders</h3>
        <button type="button" class="modal-close" @click="close">&times;</button>
      </div>
      <div class="modal-body" style="padding: 0;">
        <div class="fb-toolbar">
          <button
            type="button"
            class="btn btn-sm btn-outline fb-up-btn"
            :disabled="!parentPath"
            title="Go up"
            @click="goUp"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width:14px;height:14px">
              <polyline points="15 18 9 12 15 6" />
            </svg>
          </button>
          <div class="fb-breadcrumb">
            <template v-for="(c, i) in breadcrumbs" :key="c.path">
              <span v-if="i > 0" class="fb-bc-sep">/</span>
              <span class="fb-bc-item" @click="load(c.path)">{{ c.label }}</span>
            </template>
          </div>
        </div>

        <div class="fb-path-bar">
          <input
            v-model="manualPath"
            type="text"
            class="text-input"
            placeholder="Type a path and press Enter..."
            @keydown="onPathKeydown"
          />
          <button type="button" class="btn btn-sm btn-primary" @click="load(manualPath)">Go</button>
        </div>

        <div class="fb-listing">
          <div v-if="loading" class="fb-loading">Loading...</div>
          <div v-else-if="error" class="fb-error">{{ error }}</div>
          <div v-else-if="!directories.length" class="fb-empty">No subdirectories</div>
          <div
            v-for="d in directories"
            v-else
            :key="d.path"
            class="fb-item"
            :class="{ selected: selectedPath === d.path }"
            @click="selectItem(d.path)"
            @dblclick="enter(d)"
          >
            <div class="fb-item-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
              </svg>
            </div>
            <div class="fb-item-name">{{ d.name }}</div>
            <div v-if="d.has_children" class="fb-item-arrow">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="9 18 15 12 9 6" />
              </svg>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-footer fb-modal-footer">
        <div class="fb-selected-path">
          <span class="fb-selected-label">Selected:</span>
          <code id="fb-selected-value">{{ selectedPath }}</code>
        </div>
        <div style="display:flex;gap:0.75rem;">
          <button type="button" class="btn btn-outline" @click="close">Cancel</button>
          <button type="button" class="btn btn-primary" @click="confirm">Select Folder</button>
        </div>
      </div>
    </div>
  </div>
</template>
