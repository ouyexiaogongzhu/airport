<template>
  <div class="page account-page">
    <nav class="topbar">
      <span class="brand">RFPlay</span>
      <div class="nav-links">
        <router-link to="/dashboard">Dashboard</router-link>
        <router-link to="/plans">Plans</router-link>
        <router-link to="/account">Account</router-link>
        <router-link to="/subscription">Subscription</router-link>
        <router-link to="/account/guide">Setup Guide</router-link>
        <a href="#" @click.prevent="auth.logout(); $router.push('/')">Logout</a>
      </div>
      <span class="user-badge">{{ auth.username }}</span>
    </nav>

    <main class="content">
      <h2>Account</h2>
      <p class="subtitle">Manage your subscription and client token.</p>

      <div v-if="loading" class="loading">Loading account info…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <!-- Subscription Status -->
      <section class="card-section">
        <h3>Subscription</h3>
        <div class="sub-info">
          <div class="info-row">
            <span class="label">Status</span>
            <span :class="['status-badge', statusClass]">{{ profile.subscription_status || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="label">Plan</span>
            <span class="value">{{ profile.subscription_tier || '—' }}</span>
          </div>
          <div class="info-row">
            <span class="label">Traffic Used</span>
            <span class="value">{{ formatBytes(profile.traffic_used_bytes) }}</span>
          </div>
          <div class="info-row">
            <span class="label">Traffic Limit</span>
            <span class="value">{{ formatBytes(profile.traffic_limit_bytes) }}</span>
          </div>
          <div class="info-row">
            <span class="label">Traffic Remaining</span>
            <span class="value">{{ formatTrafficRemaining() }}</span>
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
            <p class="qr-hint">Scan with V2rayNG or import URL</p>
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

        <!-- Regenerate confirm modal -->
        <div v-if="showConfirm" class="modal-overlay" role="dialog" aria-modal="true" @click.self="showConfirm = false">
          <div class="modal">
            <h4>Confirm Token Reset</h4>
            <p>Are you sure? After resetting, all existing connections using this token will stop working immediately. You will need to update every device with the new token.</p>
            <div class="modal-actions">
              <button class="btn-outline" @click="showConfirm = false">Cancel</button>
              <button class="btn-danger" @click="regenerateToken">Confirm Reset</button>
            </div>
          </div>
        </div>

        <!-- Regenerate result -->
        <div v-if="newToken" class="new-token-banner">
          <h4>New Token Generated</h4>
          <p class="warning">Save this now — it will only be shown once!</p>
          <code class="token-text full">{{ newToken }}</code>
          <button class="btn-outline" @click="copyNewToken">{{ newCopied ? 'Copied!' : 'Copy New Token' }}</button>
        </div>
      </section>

      <!-- Node List -->
      <section class="card-section">
        <h3>Available Nodes</h3>
        <div v-if="nodesLoading" class="loading">Loading nodes…</div>
        <div v-if="nodesError" class="error-msg">{{ nodesError }}</div>
        <div v-if="nodes.length > 0" class="node-list">
          <div v-for="(node, i) in nodes" :key="i" class="node-item">
            <span class="node-icon">🔗</span>
            <span class="node-name">{{ node }}</span>
          </div>
        </div>
        <div v-if="nodes.length === 0 && !nodesLoading && !nodesError" class="no-data">
          No nodes available yet. Purchase a plan to get access.
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api/index'
import QrCode from '../components/QrCode.vue'

const auth = useAuthStore()

// Profile (from /api/v1/user/profile)
const profile = ref<any>({})
const loading = ref(true)
const error = ref('')

// Token
const tokenData = ref<any>(null)
const fullToken = ref('')
const showFullToken = ref(false)
const tokenLoading = ref(false)
const tokenError = ref('')

// Regenerate
const showConfirm = ref(false)
const regenerating = ref(false)
const newToken = ref('')
const newCopied = ref(false)

// Node list
const nodes = ref<string[]>([])
const nodesLoading = ref(false)
const nodesError = ref('')

// QR
const showQr = ref(false)
const copied = ref(false)

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

function formatTrafficRemaining(): string {
  const limit = profile.value.traffic_limit_bytes || 0
  const used = profile.value.traffic_used_bytes || 0
  const rem = Math.max(0, limit - used)
  return formatBytes(rem)
}

function formatExpiry(ts: number | undefined | null): string {
  if (!ts || ts <= 0) return '—'
  const d = new Date(ts * 1000)
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

// Fetch profile (subscription info)
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

// Fetch client token
async function fetchToken() {
  tokenLoading.value = true
  tokenError.value = ''
  showFullToken.value = false
  copied.value = false
  try {
    const res = await api.get('/web/client-token')
    tokenData.value = res.data
    fullToken.value = '' // Full token only available after regenerate or from profile
  } catch (e: any) {
    tokenError.value = e.response?.data?.error || 'Failed to load token'
    tokenData.value = null
  } finally {
    tokenLoading.value = false
  }
}

// Toggle showing full token
function toggleTokenVisibility() {
  if (showFullToken.value) {
    showFullToken.value = false
    return
  }
  // If we don't have full token, try to get from profile
  if (profile.value.client_token) {
    fullToken.value = profile.value.client_token
    showFullToken.value = true
  }
}

// Copy masked token
async function copyToken() {
  if (!tokenData.value?.token) return
  await navigator.clipboard.writeText(tokenData.value.token)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

// Confirm regenerate
function confirmRegenerate() {
  showConfirm.value = true
}

// Regenerate token
async function regenerateToken() {
  regenerating.value = true
  showConfirm.value = false
  try {
    const res = await api.post('/web/client-token/regenerate')
    newToken.value = res.data.token
    // Refresh masked token display
    await fetchToken()
    // Clear fullToken since it's been regenerated
    fullToken.value = ''
  } catch (e: any) {
    tokenError.value = e.response?.data?.error || 'Failed to regenerate token'
  } finally {
    regenerating.value = false
  }
}

// Copy new token
async function copyNewToken() {
  if (!newToken.value) return
  await navigator.clipboard.writeText(newToken.value)
  newCopied.value = true
  setTimeout(() => { newCopied.value = false }, 2000)
}

// Fetch node list
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
.account-page {
  min-height: 100vh;
  background: #1a1a2e;
  color: #e0e0e0;
}
.topbar {
  display: flex;
  align-items: center;
  padding: 0.75rem 2rem;
  background: #16213e;
  border-bottom: 1px solid #0f3460;
  gap: 2rem;
}
.brand { font-weight: 700; color: #e94560; font-size: 1.2rem; }
.nav-links { display: flex; gap: 1.25rem; flex: 1; }
.nav-links a { color: #a0a0b0; text-decoration: none; font-size: 0.9rem; font-weight: 500; }
.nav-links a:hover, .nav-links a.router-link-active { color: #e94560; }
.user-badge { background: rgba(233,69,96,0.15); color: #e94560; padding: 0.3rem 0.8rem; border-radius: 20px; font-size: 0.8rem; font-weight: 600; }
.content { max-width: 800px; margin: 0 auto; padding: 2rem; }
h2 { margin: 0; font-size: 1.5rem; color: #f0f0f0; }
.subtitle { color: #a0a0b0; margin: 0.25rem 0 1.5rem; font-size: 0.9rem; }
.loading { color: #a0a0b0; font-size: 0.9rem; padding: 1rem 0; }
.error-msg { color: #ff6b6b; background: rgba(255,107,107,0.1); border-radius: 8px; padding: 0.75rem 1rem; margin-bottom: 1rem; font-size: 0.9rem; }

/* Card sections */
.card-section {
  background: #16213e;
  border-radius: 12px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}
.card-section h3 { margin: 0 0 1rem; font-size: 1rem; color: #f0f0f0; }
.section-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
.section-header h3 { margin: 0; }

/* Subscription info */
.sub-info { display: grid; gap: 0.75rem; }
.info-row { display: flex; justify-content: space-between; align-items: center; }
.info-row .label { color: #a0a0b0; font-size: 0.85rem; }
.info-row .value { color: #e0e0e0; font-size: 0.9rem; font-weight: 500; }
.status-badge {
  display: inline-block;
  padding: 0.2rem 0.7rem;
  border-radius: 12px;
  font-size: 0.8rem;
  font-weight: 600;
  text-transform: capitalize;
}
.status-badge.active { background: rgba(76,175,80,0.2); color: #81c784; }
.status-badge.pending { background: rgba(255,193,7,0.2); color: #ffd54f; }
.status-badge.expired { background: rgba(244,67,54,0.2); color: #e57373; }

/* Progress bar */
.progress-row { display: flex; align-items: center; gap: 0.75rem; }
.progress-bar {
  flex: 1;
  height: 8px;
  background: #0f3460;
  border-radius: 4px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #e94560, #ff6b6b);
  border-radius: 4px;
  transition: width 0.3s ease;
}
.progress-label { color: #a0a0b0; font-size: 0.8rem; min-width: 2.5rem; text-align: right; }

/* Token area */
.token-area { margin-bottom: 1.5rem; }
.token-display { margin-bottom: 0.75rem; }
.token-text {
  display: block;
  background: #0f3460;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.9rem;
  color: #e0e0e0;
  word-break: break-all;
  user-select: all;
}
.token-text.full { background: #1b5e20; }
.token-actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }

/* QR */
.qr-area { margin-top: 1rem; padding: 1rem; background: #0f3460; border-radius: 8px; }
.qr-hint { text-align: center; color: #a0a0b0; font-size: 0.8rem; margin: 0.5rem 0 0; }

/* No token */
.no-token { color: #a0a0b0; font-size: 0.9rem; padding: 1rem 0; }

/* Token danger zone */
.token-danger { border-top: 1px solid #0f3460; padding-top: 1rem; }
.danger-hint { color: #a0a0b0; font-size: 0.8rem; margin: 0.5rem 0 0; }

/* Buttons */
.btn-small {
  padding: 0.3rem 0.7rem;
  background: #0f3460;
  border: 1px solid #1a5276;
  border-radius: 6px;
  color: #e0e0e0;
  cursor: pointer;
  font-size: 0.8rem;
  transition: all 0.2s;
}
.btn-small:hover { background: #1a5276; }
.btn-outline {
  padding: 0.4rem 0.9rem;
  background: transparent;
  border: 1px solid #0f3460;
  border-radius: 6px;
  color: #e0e0e0;
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.2s;
}
.btn-outline:hover { border-color: #e94560; color: #e94560; }
.btn-danger {
  padding: 0.5rem 1rem;
  background: rgba(244,67,54,0.2);
  border: 1px solid #e57373;
  border-radius: 6px;
  color: #e57373;
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.2s;
}
.btn-danger:hover { background: rgba(244,67,54,0.3); }

/* Modal */
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.modal {
  background: #16213e;
  border-radius: 12px;
  padding: 2rem;
  max-width: 480px;
  width: 90%;
  border: 1px solid #0f3460;
}
.modal h4 { margin: 0 0 0.75rem; color: #e57373; font-size: 1.1rem; }
.modal p { color: #a0a0b0; font-size: 0.9rem; line-height: 1.5; margin: 0 0 1.5rem; }
.modal-actions { display: flex; gap: 0.75rem; justify-content: flex-end; }

/* New token banner */
.new-token-banner {
  background: #1b5e20;
  border-radius: 8px;
  padding: 1.25rem;
  margin-top: 1rem;
}
.new-token-banner h4 { margin: 0 0 0.25rem; color: #81c784; font-size: 1rem; }
.new-token-banner .warning { color: #ffd54f; font-size: 0.85rem; margin: 0 0 0.75rem; }
.new-token-banner .btn-outline { margin-top: 0.75rem; border-color: #81c784; color: #81c784; }
.new-token-banner .btn-outline:hover { border-color: #a5d6a7; color: #a5d6a7; }

/* Node list */
.node-list { display: grid; gap: 0.5rem; }
.node-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 0.75rem;
  background: #0f3460;
  border-radius: 8px;
  font-size: 0.9rem;
}
.node-icon { font-size: 1rem; }
.node-name { color: #e0e0e0; }
.no-data { color: #a0a0b0; font-size: 0.9rem; padding: 1rem 0; }
</style>
