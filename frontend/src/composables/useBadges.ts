export function statusBadgeClass(status: string | null | undefined): string {
  if (!status) return 'status-neutral'
  const s = status.toLowerCase()
  if (s === 'success' || s === 'ok' || s === 'healthy') return 'status-success'
  if (s === 'failed' || s === 'error' || s === 'unhealthy') return 'status-error'
  if (s === 'running' || s === 'in_progress') return 'status-running'
  if (s === 'warning' || s === 'degraded') return 'status-warning'
  return 'status-neutral'
}

export function typeBadgeClass(type: string | null | undefined): string {
  if (!type) return 'type-local'
  return `type-${type.toLowerCase()}`
}
