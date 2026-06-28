<script setup lang="ts">
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
</script>

<template>
  <div class="page-content">
    <div class="dash-card" style="max-width: 420px; margin: 4rem auto; text-align: center;">
      <svg class="logo" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width: 48px; height: 48px; color: var(--accent-primary); margin: 0 auto 1rem;">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      </svg>
      <h1 style="font-size: 1.5rem; margin-bottom: 0.5rem;">Borg Backup Manager</h1>
      <p class="text-muted" style="margin-bottom: 1.5rem;">Sign in to manage your backups.</p>
      <a href="/auth/login" class="btn btn-primary btn-lg">Sign in</a>
      <button class="btn btn-outline btn-sm" style="margin-left: 0.5rem;" @click="auth.fetchMe()">Retry</button>
    </div>
  </div>
</template>
