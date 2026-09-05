<template>
  <div v-if="siteKey" ref="widgetEl" class="turnstile-box"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'

// Cloudflare Turnstile widget. Renders nothing (no error) when
// VITE_TURNSTILE_SITE_KEY is not configured. The verified token is exposed
// via v-model for the parent form to send as `cf-turnstile-response`.
const token = defineModel<string>()

const siteKey = import.meta.env.VITE_TURNSTILE_SITE_KEY
const widgetEl = ref<HTMLElement>()
let widgetId = ''

interface TurnstileApi {
  render: (el: HTMLElement, opts: Record<string, unknown>) => string
  remove: (id: string) => void
}
declare global {
  interface Window {
    turnstile?: TurnstileApi
  }
}

// Load the official api.js once per page; render mode is explicit so the
// widget works after SPA navigation (implicit rendering only scans on load).
let scriptPromise: Promise<void> | null = null
function loadTurnstileScript(): Promise<void> {
  if (window.turnstile) return Promise.resolve()
  scriptPromise ??= new Promise((resolve, reject) => {
    const s = document.createElement('script')
    s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => reject(new Error('Failed to load Turnstile script'))
    document.head.appendChild(s)
  })
  return scriptPromise
}

onMounted(async () => {
  if (!siteKey || !widgetEl.value) return
  try {
    await loadTurnstileScript()
  } catch {
    return // Script blocked/failed — widget stays hidden, token stays empty
  }
  const ts = window.turnstile
  if (!ts || !widgetEl.value) return
  widgetId = ts.render(widgetEl.value, {
    sitekey: siteKey,
    callback: (t: string) => { token.value = t },
    'expired-callback': () => { token.value = '' },
  })
})

onBeforeUnmount(() => {
  if (widgetId) window.turnstile?.remove(widgetId)
})
</script>

<style scoped>
.turnstile-box { margin: 0.5rem 0 0.75rem; }
</style>
