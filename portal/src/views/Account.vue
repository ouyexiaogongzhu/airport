<template>
  <div class="page account-page">
    <nav class="topbar">
      <span class="brand">RFPlay</span>
      <div class="nav-links">
        <router-link to="/dashboard">Dashboard</router-link>
        <router-link to="/plans">Plans</router-link>
        <router-link to="/account">Account</router-link>
        <router-link to="/account/devices">Devices</router-link>
        <a href="#" @click.prevent="auth.logout(); $router.push('/login')">Logout</a>
      </div>
      <span class="user-badge">{{ auth.username }}</span>
    </nav>

    <main class="content">
      <h2>Account</h2>
      <p class="subtitle">Profile, subscription, client token, and setup in one place.</p>

      <div v-if="loading" class="loading">Loading account info…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <section class="card-section" id="profile">
        <div class="section-header">
          <h3>Profile</h3>
          <button class="btn-small" type="button" @click="saveProfile" :disabled="profileSaving">
            {{ profileSaving ? 'Saving…' : 'Save' }}
          </button>
        </div>
        <p v-if="profileMsg" :class="profileMsgOk ? 'ok-msg' : 'error-msg'">{{ profileMsg }}</p>
        <div class="form-grid">
          <label class="form-field">
            <span class="label">Username</span>
            <input v-model="profileForm.username" type="text" maxlength="64" autocomplete="username" />
          </label>
          <label class="form-field">
            <span class="label">Display name</span>
            <input v-model="profileForm.display_name" type="text" maxlength="128" autocomplete="name" />
          </label>
          <label class="form-field">
            <span class="label">Email</span>
            <input v-model="profileForm.email" type="email" maxlength="254" autocomplete="email" />
          </label>
          <label class="form-field">
            <span class="label">Phone</span>
            <input v-model="profileForm.phone" type="tel" maxlength="32" autocomplete="tel" />
          </label>
          <label class="form-field full">
            <span class="label">Billing address</span>
            <textarea v-model="profileForm.billing_address" rows="2" maxlength="512"></textarea>
          </label>
        </div>
      </section>

      <section class="card-section" id="billing">
        <div class="section-header">
          <h3>Billing</h3>
          <router-link to="/plans" class="btn-outline">Browse Plans</router-link>
        </div>
        <div v-if="ordersLoading" class="loading">Loading orders…</div>
        <div v-else-if="!orders.length" class="placeholder-box">
          <p>No orders yet.</p>
        </div>
        <div v-else class="orders-list">
          <div v-for="o in orders" :key="o.id" class="info-row">
            <span class="label">#{{ o.id }} · {{ o.status }}</span>
            <span class="value">{{ formatOrderAmount(o) }} · {{ formatOrderDate(o.created_at) }}</span>
          </div>
        </div>
      </section>

      <!-- Subscription Status -->
      <section class="card-section" id="subscription">
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

      <!-- Subscription Link -->
      <section class="card-section">
        <h3>Subscription Link</h3>
        <p class="link-hint">Clash clients (Clash Verge / Stash) use the Clash link. V2rayNG / v2rayA / OpenWrt routers use the Base64 link.</p>

        <template v-if="clashSubscriptionUrl">
          <div class="token-display">
            <code class="token-text">{{ clashSubscriptionUrl }}</code>
          </div>
          <div class="token-actions">
            <button class="btn-outline" @click="copySubscriptionUrl(clashSubscriptionUrl, 'clash')">
              {{ copiedKind === 'clash' ? 'Copied!' : 'Copy Clash Subscription' }}
            </button>
            <button class="btn-outline" @click="copySubscriptionUrl(subscriptionUrl, 'base64')">
              {{ copiedKind === 'base64' ? 'Copied!' : 'Copy Base64 Link (v2rayA / OpenWrt)' }}
            </button>
            <button class="btn-outline" @click="showLinkQr = !showLinkQr">
              {{ showLinkQr ? 'Hide QR Code' : 'Show QR Code' }}
            </button>
          </div>
          <div v-if="showLinkQr" class="qr-area">
            <QrCode :url="clashSubscriptionUrl" />
            <p class="qr-hint">Scan with Clash Verge or import the URL</p>
          </div>
        </template>
        <template v-else>
          <div v-if="tokenData" class="token-display">
            <code class="token-text dim">{{ tokenData.token }}</code>
          </div>
          <p class="no-token">
            Your full subscription link becomes available after resetting your token. Use the
            <strong>Reset Token</strong> button below to generate it.
          </p>
        </template>
      </section>

      <!-- Client Token -->
      <section class="card-section">
        <div class="section-header">
          <h3>Client Token</h3>
          <button class="btn-small" @click="fetchToken(true)" :disabled="tokenLoading">
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
          <div v-if="showQr && clashSubscriptionUrl" class="qr-area">
            <QrCode :url="clashSubscriptionUrl" />
            <p class="qr-hint">Scan with Clash Verge or import URL</p>
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

        <div v-if="newToken" class="new-token-banner">
          <h4>New Token Generated</h4>
          <p class="warning">Save this now — it will only be shown once!</p>
          <code class="token-text full">{{ newToken }}</code>
          <button class="btn-outline" @click="copyNewToken">{{ newCopied ? 'Copied!' : 'Copy New Token' }}</button>
        </div>
      </section>

      <!-- Setup Guide (merged from SetupGuide.vue) -->
      <section class="card-section" id="setup">
        <h3>Setup Guide</h3>
        <p class="section-hint">Follow the steps for your device to get connected.</p>

        <div class="tabs">
          <button
            v-for="t in tabs"
            :key="t.key"
            :class="['tab', { active: activeTab === t.key }]"
            @click="activeTab = t.key"
          >
            {{ t.label }}
          </button>
        </div>

        <div v-if="activeTab === 'v2rayng'" class="guide-section">
          <div class="guide-header">
            <span class="platform-badge android">Android</span>
            <h4>V2rayNG</h4>
          </div>
          <div class="install-methods">
            <a href="https://play.google.com/store/apps/details?id=com.v2ray.ang" target="_blank" rel="noopener" class="method-btn">Google Play</a>
            <a href="https://github.com/2dust/v2rayNG/releases" target="_blank" rel="noopener" class="method-btn">APK Download</a>
          </div>
          <ol class="steps">
            <li>Open V2rayNG app</li>
            <li>Tap <strong>+</strong> icon in the top-right corner</li>
            <li>Select <strong>Import subscription from clipboard</strong></li>
            <li>Paste your <strong>Base64</strong> subscription URL (copied above)</li>
            <li>Tap <strong>✓</strong> to confirm</li>
            <li>Select a node and tap <strong>Connect</strong></li>
          </ol>
        </div>

        <div v-if="activeTab === 'shadowrocket'" class="guide-section">
          <div class="guide-header">
            <span class="platform-badge ios">iOS</span>
            <h4>Shadowrocket</h4>
          </div>
          <div class="install-methods">
            <a href="https://apps.apple.com/app/shadowrocket/id932747118" target="_blank" rel="noopener" class="method-btn">App Store</a>
          </div>
          <ol class="steps">
            <li>Open Shadowrocket app</li>
            <li>Tap the <strong>+</strong> icon in the top-right corner</li>
            <li>Select type: <strong>Subscribe</strong></li>
            <li>Paste your subscription URL</li>
            <li>Tap <strong>Save</strong> (top-right)</li>
            <li>Select a node and toggle <strong>Connect</strong></li>
          </ol>
        </div>

        <div v-if="activeTab === 'clash-verge'" class="guide-section">
          <div class="guide-header">
            <span class="platform-badge desktop">Desktop</span>
            <h4>Clash Verge</h4>
          </div>
          <div class="install-methods">
            <a href="https://github.com/clash-verge-rev/clash-verge-rev/releases" target="_blank" rel="noopener" class="method-btn">GitHub Releases</a>
          </div>
          <ol class="steps">
            <li>Download and install Clash Verge for your OS</li>
            <li>Open Clash Verge → go to <strong>Profiles</strong></li>
            <li>Click <strong>Import</strong> (or paste URL)</li>
            <li>Paste your Clash subscription URL (<code>/clash</code>)</li>
            <li>Click <strong>Import</strong> to confirm</li>
            <li>Go to <strong>Proxies</strong> and select a node</li>
            <li>Toggle <strong>System Proxy</strong> or <strong>TUN Mode</strong></li>
          </ol>
        </div>

        <div v-if="activeTab === 'v2raya'" class="guide-section">
          <div class="guide-header">
            <span class="platform-badge desktop">Linux / OpenWrt</span>
            <h4>v2rayA</h4>
          </div>
          <div class="install-methods">
            <a href="https://github.com/v2rayA/v2rayA/releases" target="_blank" rel="noopener" class="method-btn">GitHub Releases</a>
            <a href="https://v2raya.org/docs/manual/use-other-core/" target="_blank" rel="noopener" class="method-btn">Use xray-core</a>
          </div>
          <ol class="steps">
            <li>Install <strong>v2rayA ≥ 2.2.7.5</strong>（推荐最新 2.5.x）；XHTTP 节点勿用 2.2.7.3 的 minimal 实现</li>
            <li>核心必须是 <strong>xray</strong>（不是 v2ray）：<code>xray version</code>，或设置 <code>V2RAYA_V2RAY_BIN</code> 指向 xray</li>
            <li>导入 <strong>Base64</strong> 订阅（不要带 <code>/clash</code>），更新订阅</li>
            <li>XHTTP 节点（如 w2）测速常显示 <strong>TIMEOUT</strong>：v2rayA HTTP 探测硬超时 8 秒，经 Cloudflare 冷启动常更慢——请<strong>直接连接</strong>试用，勿仅看测速</li>
            <li>若仍不通：用 <strong>w1（ws）</strong>，或改用 Clash Verge 连 XHTTP 节点</li>
          </ol>
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
import { buildSubscriptionUrl } from '../utils/subscriptionUrl'

