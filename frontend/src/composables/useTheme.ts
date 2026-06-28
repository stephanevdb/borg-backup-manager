import { ref, computed, onMounted, onUnmounted } from 'vue'

const THEME_OPTIONS = ['system', 'light', 'dark'] as const
export type ThemePreference = (typeof THEME_OPTIONS)[number]

const preference = ref<ThemePreference>('system')
const systemIconHint = ref(false)
let hintTimeout: ReturnType<typeof setTimeout> | null = null

function normalizeTheme(value: string | null): ThemePreference {
  return THEME_OPTIONS.includes(value as ThemePreference) ? (value as ThemePreference) : 'system'
}

function getSystemTheme(): 'light' | 'dark' {
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

function resolveTheme(pref: ThemePreference): 'light' | 'dark' {
  if (pref === 'light' || pref === 'dark') return pref
  return getSystemTheme()
}

function applyTheme(pref: ThemePreference) {
  const resolved = resolveTheme(pref)
  document.documentElement.classList.toggle('light-mode', resolved === 'light')
  document.documentElement.dataset.themePreference = pref
}

const themeLabels: Record<ThemePreference, string> = {
  system: 'Follow system',
  light: 'Light',
  dark: 'Dark',
}

export function useTheme() {
  const visibleIcon = computed(() => {
    if (preference.value === 'system' && systemIconHint.value) return 'system'
    if (preference.value === 'system') return resolveTheme('system')
    return preference.value
  })

  const themeTitle = computed(
    () => `Theme: ${themeLabels[preference.value]} (click to change)`,
  )

  function cycleTheme() {
    const idx = THEME_OPTIONS.indexOf(preference.value)
    preference.value = THEME_OPTIONS[(idx + 1) % THEME_OPTIONS.length]
    localStorage.setItem('theme', preference.value)
    applyTheme(preference.value)

    if (hintTimeout) clearTimeout(hintTimeout)
    if (preference.value === 'system') {
      systemIconHint.value = true
      hintTimeout = setTimeout(() => {
        systemIconHint.value = false
        hintTimeout = null
      }, 5000)
    } else {
      systemIconHint.value = false
    }
  }

  let mq: MediaQueryList | null = null
  const onSystemChange = () => {
    if (preference.value === 'system') applyTheme('system')
  }

  onMounted(() => {
    preference.value = normalizeTheme(localStorage.getItem('theme'))
    applyTheme(preference.value)
    mq = window.matchMedia('(prefers-color-scheme: light)')
    mq.addEventListener('change', onSystemChange)
  })

  onUnmounted(() => {
    mq?.removeEventListener('change', onSystemChange)
    if (hintTimeout) clearTimeout(hintTimeout)
  })

  return { preference, visibleIcon, themeTitle, cycleTheme }
}
