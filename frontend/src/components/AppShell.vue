<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useTheme } from '../composables/useTheme'
import NavIcon from './NavIcon.vue'

const auth = useAuthStore()
const route = useRoute()
const { visibleIcon, themeTitle, cycleTheme } = useTheme()

const MOBILE_BREAKPOINT = 992
const sidebarCollapsed = ref(localStorage.getItem('sidebarCollapsed') === 'true')
const mobileOpen = ref(false)

const navItems = [
  { to: '/', label: 'Dashboard', icon: 'dashboard' },
  { to: '/sources', label: 'Sources', icon: 'sources' },
  { to: '/destinations', label: 'Destinations', icon: 'destinations' },
  { to: '/jobs', label: 'Backup Jobs', icon: 'jobs' },
  { to: '/archives', label: 'Backups', icon: 'archives' },
  { to: '/repo-health', label: 'Repo Health', icon: 'health' },
  { to: '/ssh-keys', label: 'SSH Keys', icon: 'ssh' },
  { to: '/audit-log', label: 'Audit Log', icon: 'audit' },
  { to: '/api-keys', label: 'API Keys', icon: 'api-keys' },
  { to: '/notifications', label: 'Notifications', icon: 'notifications' },
  { to: '/settings', label: 'Settings', icon: 'settings' },
]

function isMobile() {
  return window.innerWidth <= MOBILE_BREAKPOINT
}

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path === path || route.path.startsWith(path + '/')
}

function syncMobileOverlay() {
  document.body.classList.toggle('mobile-overlay-open', isMobile() && mobileOpen.value)
}

function closeMobileSidebar() {
  if (!isMobile()) return
  mobileOpen.value = false
  syncMobileOverlay()
}

function toggleSidebar() {
  if (isMobile()) {
    sidebarCollapsed.value = false
    mobileOpen.value = !mobileOpen.value
    syncMobileOverlay()
  } else {
    sidebarCollapsed.value = !sidebarCollapsed.value
    localStorage.setItem('sidebarCollapsed', String(sidebarCollapsed.value))
  }
}

function onResize() {
  if (isMobile()) {
    sidebarCollapsed.value = false
  } else {
    mobileOpen.value = false
  }
  syncMobileOverlay()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') closeMobileSidebar()
}

onMounted(() => {
  if (isMobile()) {
    sidebarCollapsed.value = false
  } else if (localStorage.getItem('sidebarCollapsed') === 'true') {
    sidebarCollapsed.value = true
  }
  window.addEventListener('resize', onResize)
  document.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  window.removeEventListener('resize', onResize)
  document.removeEventListener('keydown', onKeydown)
  document.body.classList.remove('mobile-overlay-open')
})
</script>

<template>
  <div class="app-root">
  <nav class="navbar">
    <div class="nav-brand">
      <button class="sidebar-toggle" type="button" title="Toggle sidebar" @click="toggleSidebar">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <line x1="3" y1="6" x2="21" y2="6" />
          <line x1="3" y1="12" x2="21" y2="12" />
          <line x1="3" y1="18" x2="21" y2="18" />
        </svg>
      </button>
      <RouterLink to="/" class="brand-link">
        <svg class="logo" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
        </svg>
        <span>Borg Backup Manager</span>
      </RouterLink>
    </div>
    <div class="nav-user">
      <button
        id="theme-toggle"
        class="btn-icon"
        type="button"
        :title="themeTitle"
        :aria-label="themeTitle"
        @click="cycleTheme"
      >
        <svg
          v-show="visibleIcon === 'system'"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          aria-hidden="true"
        >
          <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
          <line x1="8" y1="21" x2="16" y2="21" />
          <line x1="12" y1="17" x2="12" y2="21" />
        </svg>
        <svg
          v-show="visibleIcon === 'dark'"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          aria-hidden="true"
        >
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
        <svg
          v-show="visibleIcon === 'light'"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="5" />
          <line x1="12" y1="1" x2="12" y2="3" />
          <line x1="12" y1="21" x2="12" y2="23" />
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
          <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
          <line x1="1" y1="12" x2="3" y2="12" />
          <line x1="21" y1="12" x2="23" y2="12" />
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
          <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
        </svg>
      </button>
      <span class="user-name">{{ auth.user?.preferred_username || auth.user?.name || auth.user?.email || 'User' }}</span>
      <button class="btn btn-outline btn-sm" type="button" @click="auth.logout()">Logout</button>
    </div>
  </nav>

  <div class="app-layout">
    <aside
      id="sidebar"
      class="sidebar"
      :class="{ collapsed: sidebarCollapsed, show: mobileOpen }"
    >
      <div class="sidebar-section">
        <div class="sidebar-label">BACKUP</div>
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="sidebar-item"
          :class="{ active: isActive(item.to) }"
          @click="closeMobileSidebar"
        >
          <NavIcon :name="item.icon" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </div>
    </aside>
    <div class="sidebar-backdrop" @click="closeMobileSidebar" />

    <main class="main-content">
      <RouterView v-slot="{ Component }">
        <KeepAlive :max="12">
          <component :is="Component" />
        </KeepAlive>
      </RouterView>
    </main>
  </div>
  </div>
</template>