const auth = useAuthStore()

const profile = ref<any>({})
const loading = ref(true)
const error = ref('')
const profileForm = ref({
  username: '',
  display_name: '',
  email: '',
  phone: '',
  billing_address: '',
})
const profileSaving = ref(false)
const profileMsg = ref('')
const profileMsgOk = ref(false)
const orders = ref<any[]>([])
const ordersLoading = ref(false)

const tokenData = ref<any>(null)
const fullToken = ref('')
const showFullToken = ref(false)
const tokenLoading = ref(false)
const tokenError = ref('')

const showConfirm = ref(false)
const regenerating = ref(false)
const newToken = ref('')
const newCopied = ref(false)

const showQr = ref(false)
const copied = ref(false)
const showLinkQr = ref(false)
const copiedKind = ref('')

const activeTab = ref('v2rayng')
const tabs = [
  { key: 'v2rayng', label: 'V2rayNG' },
  { key: 'shadowrocket', label: 'Shadowrocket' },
  { key: 'clash-verge', label: 'Clash Verge' },
  { key: 'v2raya', label: 'v2rayA' },
]

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

const fullSubscriptionToken = computed(() => newToken.value || profile.value.client_token || '')
const subscriptionUrl = computed(() => buildSubscriptionUrl(fullSubscriptionToken.value))
const clashSubscriptionUrl = computed(() => buildSubscriptionUrl(fullSubscriptionToken.value, 'clash'))

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
  return formatBytes(Math.max(0, limit - used))
}

