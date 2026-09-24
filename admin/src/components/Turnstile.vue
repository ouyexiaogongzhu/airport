<template>
  <div v-if="siteKey" class="turnstile-wrap">
    <div ref="widgetEl" class="turnstile-box"></div>
    <p v-if="status === 'pending'" class="turnstile-status">自动校验中…</p>
    <p v-else-if="status === 'error'" class="turnstile-status error">
      {{ statusError }}
      <button class="turnstile-retry" type="button" @click="retry">重试</button>
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick } from 'vue'

// Cloudflare Turnstile widget (dashboard mode: non-interactive).
// Renders nothing when VITE_TURNSTILE_SITE_KEY is unset.
// Auto-runs on mount; exposes the verified token via v-model.
const token = defineModel<string>()

const siteKey = (import.meta.env.VITE_TURNSTILE_SITE_KEY as string | undefined)?.trim() || ''
const widgetEl = ref<HTMLElement>()
const status = ref<'idle' | 'pending' | 'ready' | 'error'>(siteKey ? 'pending' : 'idle')
const statusError = ref('人机验证加载失败，请刷新重试')
let widgetId = ''

interface TurnstileApi {
  render: (el: HTMLElement, opts: Record<string, unknown>) => string
  remove: (id: string) => void
  reset: (id: string) => void
}
declare global {
  interface Window {
    turnstile?: TurnstileApi
  }
}

let scriptPromise: Promise<void> | null = null
function loadTurnstileScript(): Promise<void> {
  if (window.turnstile) return Promise.resolve()
  scriptPromise ??= new Promise((resolve, reject) => {
    const s = document.createElement('script')
    s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => {
      // 失败的 promise 不能缓存：否则一次加载失败会毒化之后所有重试，只能整页刷新
      scriptPromise = null
      reject(new Error('Failed to load Turnstile script'))
    }
    document.head.appendChild(s)
  })
  return scriptPromise
}

async function renderWidget() {
  if (!siteKey) return
  status.value = 'pending'
  await nextTick()
  if (!widgetEl.value) {
    status.value = 'error'
    statusError.value = '人机验证控件未就绪，请刷新重试'
    return
  }
  try {
    await loadTurnstileScript()
  } catch {
    status.value = 'error'
    statusError.value = '人机验证脚本加载失败（请检查网络或广告拦截）'
    return
  }
  const ts = window.turnstile
  if (!ts || !widgetEl.value) {
    status.value = 'error'
    return
  }
  if (widgetId) {
    try {
      ts.remove(widgetId)
    } catch {
      /* already gone */
    }
    widgetId = ''
  }
  // non-interactive widget mode (dashboard) + interaction-only UI:
  // runs automatically on render; no checkbox unless CF escalates.
  widgetId = ts.render(widgetEl.value, {
    sitekey: siteKey,
    appearance: 'interaction-only',
    callback: (t: string) => {
      token.value = t
      status.value = 'ready'
    },
    'expired-callback': () => {
      token.value = ''
      status.value = 'pending'
    },
    'error-callback': () => {
      token.value = ''
      status.value = 'error'
      statusError.value = '人机验证失败'
    },
  })
}

async function retry() {
  scriptPromise = null
  await renderWidget()
}

onMounted(renderWidget)

onBeforeUnmount(() => {
  if (widgetId) window.turnstile?.remove(widgetId)
})

defineExpose({
  reset() {
    token.value = ''
    if (widgetId && window.turnstile) {
      status.value = 'pending'
      window.turnstile.reset(widgetId)
    }
  },
  retry,
  get ready() {
    return !siteKey || !!token.value
  },
  get required() {
    return !!siteKey
  },
})
</script>

<style scoped>
.turnstile-wrap { margin: 0.5rem 0 0.75rem; }
.turnstile-box { min-height: 0; }
.turnstile-status { margin: 0.35rem 0 0; font-size: 0.8rem; color: #aaa; }
.turnstile-status.error { color: #ff6b6b; }
.turnstile-retry { margin-left: 0.4rem; font-size: 0.8rem; cursor: pointer; }
</style>
