import { onUnmounted, ref } from 'vue'

export function useSSE(
  url: string,
  onMessage: (event: string, data: unknown) => void,
) {
  const connected = ref(false)
  let es: EventSource | null = null
  let closed = false

  function connect() {
    if (closed) return
    es = new EventSource(url, { withCredentials: true })
    es.onopen = () => { connected.value = true }
    es.onerror = () => {
      connected.value = false
      // EventSource auto-reconnects; don't tear down or trigger side effects here
    }
    for (const evt of ['log', 'progress', 'run_complete', 'message'] as const) {
      es.addEventListener(evt, (e: Event) => {
        const me = e as MessageEvent
        if (!me.data || me.data.startsWith(':')) return
        try {
          onMessage(evt, JSON.parse(me.data))
        } catch {
          onMessage(evt, me.data)
        }
      })
    }
  }

  connect()
  onUnmounted(() => {
    closed = true
    es?.close()
  })

  return { connected }
}
