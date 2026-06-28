import { createRouter, createWebHistory } from 'vue-router'
import AppShell from '../components/AppShell.vue'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: AppShell,
      meta: { requiresAuth: true },
      children: [
        { path: '', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
        { path: 'sources', name: 'sources', component: () => import('../views/SourcesView.vue') },
        { path: 'destinations', name: 'destinations', component: () => import('../views/DestinationsView.vue') },
        { path: 'jobs', name: 'jobs', component: () => import('../views/JobsView.vue') },
        { path: 'archives', name: 'archives', component: () => import('../views/ArchivesView.vue') },
        { path: 'repo-health', name: 'repo-health', component: () => import('../views/RepoHealthView.vue') },
        { path: 'ssh-keys', name: 'ssh-keys', component: () => import('../views/SshKeysView.vue') },
        { path: 'audit-log', name: 'audit-log', component: () => import('../views/AuditLogView.vue') },
        { path: 'api-keys', name: 'api-keys', component: () => import('../views/ApiKeysView.vue') },
        { path: 'notifications', name: 'notifications', component: () => import('../views/NotificationsView.vue') },
        { path: 'settings', name: 'settings', component: () => import('../views/SettingsView.vue') },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) {
    await auth.fetchMe()
  }
  if (to.meta.public) {
    if (auth.authenticated && to.name === 'login') {
      return { name: 'dashboard' }
    }
    return true
  }
  if (to.meta.requiresAuth && !auth.authenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
