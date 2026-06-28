import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface User {
  sub: string
  email?: string
  name?: string
  preferred_username?: string
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const authenticated = ref(false)
  const ready = ref(false)

  async function fetchMe() {
    try {
      const res = await fetch('/auth/me', { credentials: 'include' })
      const data = await res.json()
      authenticated.value = !!data.authenticated
      user.value = data.user || null
    } catch {
      authenticated.value = false
      user.value = null
    } finally {
      ready.value = true
    }
  }

  async function logout() {
    await fetch('/auth/logout', { method: 'POST', credentials: 'include' })
    user.value = null
    authenticated.value = false
    window.location.href = '/auth/login'
  }

  return { user, authenticated, ready, fetchMe, logout }
})
