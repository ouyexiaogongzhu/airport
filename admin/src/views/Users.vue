<template>
  <div class="page users-page">
    <main class="main">
      <header class="topbar">
        <h2>Users</h2>
        <div class="topbar-right">
          <input v-model="search" placeholder="Search users…" class="search-input" />
          <button class="btn-sm" @click="loadUsers(true)">🔄 Refresh</button>
        </div>
      </header>

      <div v-if="loading" class="loading">Loading users…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <div v-if="!loading" class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Username</th>
              <th>Role</th>
              <th>Status</th>
              <th>Subscription</th>
              <th>Token</th>
              <th>Traffic Used</th>
              <th>Expires</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in filteredUsers" :key="u.id">
              <td>{{ u.id }}</td>
              <td><strong>{{ u.username }}</strong></td>
              <td><span class="tag">{{ u.role }}</span></td>
              <td><span :class="['status', u.status]">{{ u.status }}</span></td>
              <td>
                <span :class="['sub-badge', u.subscription_status || 'pending']">
                  {{ subLabel(u.subscription_status) }}
                </span>
              </td>
              <td>
                <code class="token-text">{{ maskToken(u.client_token) }}</code>
                <button class="btn-tiny" @click="copyToken(u.client_token)">📋</button>
              </td>
              <td>{{ formatBytes(u.traffic_used_bytes) }}</td>
              <td>{{ formatExpiry(u.expire_time) }}</td>
              <td class="actions-cell">
                <button class="btn-tiny" @click="toggleActive(u)" :disabled="activatingId === u.id">
                  {{ u.status === 'active' ? '⏸ Suspend' : '✅ Activate' }}
                </button>
                <button class="btn-tiny" @click="openManage(u)">⚙️ Manage</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>

    <div v-if="managing" class="modal-overlay" @click.self="managing = null">
      <div class="modal-content">
        <h3>Manage {{ managing.username }}</h3>

        <section class="modal-section">
          <label>Grant product (starts or extends the subscription)</label>
          <div class="inline-row">
            <select v-model.number="grantProductId">
              <option :value="0">-- Select product --</option>
              <option v-for="p in products" :key="p.id" :value="p.id">
                {{ p.name }} ({{ p.duration_days ?? 30 }}d)
              </option>
            </select>
            <button class="btn-sm" :disabled="!grantProductId || saving" @click="grantProduct">Grant</button>
          </div>
        </section>

        <form class="modal-section" @submit.prevent="saveManage">
          <div class="field-row">
            <div class="field">
              <label>Subscription</label>
              <select v-model="manageForm.subscription_status">
                <option value="active">active</option>
                <option value="pending">pending</option>
                <option value="expired">expired</option>
              </select>
            </div>
            <div class="field">
              <label>Expires (empty = never)</label>
              <input v-model="manageForm.expire_date" type="date" />
            </div>
          </div>
          <div class="field-row">
            <div class="field">
              <label>Traffic limit (GB, 0 = unlimited)</label>
              <input v-model.number="manageForm.traffic_limit_gb" type="number" min="0" step="1" />
            </div>
            <div class="field">
              <label>Used (GB)</label>
              <input v-model.number="manageForm.traffic_used_gb" type="number" min="0" step="0.01" />
            </div>
          </div>
          <p v-if="manageError" class="error">{{ manageError }}</p>
          <div class="modal-actions">
            <button type="button" class="btn-cancel" @click="managing = null">Close</button>
            <button type="submit" class="btn-primary" :disabled="saving">{{ saving ? 'Saving…' : 'Save' }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api/index'
const auth = useAuthStore()

const search = ref('')
const users = ref<any[]>([])
const loading = ref(false)
const error = ref('')
const activatingId = ref<number | null>(null)

function maskToken(token?: string): string {
  if (!token || token.length < 12) return '—'
  return token.substring(0, 6) + '***' + token.substring(token.length - 4)
}

function subLabel(status?: string): string {
  switch (status) {
    case 'active': return 'Active'
    case 'pending': return 'Pending'
    case 'expired': return 'Expired'
    case 'disabled': return 'Disabled'
    default: return '—'
  }
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const gb = bytes / (1024 * 1024 * 1024)
  return gb.toFixed(2) + ' GB'
}

function formatExpiry(ts?: number): string {
  if (!ts || ts <= 0) return '—'
  const d = new Date(ts * 1000)
  return d.toISOString().split('T')[0]
}

async function copyToken(token?: string) {
  if (!token) return
  try {
    await navigator.clipboard.writeText(token)
  } catch {
    // fallback
  }
}

// `skipCache` is set by the explicit Refresh button so a manual refresh
// always talks to the server instead of reusing the TTL cache.
async function loadUsers(skipCache = false) {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/admin/users', { cache: skipCache ? { skipCache: true } : undefined })
    const data = res.data
    users.value = Array.isArray(data) ? data :
                  Array.isArray(data.data) ? data.data : []
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to load users'
  } finally {
    loading.value = false
  }
}

