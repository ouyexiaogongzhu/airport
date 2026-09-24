<template>
  <div class="page tokens-page">
    <main class="main">
      <!-- ── Issue Token（at_ 自建訂閱令牌）─────────────────────────────── -->
      <header class="topbar">
        <h2>Tokens</h2>
        <div class="topbar-right">
          <button class="btn-sm" @click="loadTokens(true)">🔄 Refresh</button>
        </div>
      </header>

      <section class="issue-card">
        <h3>Issue Token</h3>
        <p class="hint">Creates a registration-free subscription token. The plaintext token is shown once after issuing.</p>
        <div class="issue-form">
          <label>Days <input v-model.number="issueDays" type="number" min="1" step="1" /></label>
          <label>Traffic (GB) <input v-model.number="issueGb" type="number" min="0.1" step="any" /></label>
          <label>Note <input v-model="issueNote" type="text" maxlength="200" placeholder="optional, ≤200 chars" /></label>
          <button class="btn-sm" :disabled="issuing" @click="issueToken">{{ issuing ? '…' : 'Issue' }}</button>
        </div>
        <div v-if="issued" class="issued-box">
          <p class="hint">Token shown only once — copy a subscription URL now.</p>
          <code class="issued-token">{{ issued.token }}</code>
          <div class="issued-actions">
            <button class="btn-tiny btn-sub" @click="copyIssuedUrl('base64')">Copy Base64 URL</button>
            <button class="btn-tiny btn-sub" @click="copyIssuedUrl('clash')">Copy Clash URL</button>
          </div>
        </div>
      </section>

      <div v-if="tokensLoading" class="loading">Loading tokens…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>
      <div v-if="successMsg" class="success-msg">{{ successMsg }}</div>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Token</th>
              <th>Note</th>
              <th>Status</th>
              <th>Expires</th>
              <th>Usage</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in tokens" :key="t.id">
              <td>{{ t.id }}</td>
              <td><code class="token-text">{{ t.client_token_prefix }}…</code></td>
              <td>{{ t.note || '—' }}</td>
              <td><span :class="['status', t.subscription_status]">{{ t.subscription_status }}</span></td>
              <td class="date-cell">{{ formatDate(t.expire_time) }}</td>
              <td>
                <div class="usage">
                  <div class="usage-bar"><div class="usage-fill" :style="{ width: usagePct(t) + '%' }"></div></div>
                  <span class="usage-text">{{ formatBytes(t.traffic_used_bytes) }} / {{ formatBytes(t.traffic_limit_bytes) }}</span>
                </div>
              </td>
              <td class="actions-cell">
                <button class="btn-tiny" :disabled="t.subscription_status !== 'active'" @click="renewToken(t)">Renew</button>
                <button class="btn-tiny btn-danger" :disabled="t.subscription_status !== 'active'" @click="revokeToken(t)">Revoke</button>
              </td>
            </tr>
            <tr v-if="!tokensLoading && tokens.length === 0">
              <td colspan="7" class="empty-row">No tokens issued yet</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- ── User Tokens（既有功能，保持不動）───────────────────────────── -->
      <header class="topbar user-tokens-header">
        <h2>User Tokens</h2>
        <div class="topbar-right">
          <input v-model="search" placeholder="Search by username…" class="search-input" />
          <button class="btn-sm" @click="loadUsers(true)">🔄 Refresh</button>
        </div>
      </header>

      <p class="hint">
        Copy Clash or Base64 subscription URLs for an existing user — no new account needed.
        Rotate token only when the old link should stop working.
      </p>

      <div v-if="loading" class="loading">Loading users…</div>

      <div v-if="!loading" class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Username</th>
              <th>Client Token</th>
              <th>Status</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in filteredUsers" :key="u.id">
              <td>{{ u.id }}</td>
              <td><strong>{{ u.username }}</strong></td>
              <td>
                <code class="token-text">{{ maskToken(u.client_token) }}</code>
              </td>
              <td><span :class="['status', u.status]">{{ u.status }}</span></td>
              <td class="date-cell">{{ formatDate(u.created_at) }}</td>
              <td class="actions-cell">
                <button
                  class="btn-tiny btn-sub"
                  :disabled="!u.client_token"
                  title="Copy v2rayNG / Base64 subscription URL"
                  @click="copySubUrl(u, 'base64')"
                >Copy Base64</button>
                <button
                  class="btn-tiny btn-sub"
                  :disabled="!u.client_token"
                  title="Copy Clash subscription URL"
                  @click="copySubUrl(u, 'clash')"
                >Copy Clash</button>
                <button
                  class="btn-tiny"
                  @click="regenerateToken(u)"
                  :disabled="regeneratingId === u.id"
                >
                  {{ regeneratingId === u.id ? '…' : 'Rotate token' }}
                </button>
              </td>
            </tr>
            <tr v-if="filteredUsers.length === 0">
              <td colspan="6" class="empty-row">No users found</td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import api from '../api/index'
