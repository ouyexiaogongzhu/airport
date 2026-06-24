<template>
  <div class="page subscription-page">
    <nav class="topbar">
      <span class="brand">RFPlay</span>
      <div class="nav-links">
        <router-link to="/dashboard">Dashboard</router-link>
        <router-link to="/products">Plans</router-link>
        <router-link to="/subscription">Subscription</router-link>
        <router-link to="/account">Account</router-link>
        <a href="#" @click.prevent="auth.logout(); $router.push('/')">Logout</a>
      </div>
      <span class="user-badge">{{ auth.username }}</span>
    </nav>

    <main class="content">
      <h2>Subscription</h2>
      <p class="subtitle">Manage your subscription and client token.</p>

      <div v-if="loading" class="loading">Loading subscription info…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <!-- Subscription Status Card -->
      <section v-if="profile.subscription_status" class="card-section">
        <div class="section-header">
          <h3>Subscription Status</h3>
          <span :class="['status-badge', statusClass]">{{ profile.subscription_status }}</span>
        </div>
        <div class="info-grid">
          <div class="info-row">
            <span class="label">Tier</span>
            <span class="value">{{ profile.subscription_tier || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="label">Traffic Used</span>
            <span class="value">{{ formatBytes(profile.traffic_used_bytes) }} / {{ formatBytes(profile.traffic_limit_bytes) }}</span>
          </div>
          <div v-if="profile.traffic_limit_bytes > 0" class="progress-row">
            <div class="progress-bar">
              <div class="progress-fill" :style="{ width: trafficPercent + '%' }"></div>
            </div>
            <span class="progress-label">{{ trafficPercent }}%</span>
          </div>
          <div class="info-row">
            <span class="label">Expires</span>
            <span class="value">{{ formatExpiry(profile.expire_time) }}</span>
          </div>
        </div>
      </section>

      <!-- Client Token -->
      <section class="card-section">
        <div class="section-header">
          <h3>Client Token</h3>
          <button class="btn-small" @click="fetchToken" :disabled="tokenLoading">
            {{ tokenLoading ? '…' : 'Refresh' }}
          </button>
        </div>
        <div v-if="tokenLoading" class="loading">Loading token…</div>
        <div v-if="tokenError" class="error-msg">{{ tokenError }}</div>
        <div v-if="tokenData" class="token-area">
          <div class="token-display">
            <code class="token-text">{{ showFullToken ? fullToken : tokenData.token }}</code>
          </div>
          <div class="token-actions">
            <button class="btn-outline" @click="toggleTokenVisibility">
              {{ showFullToken ? 'Hide' : 'Show Full' }}
            </button>
            <button class="btn-outline" @click="copyToken">
              {{ copied ? 'Copied!' : 'Copy' }}
            </button>
            <button class="btn-outline" @click="showQr = !showQr">
              {{ showQr ? 'Hide QR' : 'Show QR' }}
            </button>
          </div>
          <div v-if="showQr && subscriptionUrl" class="qr-area">
            <QrCode :url="subscriptionUrl" />
            <p class="qr-hint">Scan with V2rayNG or Shadowrocket</p>
          </div>
        </div>
        <div v-if="!tokenData && !tokenLoading" class="no-token">
          <p>No client token found. You need an active subscription to generate one.</p>
        </div>

        <div class="token-danger">
          <button class="btn-danger" @click="confirmRegenerate" :disabled="regenerating">
            {{ regenerating ? 'Regenerating…' : 'Reset Token' }}
          </button>
          <p class="danger-hint">Resetting will invalidate the current token. All devices will need to re-import.</p>
        </div>

        <!-- Confirm modal -->
        <div v-if="showConfirm" class="modal-overlay" @click.self="showConfirm = false">
          <div class="modal">
            <h4>Confirm Token Reset</h4>
            <p>After resetting, all existing connections using this token will stop immediately. You must update every device with the new token.</p>
            <div class="modal-actions">
              <button class="btn-outline" @click="showConfirm = false">Cancel</button>
              <button class="btn-danger" @click="regenerateToken">Confirm Reset</button>
            </div>
          </div>
        </div>

        <!-- New token result -->
        <div v-if="newToken" class="new-token-banner">
          <h4>New Token Generated</h4>
          <p class="warning">Save this now — it will only be shown once!</p>
          <code class="token-text full">{{ newToken }}</code>
          <button class="btn-outline" @click="copyNewToken">{{ newCopied ? 'Copied!' : 'Copy New Token' }}</button>
        </div>
      </section>

      <!-- Available Nodes -->
      <section class="card-section">
        <h3>Available Nodes</h3>
        <div v-if="nodesLoading" class="loading">Loading nodes…</div>
        <div v-if="nodesError" class="error-msg">{{ nodesError }}</div>
        <div v-if="nodes.length" class="node-grid">
          <div v-for="(node, i) in nodes" :key="i" class="node-chip">
            <span class="node-icon">📡</span>
            <span>{{ node }}</span>
          </div>
        </div>
        <div v-if="!nodes.length && !nodesLoading && !nodesError" class="empty">
          <p>No nodes available.</p>
        </div>
      </section>

      <!-- Setup Guide -->
      <div class="guide-link">
        <router-link to="/account/guide" class="btn-outline">📖 Import Setup Guide</router-link>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api/index.js'
import QrCode from '../components/QrCode.vue'

const auth = useAuthStore()

// Profile
const profile = ref<any>({})
const loading = ref(true)
const error = ref('')

// Token
const tokenData = ref<any>(null)
const fullToken = ref('')
const showFullToken = ref(false)
const tokenLoading = ref(false)
const tokenError = ref('')
const copied = ref(false)
const showQr = ref(false)

// Regenerate
const showConfirm = ref(false)
const regenerating = ref(false)
const newToken = ref('')
const newCopied = ref(false)

// Nodes
const nodes = ref<string[]>([])
const nodesLoading = ref(false)
const nodesError = ref('')

const statusClass = computed(() => {
  const s = profile.value.subscription_status || ''
  if (s === 'active') return 'active'
  if (s === 'pending') return 'pending'
  if (s === 'expired' || s === 'disabled') return 'expired'
  return ''
})

const trafficPercent = computed(() => {
  const limit = profile.value.traffic_limit_bytes || 0
  const used = profile.value.traffic_used_bytes || 0
  if (limit <= 0) return 0
  return Math.min(100, Math.round((used / limit) * 100))
})

const subscriptionUrl = computed(() => {
  if (!fullToken.value) return ''
  return `${api.defaults.baseURL}/client/links/${fullToken.value}`
})

function formatBytes(bytes: number | undefined | null): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let b = bytes
  while (b >= 1024 && i < units.length - 1) { b /= 1024; i++ }
  return `${b.toFixed(1)} ${units[i]}`
}

function formatExpiry(ts: number | undefined | null): string {
  if (!ts || ts <= 0) return '—'
  const d = new Date(ts * 1000)
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

async function fetchProfile() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/user/profile')
    profile.value = res.data
  } catch (e: any) {
    error.value = e.response?.data?.error || 'Failed to load account info'
  } finally {
    loading.value = false
  }
}