function formatExpiry(ts: number | undefined | null): string {
  if (!ts || ts <= 0) return '—'
  const d = new Date(ts * 1000)
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

function syncProfileForm(data: any) {
  profileForm.value = {
    username: data.username || '',
    display_name: data.display_name || '',
    email: data.email || '',
    phone: data.phone || '',
    billing_address: data.billing_address || '',
  }
}

async function fetchProfile() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/user/profile')
    profile.value = res.data
    syncProfileForm(res.data)
  } catch (e: any) {
    error.value = e.response?.data?.error || 'Failed to load account info'
  } finally {
    loading.value = false
  }
}

async function saveProfile() {
  profileSaving.value = true
  profileMsg.value = ''
  try {
    const res = await api.put('/user/profile', {
      username: profileForm.value.username,
      display_name: profileForm.value.display_name,
      email: profileForm.value.email,
      phone: profileForm.value.phone,
      billing_address: profileForm.value.billing_address,
    })
    profile.value = res.data
    syncProfileForm(res.data)
    if (auth.user && res.data.username) {
      auth.user = { ...auth.user, username: res.data.username }
    }
    profileMsgOk.value = true
    profileMsg.value = 'Profile saved.'
  } catch (e: any) {
    profileMsgOk.value = false
    profileMsg.value = e.response?.data?.error || 'Failed to save profile'
  } finally {
    profileSaving.value = false
  }
}

function formatOrderAmount(o: any): string {
  if (o.amount == null) return '—'
  return String(o.amount)
}

