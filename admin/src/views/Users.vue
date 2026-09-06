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
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
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
</style>