async function fetchToken() {
  tokenLoading.value = true
  tokenError.value = ''
  showFullToken.value = false
  copied.value = false
  try {
    const res = await api.get('/web/client-token')
    tokenData.value = res.data
  } catch (e: any) {
    tokenError.value = e.response?.data?.error || 'Failed to load token'
    tokenData.value = null
  } finally {
    tokenLoading.value = false
  }
}

function toggleTokenVisibility() {
  if (showFullToken.value) {
    showFullToken.value = false
    return
  }
  if (profile.value.client_token) {
    fullToken.value = profile.value.client_token
    showFullToken.value = true
  }
}

async function copyToken() {
  if (!tokenData.value?.token) return
  try {
    await navigator.clipboard.writeText(tokenData.value.token)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = tokenData.value.token
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

function confirmRegenerate() {
  showConfirm.value = true
}

async function regenerateToken() {
  regenerating.value = true
  showConfirm.value = false
  try {
    const res = await api.post('/web/client-token/regenerate')
    newToken.value = res.data.token
    await fetchToken()
    fullToken.value = ''
  } catch (e: any) {
    tokenError.value = e.response?.data?.error || 'Failed to regenerate token'
  } finally {
    regenerating.value = false
  }
}

async function copyNewToken() {
  if (!newToken.value) return
  try {
    await navigator.clipboard.writeText(newToken.value)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = newToken.value
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
  newCopied.value = true
}

async function fetchNodes() {
  nodesLoading.value = true
  nodesError.value = ''
  try {
    const res = await api.get('/client/subscription')
    nodes.value = res.data.nodes || []
  } catch (e: any) {
    const msg = e.response?.data?.error || ''
    if (msg === 'SUBSCRIPTION_PENDING') {
      nodesError.value = 'Subscription pending — no nodes available yet.'
    } else if (msg === 'SUBSCRIPTION_EXPIRED') {
      nodesError.value = 'Subscription expired — renew to access nodes.'
    } else {
      nodesError.value = msg || 'Failed to load nodes'
    }
  } finally {
    nodesLoading.value = false
  }
}

onMounted(() => {
  fetchProfile()
  fetchToken()
  fetchNodes()
})
</script>

<style scoped>
.subscription-page {
  min-height: 100vh;
  background: #1a1a2e;
}
.topbar {
  display: flex;
  align-items: center;
  padding: 0.75rem 2rem;
  background: #16213e;
  border-bottom: 1px solid #0f3460;
  gap: 2rem;
}
.brand {
  font-weight: 700;
  color: #1a73e8;
  font-size: 1.2rem;
}
.nav-links { display: flex; gap: 1.25rem; flex: 1; }
.nav-links a {
  color: #a0aec0;
  text-decoration: none;
  font-size: 0.9rem;
  font-weight: 500;
}
.nav-links a:hover, .nav-links a.router-link-active { color: #1a73e8; }
.user-badge {
  background: #1a73e8;
  color: white;
  padding: 0.3rem 0.8rem;
  border-radius: 20px;
  font-size: 0.8rem;
  font-weight: 600;
}
.content {
  max-width: 720px;
  margin: 0 auto;
  padding: 2rem;
}
h2 { margin: 0; font-size: 1.5rem; color: #e2e8f0; }
.subtitle { color: #a0aec0; margin: 0.25rem 0 2rem; }

.card-section {
  background: #16213e;
  border: 1px solid #0f3460;
  border-radius: 10px;
  padding: 1.5rem;
  margin-bottom: 1.25rem;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}
.section-header h3 { margin: 0; font-size: 1rem; color: #e2e8f0; }

.status-badge {
  padding: 0.2rem 0.6rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}
.status-badge.active { background: #1b4332; color: #68d391; }
.status-badge.pending { background: #7b5e1e; color: #f6e05e; }
.status-badge.expired { background: #4a1c1c; color: #fc8181; }

.info-grid { display: flex; flex-direction: column; gap: 0.75rem; }
.info-row { display: flex; justify-content: space-between; align-items: center; }
.info-row .label { color: #a0aec0; font-size: 0.85rem; }
.info-row .value { color: #e2e8f0; font-weight: 500; }

.progress-row { display: flex; align-items: center; gap: 0.75rem; }
.progress-bar { flex: 1; height: 6px; background: #0f3460; border-radius: 3px; overflow: hidden; }
.progress-fill { height: 100%; background: #1a73e8; border-radius: 3px; transition: width 0.3s ease; }
.progress-label { font-size: 0.75rem; color: #a0aec0; }

.token-area { margin-bottom: 1rem; }
.token-display {
  background: #0f3460;
  border-radius: 6px;
  padding: 0.75rem;
  margin-bottom: 0.75rem;
  word-break: break-all;
}
.token-text { font-family: monospace; font-size: 0.85rem; color: #68d391; }
.token-text.full { display: block; padding: 0.5rem; background: #1a1a2e; border-radius: 4px; margin-top: 0.5rem; }
.token-actions { display: flex; gap: 0.5rem; }
.qr-area {
  margin-top: 1rem;
  padding: 1rem;
  background: #0f3460;
  border-radius: 8px;
  text-align: center;
}
.qr-hint { color: #a0aec0; font-size: 0.8rem; margin-top: 0.5rem; }
.no-token { color: #a0aec0; margin-bottom: 1rem; }
.token-danger { border-top: 1px solid #0f3460; padding-top: 1rem; }
.danger-hint { color: #a0aec0; font-size: 0.75rem; margin: 0.5rem 0 0; }

.new-token-banner {
  border: 1px solid #68d391;
  background: #1b4332;
  border-radius: 8px;
  padding: 1rem;
  margin-top: 1rem;
}
.new-token-banner h4 { margin: 0 0 0.25rem; color: #68d391; }
.warning { color: #f6e05e; font-size: 0.8rem; margin: 0 0 0.5rem; }

.node-grid { display: flex; flex-wrap: wrap; gap: 0.5rem; }
.node-chip {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  background: #0f3460;
  padding: 0.4rem 0.75rem;
  border-radius: 6px;
  font-size: 0.85rem;
  color: #e2e8f0;
}
.node-icon { font-size: 1rem; }

.guide-link { text-align: center; padding: 1rem; }

.modal-overlay {
  position: fixed;
  top: 0; left: 0; width: 100%; height: 100%;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.modal {
  background: #16213e;
  border: 1px solid #0f3460;
  border-radius: 10px;
  padding: 1.5rem;
  max-width: 420px;
  width: 90%;
}
.modal h4 { margin: 0 0 0.75rem; color: #e2e8f0; }
.modal p { color: #a0aec0; font-size: 0.9rem; }
.modal-actions { display: flex; gap: 0.75rem; justify-content: flex-end; margin-top: 1.25rem; }

.loading { color: #a0aec0; padding: 1rem 0; text-align: center; }
.error-msg { color: #fc8181; padding: 0.5rem; background: #4a1c1c; border-radius: 6px; margin-bottom: 1rem; }
.empty { color: #a0aec0; text-align: center; padding: 1rem; }

.btn-outline {
  background: transparent;
  border: 1px solid #0f3460;
  color: #a0aec0;
  padding: 0.4rem 0.8rem;
  border-radius: 6px;
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.15s;
}
.btn-outline:hover { border-color: #1a73e8; color: #1a73e8; }
.btn-small {
  background: #1a73e8;
  border: none;
  color: white;
  padding: 0.35rem 0.7rem;
  border-radius: 5px;
  font-size: 0.75rem;
  cursor: pointer;
}
.btn-small:disabled { opacity: 0.5; }
.btn-danger {
  background: #c53030;
  border: none;
  color: white;
  padding: 0.4rem 0.8rem;
  border-radius: 6px;
  font-size: 0.8rem;
  cursor: pointer;
}
.btn-danger:disabled { opacity: 0.5; }
</style>