function formatOrderDate(v: string | undefined | null): string {
  if (!v) return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return String(v)
  return d.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

async function fetchOrders() {
  ordersLoading.value = true
  try {
    const res = await api.get('/user/orders')
    orders.value = res.data?.data || []
  } catch {
    orders.value = []
  } finally {
    ordersLoading.value = false
  }
}

async function fetchToken(skipCache = false) {
  tokenLoading.value = true
  tokenError.value = ''
  showFullToken.value = false
  copied.value = false
  try {
    const res = await api.get('/web/client-token', { cache: skipCache ? { skipCache: true } : undefined })
    tokenData.value = res.data
    fullToken.value = ''
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
  await navigator.clipboard.writeText(tokenData.value.token)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

async function copySubscriptionUrl(url: string, kind: 'clash' | 'base64') {
  if (!url) return
  await navigator.clipboard.writeText(url)
  copiedKind.value = kind
  setTimeout(() => { copiedKind.value = '' }, 2000)
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
  await navigator.clipboard.writeText(newToken.value)
  newCopied.value = true
  setTimeout(() => { newCopied.value = false }, 2000)
}

onMounted(() => {
  fetchProfile()
  fetchToken()
  fetchOrders()
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
.ok-msg { color: #81c784; background: rgba(129,199,132,0.1); border-radius: 8px; padding: 0.75rem 1rem; margin-bottom: 1rem; font-size: 0.9rem; }
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.85rem 1rem;
}
.form-field { display: flex; flex-direction: column; gap: 0.35rem; }
.form-field.full { grid-column: 1 / -1; }
.form-field .label { color: #a0a0b0; font-size: 0.8rem; }
.form-field input,
.form-field textarea {
  background: #0f3460;
  border: 1px solid #1a5276;
  border-radius: 8px;
  color: #e0e0e0;
  padding: 0.55rem 0.75rem;
  font: inherit;
}
.form-field input:focus,
.form-field textarea:focus {
  outline: none;
  border-color: #e94560;
}
.orders-list { display: flex; flex-direction: column; gap: 0.5rem; }
@media (max-width: 640px) {
  .form-grid { grid-template-columns: 1fr; }
}

.card-section {
  background: #16213e;
  border-radius: 12px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
  scroll-margin-top: 1.5rem;
}
.card-section h3 { margin: 0 0 0.5rem; font-size: 1rem; color: #f0f0f0; }
.section-hint { color: #a0a0b0; font-size: 0.85rem; margin: 0 0 1rem; }
.section-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
.section-header h3 { margin: 0; }

.sub-info { display: grid; gap: 0.75rem; }
.info-row { display: flex; justify-content: space-between; align-items: center; }
.info-row .label { color: #a0a0b0; font-size: 0.85rem; }
.info-row .value { color: #e0e0e0; font-size: 0.9rem; font-weight: 500; }
.info-row .value.muted { color: #718096; font-weight: 400; }
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

.placeholder-box {
  padding: 1rem;
  background: #0f3460;
  border-radius: 8px;
  color: #a0a0b0;
  font-size: 0.9rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}
.placeholder-box p { margin: 0; }

.token-area { margin-bottom: 1.5rem; }
.token-display { margin-bottom: 0.75rem; }
.link-hint { color: #a0a0b0; font-size: 0.85rem; margin: 0 0 0.75rem; }
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
.token-text.dim { opacity: 0.6; }
.token-actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }

.qr-area { margin-top: 1rem; padding: 1rem; background: #0f3460; border-radius: 8px; }
.qr-hint { text-align: center; color: #a0a0b0; font-size: 0.8rem; margin: 0.5rem 0 0; }
.no-token { color: #a0a0b0; font-size: 0.9rem; padding: 1rem 0; }

.token-danger { border-top: 1px solid #0f3460; padding-top: 1rem; }
.danger-hint { color: #a0a0b0; font-size: 0.8rem; margin: 0.5rem 0 0; }

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
  text-decoration: none;
  display: inline-block;
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

.tabs { display: flex; gap: 0.5rem; margin-bottom: 1.25rem; flex-wrap: wrap; }
.tab {
  padding: 0.45rem 0.9rem;
  border: 1px solid #0f3460;
  border-radius: 20px;
  background: transparent;
  color: #a0a0b0;
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.2s;
}
.tab:hover { background: rgba(233,69,96,0.1); border-color: #e94560; }
.tab.active { background: #e94560; color: white; border-color: #e94560; }
.guide-section { margin-top: 0.25rem; }
.guide-header { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 1rem; }
.guide-header h4 { margin: 0; font-size: 1.05rem; color: #f0f0f0; }
.platform-badge {
  display: inline-block;
  padding: 0.2rem 0.6rem;
  border-radius: 6px;
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
}
.platform-badge.android { background: #4caf50; color: white; }
.platform-badge.ios { background: #2196f3; color: white; }
.platform-badge.desktop { background: #ff9800; color: white; }
.install-methods { display: flex; gap: 0.75rem; margin-bottom: 1.25rem; flex-wrap: wrap; }
.method-btn {
  display: inline-block;
  padding: 0.4rem 0.9rem;
  background: #0f3460;
  border-radius: 8px;
  color: #e0e0e0;
  text-decoration: none;
  font-size: 0.85rem;
  transition: background 0.2s;
}
.method-btn:hover { background: #1a5276; }
.steps { padding-left: 1.5rem; margin: 0; }
.steps li { margin-bottom: 0.6rem; line-height: 1.5; color: #c0c0d0; font-size: 0.9rem; }
.steps li strong { color: #e94560; }
code { background: #0f3460; padding: 0.1rem 0.4rem; border-radius: 4px; font-size: 0.85rem; }
</style>