import { buildSubscriptionUrl, type SubscriptionFormat } from '../utils/subscriptionUrl'

interface TokenRow {
  id: number
  username: string
  note: string | null
  subscription_status: string
  expire_time: number
  traffic_used_bytes: number
  traffic_limit_bytes: number
  client_token_prefix: string
  created_at: string
}

interface User {
  id: number
  username: string
  client_token: string
  status: string
  role: string
  subscription_status: string
  created_at: string
}

// ── at_ tokens ──────────────────────────────────────────────────────────────

const tokens = ref<TokenRow[]>([])
const tokensLoading = ref(false)
const issueDays = ref(30)
const issueGb = ref(50)
const issueNote = ref('')
const issuing = ref(false)
const issued = ref<{ token: string } | null>(null)

function flashSuccess(msg: string) {
  successMsg.value = msg
  setTimeout(() => { if (successMsg.value === msg) successMsg.value = '' }, 2500)
}

function fail(e: any, fallback: string) {
  error.value = e?.response?.data?.error || e?.message || fallback
}

async function loadTokens(skipCache = false) {
  tokensLoading.value = true
  try {
    const res = await api.get('/admin/tokens', { cache: skipCache ? { skipCache: true } : undefined })
    tokens.value = Array.isArray(res.data?.data) ? res.data.data : []
  } catch (e: any) {
    fail(e, 'Failed to load tokens')
    tokens.value = []
  } finally {
    tokensLoading.value = false
  }
}

async function issueToken() {
  if (!Number.isInteger(issueDays.value) || issueDays.value <= 0) { error.value = 'Days must be a positive integer'; return }
  if (!(issueGb.value > 0)) { error.value = 'Traffic must be a positive number'; return }
  issuing.value = true
  error.value = ''
  issued.value = null
  try {
    const res = await api.post('/admin/tokens', {
      duration_days: issueDays.value,
      traffic_limit_gb: issueGb.value,
      note: issueNote.value.trim() || undefined,
    })
    issued.value = res.data
    issueNote.value = ''
    await loadTokens(true)
  } catch (e: any) {
    fail(e, 'Failed to issue token')
  } finally {
    issuing.value = false
  }
}

function copyIssuedUrl(format: SubscriptionFormat) {
  if (!issued.value?.token) return
  const url = buildSubscriptionUrl(issued.value.token, format)
  if (!url) return
  navigator.clipboard.writeText(url)
    .then(() => flashSuccess(`${format === 'clash' ? 'Clash' : 'Base64'} subscription URL copied`))
    .catch(() => { error.value = 'Failed to copy to clipboard' })
}

function usagePct(t: TokenRow): number {
  if (!t.traffic_limit_bytes) return 0
  return Math.min(100, Math.round((t.traffic_used_bytes / t.traffic_limit_bytes) * 100))
}

function formatBytes(bytes?: number): string {
  const b = bytes ?? 0
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
  if (b >= 1048576) return (b / 1048576).toFixed(0) + ' MB'
  return b + ' B'
}

