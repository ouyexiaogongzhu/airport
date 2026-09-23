<template>
  <div class="page devices-page">
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
      <h2>Devices</h2>
      <p class="subtitle">
        Subscription clients that have imported your link. Limit:
        <template v-if="maxDevices === 0">unlimited</template>
        <template v-else>{{ used }} / {{ maxDevices }}</template>.
      </p>

      <div v-if="loading" class="loading">Loading devices…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <section v-if="!loading" class="card-section">
        <div v-if="devices.length === 0" class="empty">
          No devices yet. Import your subscription link on a client to register one.
        </div>
        <ul v-else class="device-list">
          <li v-for="d in devices" :key="d.id" class="device-item">
            <div class="device-main">
              <span class="device-name">{{ d.device_name }}</span>
              <span class="device-meta">{{ d.platform }} · last seen {{ formatSeen(d.last_seen) }}</span>
            </div>
            <button
              class="btn-danger"
              :disabled="revokingId === d.id"
              @click="confirmRevoke(d)"
            >
              {{ revokingId === d.id ? 'Revoking…' : 'Revoke' }}
            </button>
          </li>
        </ul>
        <p class="hint">
          Revoking frees a slot. The client must re-import the subscription to reconnect after a new pull.
          Live Xray sessions are not cut instantly.
        </p>
      </section>
    </main>

    <div v-if="pending" class="modal-overlay" @click.self="pending = null">
      <div class="modal">
        <h4>Revoke device?</h4>
        <p>
          Remove <strong>{{ pending.device_name }}</strong> from your account?
          You can free a slot if you hit the device limit.
        </p>
        <div class="modal-actions">
          <button class="btn-outline" @click="pending = null">Cancel</button>
          <button class="btn-danger" @click="doRevoke">Revoke</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api/index'

type Device = {
  id: number
  device_name: string
  platform: string
  last_seen: number
  created_at?: number
  is_current?: boolean
}

const auth = useAuthStore()
const loading = ref(true)
const error = ref('')
const devices = ref<Device[]>([])
const maxDevices = ref(5)
const used = ref(0)
const revokingId = ref<number | null>(null)
const pending = ref<Device | null>(null)

function formatSeen(ts: number): string {
  if (!ts) return '—'
  const diff = Math.floor(Date.now() / 1000) - ts
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}d ago`
  return new Date(ts * 1000).toLocaleDateString()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/client/devices')
    maxDevices.value = res.data.max_devices ?? 5
    used.value = res.data.used ?? 0
    devices.value = res.data.devices ?? []
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Failed to load devices'
  } finally {
    loading.value = false
  }
}

function confirmRevoke(d: Device) {
  pending.value = d
}

async function doRevoke() {
  const d = pending.value
  if (!d) return
  pending.value = null
  revokingId.value = d.id
  error.value = ''
  try {
    await api.delete(`/client/devices/${d.id}`)
    await load()
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Failed to revoke device'
  } finally {
    revokingId.value = null
  }
}

onMounted(load)
</script>

<style scoped>
.page { min-height: 100vh; background: #1a1a2e; color: #e0e0e0; }
.topbar {
  display: flex; align-items: center; gap: 1.5rem;
  padding: 0.75rem 1.5rem; background: #16213e; border-bottom: 1px solid #0f3460;
}
.brand { font-weight: 700; color: #e94560; font-size: 1.1rem; }
.nav-links { display: flex; gap: 1rem; flex: 1; flex-wrap: wrap; }
.nav-links a { color: #a0a0b0; text-decoration: none; font-size: 0.9rem; }
.nav-links a.router-link-active { color: #e94560; }
.nav-links a:hover { color: #e0e0e0; }
.user-badge {
  background: rgba(233,69,96,0.15); color: #e94560;
  padding: 0.3rem 0.8rem; border-radius: 20px; font-size: 0.8rem; font-weight: 600;
}
.content { max-width: 800px; margin: 0 auto; padding: 2rem; }
h2 { margin: 0; font-size: 1.5rem; color: #f0f0f0; }
.subtitle { color: #a0a0b0; margin: 0.25rem 0 1.5rem; font-size: 0.9rem; }
.loading { color: #a0a0b0; font-size: 0.9rem; padding: 1rem 0; }
.error-msg {
  color: #ff6b6b; background: rgba(255,107,107,0.1);
  border-radius: 8px; padding: 0.75rem 1rem; margin-bottom: 1rem; font-size: 0.9rem;
}
.card-section {
  background: #16213e; border-radius: 12px; padding: 1.5rem; margin-bottom: 1.5rem;
}
.empty { color: #a0a0b0; font-size: 0.9rem; padding: 0.5rem 0; }
.device-list { list-style: none; margin: 0; padding: 0; display: grid; gap: 0.75rem; }
.device-item {
  display: flex; align-items: center; justify-content: space-between; gap: 1rem;
  padding: 0.85rem 1rem; background: #0f3460; border-radius: 8px;
}
.device-main { display: flex; flex-direction: column; gap: 0.2rem; min-width: 0; }
.device-name { color: #f0f0f0; font-weight: 600; font-size: 0.95rem; }
.device-meta { color: #a0a0b0; font-size: 0.8rem; }
.hint { color: #718096; font-size: 0.8rem; margin: 1rem 0 0; line-height: 1.4; }
.btn-outline {
  padding: 0.4rem 0.9rem; background: transparent; border: 1px solid #0f3460;
  border-radius: 6px; color: #e0e0e0; cursor: pointer; font-size: 0.85rem;
}
.btn-outline:hover { border-color: #e94560; color: #e94560; }
.btn-danger {
  padding: 0.4rem 0.9rem; background: rgba(244,67,54,0.2); border: 1px solid #e57373;
  border-radius: 6px; color: #e57373; cursor: pointer; font-size: 0.85rem; white-space: nowrap;
}
.btn-danger:hover:not(:disabled) { background: rgba(244,67,54,0.3); }
.btn-danger:disabled { opacity: 0.6; cursor: wait; }
.modal-overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.6);
  display: flex; align-items: center; justify-content: center; z-index: 100;
}
.modal {
  background: #16213e; border-radius: 12px; padding: 2rem; max-width: 420px; width: 90%;
  border: 1px solid #0f3460;
}
.modal h4 { margin: 0 0 0.75rem; color: #e57373; font-size: 1.1rem; }
.modal p { color: #a0a0b0; font-size: 0.9rem; line-height: 1.5; margin: 0 0 1.5rem; }
.modal-actions { display: flex; gap: 0.75rem; justify-content: flex-end; }
</style>
