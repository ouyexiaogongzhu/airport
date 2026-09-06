<template>
  <div class="page dashboard">
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
      <h2>Welcome back, {{ auth.username }}!</h2>
      <p class="greeting">Here's what's happening with your account today.</p>

      <div v-if="loading" class="loading">Loading…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <div class="cards">
        <div class="card">
          <h3>Traffic Used</h3>
          <span class="num">{{ formatBytes(profile.traffic_used_bytes) || '0 B' }}</span>
          <span class="sub">of {{ formatBytes(profile.traffic_limit_bytes) || '—' }}</span>
          <div v-if="profile.traffic_limit_bytes > 0" class="mini-bar">
            <div class="mini-fill" :style="{ width: trafficPercent + '%' }"></div>
          </div>
        </div>
        <div class="card">
          <h3>Subscription</h3>
          <span :class="['num-badge', statusClass]">{{ profile.subscription_status || '—' }}</span>
          <span class="sub">{{ profile.subscription_tier || 'No tier' }}</span>
        </div>
        <div class="card">
          <h3>Active Nodes</h3>
          <span class="num">{{ nodes.length }}</span>
          <span class="sub">available servers</span>
        </div>
      </div>

      <div class="action-row">
        <router-link to="/subscription" class="btn-primary">Manage Subscription</router-link>
        <router-link to="/account/guide" class="btn-secondary">Import Guide</router-link>
      </div>

      <section class="recent">
        <h3>Subscription Details</h3>
        <div v-if="profile.subscription_status" class="detail-row">
          <span class="detail-label">Expires</span>
          <span class="detail-value">{{ formatExpiry(profile.expire_time) }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">Traffic Remaining</span>
          <span class="detail-value">{{ formatTrafficRemaining() }}</span>
        </div>
        <div v-if="profile.traffic_used_bytes > 0" class="detail-row">
          <span class="detail-label">Daily Avg</span>
          <span class="detail-value">{{ formatDailyAvg() }}</span>
        </div>
        <div v-if="!profile.subscription_status" class="empty-state">
          <p>No subscription active. Choose a plan to get started.</p>
          <router-link to="/products" class="btn-primary">Browse Plans</router-link>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api/index'

const auth = useAuthStore()
const loading = ref(true)
const error = ref('')
const profile = ref<any>({})
const nodes = ref<string[]>([])

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

function formatTrafficRemaining(): string {
  const limit = profile.value.traffic_limit_bytes || 0
  const used = profile.value.traffic_used_bytes || 0
  return formatBytes(Math.max(0, limit - used))
}

function formatDailyAvg(): string {
  const used = profile.value.traffic_used_bytes || 0
  const start = profile.value.traffic_period_start || 0
  if (!start) return formatBytes(used)
  const days = Math.max(1, (Date.now() / 1000 - start) / 86400)
  return formatBytes(Math.round(used / days)) + '/day'
}

async function fetchData() {
  loading.value = true
  try {
    const [profRes, nodesRes] = await Promise.all([
      api.get('/user/profile'),
      api.get('/client/subscription').catch(() => ({ data: { nodes: [] } }))
    ])
    profile.value = profRes.data
    nodes.value = nodesRes.data.nodes || []
  } catch (e: any) {
    console.error('Dashboard load error:', e)
    error.value = e.response?.data?.error || 'Failed to load dashboard data'
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)
</script>

<style scoped>
.dashboard {
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
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem;
}
h2 { margin: 0; font-size: 1.5rem; color: #e2e8f0; }
.greeting { color: #a0aec0; margin: 0.25rem 0 2rem; }
.cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.25rem;
  margin-bottom: 2rem;
}
.card {
  background: #16213e;
  border: 1px solid #0f3460;
  border-radius: 10px;
  padding: 1.5rem;
}
.card h3 { margin: 0 0 0.5rem; font-size: 0.85rem; color: #a0aec0; text-transform: uppercase; letter-spacing: 0.5px; }
.card .num { font-size: 2rem; font-weight: 700; color: #e2e8f0; display: block; }
.card .sub { font-size: 0.8rem; color: #718096; display: block; margin-top: 0.15rem; }
.num-badge {
  font-size: 1.3rem;
  font-weight: 700;
  display: inline-block;
  padding: 0.2rem 0.8rem;
  border-radius: 8px;
}
.num-badge.active { background: #1b4332; color: #68d391; }
.num-badge.pending { background: #7b5e1e; color: #f6e05e; }
.num-badge.expired { background: #4a1c1c; color: #fc8181; }

.mini-bar { margin-top: 0.5rem; height: 4px; background: #0f3460; border-radius: 2px; overflow: hidden; }
.mini-fill { height: 100%; background: #1a73e8; border-radius: 2px; }

.action-row { display: flex; gap: 0.75rem; margin-bottom: 2rem; }
.btn-primary {
  background: #1a73e8;
  border: none;
  color: white;
  padding: 0.6rem 1.2rem;
  border-radius: 6px;
  font-size: 0.9rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
}
.btn-secondary {
  background: transparent;
  border: 1px solid #0f3460;
  color: #a0aec0;
  padding: 0.6rem 1.2rem;
  border-radius: 6px;
  font-size: 0.9rem;
  text-decoration: none;
}

.recent {
  background: #16213e;
  border: 1px solid #0f3460;
  border-radius: 10px;
  padding: 1.5rem;
}
.recent h3 { margin: 0 0 1rem; font-size: 1rem; color: #e2e8f0; }
.detail-row {
  display: flex;
  justify-content: space-between;
  padding: 0.6rem 0;
  border-bottom: 1px solid #0f3460;
}
.detail-row:last-child { border-bottom: none; }
.detail-label { color: #a0aec0; font-size: 0.9rem; }
.detail-value { color: #e2e8f0; font-weight: 500; }

.loading { color: #a0aec0; text-align: center; padding: 2rem; }
.error-msg { color: #fc8181; background: #4a1c1c; border-radius: 8px; padding: 0.75rem 1rem; margin-bottom: 1rem; font-size: 0.9rem; }
.empty-state { text-align: center; padding: 1rem; color: #a0aec0; }
.empty-state p { margin-bottom: 1rem; }
</style>