async function revokeToken(t: TokenRow) {
  if (!confirm(`Revoke "${t.note || t.username}"? Its subscription URLs stop working immediately.`)) return
  error.value = ''
  try {
    await api.post(`/admin/tokens/${t.id}/revoke`)
    await loadTokens(true)
  } catch (e: any) {
    fail(e, 'Failed to revoke token')
  }
}

async function renewToken(t: TokenRow) {
  const days = parseInt(prompt('Renew for how many days?', '30') ?? '', 10)
  if (!Number.isInteger(days) || days <= 0) { error.value = 'Invalid days'; return }
  const gb = Number(prompt('Traffic limit (GB)?', '50'))
  if (!Number.isFinite(gb) || gb <= 0) { error.value = 'Invalid traffic limit'; return }
  error.value = ''
  try {
    const res = await api.post(`/admin/tokens/${t.id}/renew`, { duration_days: days, traffic_limit_gb: gb })
    issued.value = res.data
    flashSuccess('Token renewed — new token shown above, shown only once.')
    await loadTokens(true)
  } catch (e: any) {
    fail(e, 'Failed to renew token')
  }
}

// ── User Tokens（既有）──────────────────────────────────────────────────────

const search = ref('')
const users = ref<User[]>([])
const loading = ref(false)
const error = ref('')
const successMsg = ref('')
const regeneratingId = ref<number | null>(null)

function maskToken(token?: string): string {
  if (!token || token.length < 12) return token || '—'
  return token.substring(0, 7) + '***' + token.substring(token.length - 4)
}

function formatDate(v?: string | number): string {
  if (v === undefined || v === null || v === 0) return '—'
  try {
    const d = typeof v === 'number' ? new Date(v * 1000) : new Date(v)
    return d.toISOString().split('T')[0]
  } catch {
    return '—'
  }
}

async function copySubUrl(u: User, format: SubscriptionFormat) {
  const url = buildSubscriptionUrl(u.client_token || '', format)
  if (!url) return
  try {
    await navigator.clipboard.writeText(url)
    const label = format === 'clash' ? 'Clash' : 'Base64'
    flashSuccess(`${label} subscription URL copied for ${u.username}`)
  } catch {
    error.value = 'Failed to copy to clipboard'
  }
}

// `skipCache` is set by the explicit Refresh button so a manual refresh
// always talks to the server instead of reusing the TTL cache.
async function loadUsers(skipCache = false) {
  loading.value = true
  error.value = ''
  successMsg.value = ''
  try {
    const res = await api.get('/admin/users', { cache: skipCache ? { skipCache: true } : undefined })
    users.value = Array.isArray(res.data) ? res.data :
                  Array.isArray(res.data.data) ? res.data.data : []
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to load users'
    users.value = []
  } finally {
    loading.value = false
  }
}

async function regenerateToken(u: User) {
  if (!confirm(`Rotate token for "${u.username}"? Existing subscription URLs stop working immediately.`)) return
  regeneratingId.value = u.id
  error.value = ''
  successMsg.value = ''
  try {
    const res = await api.put(`/admin/users/${u.id}`, { regenerate_token: true })
    const fresh = res.data as User
    if (fresh?.client_token) u.client_token = fresh.client_token
    flashSuccess(`Token rotated for ${u.username}`)
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to rotate token'
  } finally {
    regeneratingId.value = null
  }
}

const filteredUsers = computed(() => {
  const q = search.value.toLowerCase()
  return users.value.filter((u: User) =>
    u.username?.toLowerCase().includes(q)
  )
})

onMounted(() => { loadUsers(); loadTokens() })
</script>