async function toggleActive(u: any) {
  activatingId.value = u.id
  try {
    // Backend only accepts status in { active, suspended, banned }.
    const newStatus = u.status === 'active' ? 'suspended' : 'active'
    await api.put(`/admin/users/${u.id}`, { status: newStatus })
    // TODO: Direct mutation of reactive array item — should re-fetch from server instead
    u.status = newStatus
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to update'
  } finally {
    activatingId.value = null
  }
}

const GB = 1024 ** 3
const managing = ref<any | null>(null)
const products = ref<any[]>([])
const grantProductId = ref(0)
const saving = ref(false)
const manageError = ref('')
let manageInitial = { subscription_status: 'pending', expire_date: '', traffic_limit_gb: 0, traffic_used_gb: 0 }
const manageForm = ref({ ...manageInitial })

function applyUser(fresh: any) {
  const idx = users.value.findIndex((x: any) => x.id === fresh.id)
  if (idx !== -1) users.value[idx] = fresh
  managing.value = fresh
  manageInitial = {
    subscription_status: fresh.subscription_status || 'pending',
    expire_date: fresh.expire_time > 0 ? new Date(fresh.expire_time * 1000).toISOString().split('T')[0] : '',
    traffic_limit_gb: Math.round((fresh.traffic_limit_bytes || 0) / GB),
    traffic_used_gb: Number(((fresh.traffic_used_bytes || 0) / GB).toFixed(2)),
  }
  manageForm.value = { ...manageInitial }
}

async function openManage(u: any) {
  manageError.value = ''
  grantProductId.value = 0
  applyUser(u)
  if (products.value.length === 0) {
    try {
      const res = await api.get('/admin/products', { params: { per_page: 100 } })
      products.value = (res.data.products || []).filter((p: any) => p.status !== 'archived')
    } catch (e: any) {
      manageError.value = e.response?.data?.error || e.message || 'Failed to load products'
    }
  }
}

async function grantProduct() {
  if (!managing.value || !grantProductId.value) return
  saving.value = true
  manageError.value = ''
  try {
    const res = await api.post(`/admin/users/${managing.value.id}/grant`, { product_id: grantProductId.value })
    applyUser(res.data)
  } catch (e: any) {
    manageError.value = e.response?.data?.error || e.message || 'Failed to grant product'
  } finally {
    saving.value = false
  }
}

async function saveManage() {
  if (!managing.value) return
  saving.value = true
  manageError.value = ''
  const f = manageForm.value
  const init = manageInitial
  // 只提交改動的欄位：表單顯示值經過 GB 取整，原樣回寫會篡改真實用量
  const payload: Record<string, unknown> = {}
  if (f.subscription_status !== init.subscription_status) payload.subscription_status = f.subscription_status
  if (f.expire_date !== init.expire_date) {
    // 按 UTC 當天結束計算到期
    payload.expire_time = f.expire_date ? Math.floor(Date.parse(`${f.expire_date}T23:59:59Z`) / 1000) : 0
  }
  if (f.traffic_limit_gb !== init.traffic_limit_gb) payload.traffic_limit_bytes = Math.trunc((f.traffic_limit_gb || 0) * GB)
  if (f.traffic_used_gb !== init.traffic_used_gb) payload.traffic_used_bytes = Math.trunc((f.traffic_used_gb || 0) * GB)
  if (Object.keys(payload).length === 0) {
    saving.value = false
    return
  }
  try {
    const res = await api.put(`/admin/users/${managing.value.id}`, payload)
    applyUser(res.data)
  } catch (e: any) {
    manageError.value = e.response?.data?.error || e.message || 'Failed to update user'
  } finally {
    saving.value = false
  }
}

