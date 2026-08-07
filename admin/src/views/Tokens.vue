<template>
  <div class="page tokens-page">
    <main class="main">
      <header class="topbar">
        <h2>User Tokens</h2>
        <div class="topbar-right">
          <input v-model="search" placeholder="Search by username…" class="search-input" />
          <button class="btn-sm" @click="loadUsers">🔄 Refresh</button>
        </div>
      </header>

      <div v-if="loading" class="loading">Loading users…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>
      <div v-if="successMsg" class="success-msg">{{ successMsg }}</div>

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
                <button class="btn-tiny" @click="copyToken(u.client_token)" title="Copy token">📋</button>
              </td>
              <td><span :class="['status', u.status]">{{ u.status }}</span></td>
              <td class="date-cell">{{ formatDate(u.created_at) }}</td>
              <td class="actions-cell">
                <button class="btn-tiny" @click="regenerateToken(u)" :disabled="regeneratingId === u.id">
                  {{ regeneratingId === u.id ? '⟳…' : '🔄 Regenerate' }}
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

interface User {
  id: number
  username: string
  client_token: string
  status: string
  role: string
  subscription_status: string
  created_at: string
}

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

function formatDate(dateStr?: string): string {
  if (!dateStr) return '—'
  try {
    return new Date(dateStr).toISOString().split('T')[0]
  } catch {
    return '—'
  }
}

async function copyToken(token?: string) {
  if (!token) return
  try {
    await navigator.clipboard.writeText(token)
    successMsg.value = 'Token copied to clipboard'
    setTimeout(() => { successMsg.value = '' }, 2000)
  } catch {
    // fallback
  }
}

function generateToken(): string {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return 'rf_' + Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('')
}

async function loadUsers() {
  loading.value = true
  error.value = ''
  successMsg.value = ''
  try {
    const res = await api.get('/admin/users')
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
  if (!confirm(`Regenerate token for user "${u.username}"? The current token will stop working immediately.`)) return
  regeneratingId.value = u.id
  error.value = ''
  successMsg.value = ''
  try {
    const newToken = generateToken()
    await api.put(`/admin/users/${u.id}`, { client_token: newToken })
    u.client_token = newToken
    successMsg.value = `Token regenerated for ${u.username}`
    setTimeout(() => { successMsg.value = '' }, 3000)
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to regenerate token'
  } finally {
    regeneratingId.value = null
  }
}

const filteredUsers = computed(() =>
  users.value.filter((u: User) =>
    u.username?.toLowerCase().includes(search.value.toLowerCase())
  )
)

onMounted(loadUsers)
</script>

<style scoped>
.tokens-page { min-height: 100vh; background: #12141a; color: #e0e0e0; }
.search-input { padding: 0.45rem 0.75rem; border: 1px solid #444; border-radius: 6px; background: #1e2028; color: #e0e0e0; outline: none; min-width: 200px; }
.search-input:focus { border-color: #4a9eff; }
.btn-sm { padding: 0.45rem 0.9rem; border: 1px solid #4a9eff; border-radius: 6px; background: transparent; color: #4a9eff; cursor: pointer; font-size: 0.85rem; }
.btn-sm:hover { background: #4a9eff22; }
.loading { padding: 3rem; text-align: center; color: #888; }
.error-msg { padding: 1rem 2rem; color: #ff6b6b; background: #2a1515; margin: 1rem 2rem; border-radius: 8px; }
.success-msg { padding: 1rem 2rem; color: #4caf50; background: #1a3a1a; margin: 1rem 2rem 0; border-radius: 8px; }
.table-wrap { padding: 1.5rem 2rem; flex: 1; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th { text-align: left; padding: 0.75rem 0.5rem; color: #888; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid #2a2d35; }
.data-table td { padding: 0.75rem 0.5rem; border-bottom: 1px solid #22252b; font-size: 0.9rem; }
.data-table tr:hover td { background: #1a1d2322; }
.data-table tr:last-child td { border-bottom: none; }
.empty-row { text-align: center; color: #555; padding: 3rem 0 !important; }
.status { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; font-size: 0.8rem; text-transform: capitalize; }
.status.active { background: #1a3a1a; color: #4caf50; }
.status.inactive { background: #2a2d35; color: #888; }
.status.disabled { background: #3a1a1a; color: #ff6b6b; }
.token-text { font-size: 0.8rem; color: #4a9eff; background: #1e2028; padding: 0.15rem 0.4rem; border-radius: 3px; }
.date-cell { color: #888; font-size: 0.85rem; }
.actions-cell { white-space: nowrap; }
.btn-tiny { padding: 0.2rem 0.5rem; border: 1px solid #444; border-radius: 4px; background: transparent; color: #aaa; cursor: pointer; font-size: 0.75rem; margin: 0 0.15rem; }
.btn-tiny:hover { border-color: #4a9eff; color: #4a9eff; }
.btn-tiny:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