<style scoped>
.tokens-page { min-height: 100vh; background: #12141a; color: #e0e0e0; }
.hint { margin: 0 2rem 0.5rem; color: #888; font-size: 0.85rem; line-height: 1.4; }
.issue-card { margin: 0 2rem 1rem; padding: 1rem 1.25rem; border: 1px solid #2a2d35; border-radius: 8px; background: #1a1d24; }
.issue-card h3 { margin: 0 0 0.25rem; font-size: 1rem; }
.issue-card .hint { margin: 0 0 0.75rem; padding: 0; }
.issue-form { display: flex; flex-wrap: wrap; gap: 0.75rem; align-items: end; }
.issue-form label { display: flex; flex-direction: column; gap: 0.25rem; font-size: 0.8rem; color: #888; }
.issue-form input { padding: 0.4rem 0.6rem; border: 1px solid #444; border-radius: 6px; background: #12141a; color: #e0e0e0; outline: none; min-width: 140px; }
.issue-form input:focus { border-color: #4a9eff; }
.issued-box { margin-top: 0.75rem; padding: 0.75rem; border: 1px dashed #4a9eff; border-radius: 6px; background: #12141a; }
.issued-box .hint { margin: 0 0 0.4rem; padding: 0; }
.issued-token { display: block; word-break: break-all; font-size: 0.85rem; color: #4a9eff; margin-bottom: 0.5rem; }
.issued-actions { display: flex; gap: 0.5rem; }
.user-tokens-header { margin-top: 1.5rem; }
.search-input { padding: 0.45rem 0.75rem; border: 1px solid #444; border-radius: 6px; background: #1e2028; color: #e0e0e0; outline: none; min-width: 200px; }
.search-input:focus { border-color: #4a9eff; }
.btn-sm { padding: 0.45rem 0.9rem; border: 1px solid #4a9eff; border-radius: 6px; background: transparent; color: #4a9eff; cursor: pointer; font-size: 0.85rem; }
.btn-sm:hover { background: #4a9eff22; }
.btn-sm:disabled { opacity: 0.5; cursor: not-allowed; }
.loading { padding: 3rem; text-align: center; color: #888; }
.error-msg { padding: 1rem 2rem; color: #ff6b6b; background: #2a1515; margin: 1rem 2rem; border-radius: 8px; }
.success-msg { padding: 1rem 2rem; color: #4caf50; background: #1a3a1a; margin: 1rem 2rem; border-radius: 8px; }
.table-wrap { padding: 1.5rem 2rem; flex: 1; overflow-x: auto; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th { text-align: left; padding: 0.75rem 0.5rem; color: #888; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid #2a2d35; }
.data-table td { padding: 0.75rem 0.5rem; border-bottom: 1px solid #22252b; font-size: 0.9rem; }
.data-table tr:hover td { background: #1a1d2322; }
.data-table tr:last-child td { border-bottom: none; }
.empty-row { text-align: center; color: #555; padding: 3rem 0 !important; }
.status { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; font-size: 0.8rem; text-transform: capitalize; }
.status.active { background: #1a3a1a; color: #4caf50; }
.status.inactive { background: #2a2d35; color: #888; }
.status.suspended, .status.disabled { background: #3a1a1a; color: #ff6b6b; }
.usage { display: flex; flex-direction: column; gap: 0.25rem; min-width: 140px; }
.usage-bar { height: 6px; border-radius: 3px; background: #2a2d35; overflow: hidden; }
.usage-fill { height: 100%; background: #4a9eff; }
.usage-text { font-size: 0.75rem; color: #888; }
.token-text { font-size: 0.8rem; color: #4a9eff; background: #1e2028; padding: 0.15rem 0.4rem; border-radius: 3px; }
.date-cell { color: #888; font-size: 0.85rem; }
.actions-cell { white-space: nowrap; }
.btn-tiny { padding: 0.2rem 0.5rem; border: 1px solid #444; border-radius: 4px; background: transparent; color: #aaa; cursor: pointer; font-size: 0.75rem; margin: 0 0.15rem; }
.btn-tiny:hover { border-color: #4a9eff; color: #4a9eff; }
.btn-tiny:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-sub { border-color: #3d6a9e; color: #7eb6ff; }
.btn-sub:hover:not(:disabled) { border-color: #4a9eff; color: #4a9eff; background: #4a9eff18; }
.btn-danger { border-color: #6a3d3d; color: #ff8f8f; }
.btn-danger:hover:not(:disabled) { border-color: #ff6b6b; color: #ff6b6b; }
</style>