const filteredUsers = computed(() => {
  const q = search.value.toLowerCase()
  return users.value.filter((u: any) =>
    u.username?.toLowerCase().includes(q)
  )
})

onMounted(loadUsers)
</script>

<style scoped>
.search-input { padding: 0.45rem 0.75rem; border: 1px solid #444; border-radius: 6px; background: #1e2028; color: #e0e0e0; outline: none; }
.search-input:focus { border-color: #4a9eff; }
.btn-sm { padding: 0.45rem 0.9rem; border: 1px solid #4a9eff; border-radius: 6px; background: transparent; color: #4a9eff; cursor: pointer; font-size: 0.85rem; }
.btn-sm:hover { background: #4a9eff22; }
.loading { padding: 3rem; text-align: center; color: #888; }
.error-msg { padding: 1rem 2rem; color: #ff6b6b; background: #2a1515; margin: 1rem 2rem; border-radius: 8px; }
.table-wrap { padding: 1.5rem 2rem; flex: 1; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th { text-align: left; padding: 0.75rem 0.5rem; color: #888; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid #2a2d35; }
.data-table td { padding: 0.75rem 0.5rem; border-bottom: 1px solid #22252b; font-size: 0.9rem; }
.data-table tr:hover td { background: #1a1d2322; }
.tag { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; background: #2a2d35; color: #aaa; font-size: 0.8rem; }
.status { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; font-size: 0.8rem; }
.status.active { background: #1a3a1a; color: #4caf50; }
.status.suspended { background: #3a1a1a; color: #ff6b6b; }
.status.inactive { background: #2a2a1a; color: #ffa726; }
.sub-badge { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; font-size: 0.8rem; }
.sub-badge.active { background: #1a3a1a; color: #4caf50; }
.sub-badge.pending { background: #2a2a1a; color: #ffa726; }
.sub-badge.expired { background: #3a1a1a; color: #ff6b6b; }
.sub-badge.disabled { background: #2a2d35; color: #888; }
.token-text { font-size: 0.75rem; color: #4a9eff; background: #1e2028; padding: 0.1rem 0.3rem; border-radius: 3px; }
.btn-tiny { padding: 0.2rem 0.5rem; border: 1px solid #444; border-radius: 4px; background: transparent; color: #aaa; cursor: pointer; font-size: 0.75rem; margin: 0 0.15rem; }
.btn-tiny:hover { border-color: #4a9eff; color: #4a9eff; }
.actions-cell { white-space: nowrap; }
.btn-primary { padding: 0.45rem 0.9rem; background: #4a9eff; color: #fff; border: none; border-radius: 6px; cursor: pointer; font-size: 0.85rem; }
.btn-primary:disabled, .btn-sm:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-cancel { padding: 0.45rem 0.9rem; border: 1px solid #444; border-radius: 6px; background: transparent; color: #aaa; cursor: pointer; font-size: 0.85rem; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal-content { background: #1a1d23; border: 1px solid #2a2d35; border-radius: 12px; padding: 2rem; width: 100%; max-width: 560px; box-shadow: 0 8px 32px rgba(0,0,0,0.4); }
.modal-content h3 { margin: 0 0 1.5rem; color: #fff; font-size: 1.15rem; }
.modal-section { margin-bottom: 1.25rem; }
.modal-section > label, .field label { display: block; margin-bottom: 0.35rem; color: #ccc; font-size: 0.85rem; font-weight: 500; }
.inline-row { display: flex; gap: 0.75rem; }
.field { margin-bottom: 1rem; flex: 1; }
.field-row { display: flex; gap: 1rem; }
.field input, .field select, .inline-row select { width: 100%; padding: 0.55rem 0.75rem; border: 1px solid #444; border-radius: 6px; background: #12141a; color: #e0e0e0; font-size: 0.9rem; box-sizing: border-box; }
.modal-actions { display: flex; gap: 0.75rem; justify-content: flex-end; margin-top: 1rem; }
.error { color: #ff6b6b; font-size: 0.85rem; margin: 0.5rem 0; }
</style>
