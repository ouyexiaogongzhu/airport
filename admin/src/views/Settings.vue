<template>
  <div class="settings-page">
      <header class="topbar">
        <h2>Settings</h2>
        <div class="topbar-right">
          <button class="btn-sm" @click="loadAll(true)">🔄 Refresh</button>
        </div>
      </header>

      <div class="settings-layout">
        <!-- Left: Tab Nav -->
        <div class="settings-tabs">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            :class="['tab-btn', { active: activeTab === tab.key }]"
            @click="activeTab = tab.key"
          >
            <span class="tab-icon">{{ tab.icon }}</span>
            <span class="tab-label">{{ tab.label }}</span>
          </button>
        </div>

        <!-- Right: Tab Content -->
        <div class="settings-content">
          <div v-if="loading" class="loading">Loading settings…</div>
          <div v-if="error" class="error-msg">{{ error }}</div>

          <!-- ⚙️ General Tab -->
          <div v-if="activeTab === 'general' && !loading" class="tab-panel">
            <h3>General Configuration</h3>

            <div class="setting-section">
              <h4>API Endpoint</h4>
              <div class="setting-row">
                <span class="setting-label">Base URL</span>
                <code class="setting-value">{{ general.apiBase }}</code>
              </div>
              <div class="setting-row">
                <span class="setting-label">API Version</span>
                <code class="setting-value">{{ general.apiVersion }}</code>
              </div>
              <div class="setting-row">
                <span class="setting-label">App Version</span>
                <span class="setting-value">{{ general.appVersion }}</span>
              </div>
            </div>

            <div class="setting-section">
              <h4>Service Health</h4>
              <div class="health-grid">
                <div
                  v-for="svc in services"
                  :key="svc.name"
                  :class="['health-card', svc.status]"
                >
                  <span class="health-dot"></span>
                  <div class="health-info">
                    <span class="health-name">{{ svc.name }}</span>
                    <span class="health-status">{{ svc.status === 'healthy' ? 'Healthy' : svc.status === 'degraded' ? 'Degraded' : 'Unreachable' }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 📊 System Tab -->
          <div v-if="activeTab === 'system' && !loading" class="tab-panel">
            <h3>System Overview</h3>

            <div class="stats-grid">
              <div class="stat-card">
                <span class="stat-icon">👥</span>
                <div class="stat-body">
                  <span class="stat-num">{{ systemStats.totalUsers }}</span>
                  <span class="stat-label">Total Users</span>
                </div>
              </div>
              <div class="stat-card">
                <span class="stat-icon">🖥️</span>
                <div class="stat-body">
                  <span class="stat-num">{{ systemStats.activeNodes }}</span>
                  <span class="stat-label">Active Nodes</span>
                </div>
              </div>
              <div class="stat-card">
                <span class="stat-icon">📦</span>
                <div class="stat-body">
                  <span class="stat-num">{{ systemStats.totalProducts }}</span>
                  <span class="stat-label">Total Products</span>
                </div>
              </div>
              <div class="stat-card">
                <span class="stat-icon">💾</span>
                <div class="stat-body">
                  <span class="stat-num">{{ systemStats.dbSize }}</span>
                  <span class="stat-label">Database Size</span>
                </div>
              </div>
              <div class="stat-card">
                <span class="stat-icon">⏱️</span>
                <div class="stat-body">
                  <span class="stat-num">{{ systemStats.uptime }}</span>
                  <span class="stat-label">System Uptime</span>
                </div>
              </div>
            </div>

            <div class="setting-section">
              <h4>Traffic Overview</h4>
              <div class="traffic-summary">
                <div class="traffic-item">
                  <span class="traffic-label">Total Traffic Today</span>
                  <span class="traffic-val">{{ systemStats.todayTraffic }}</span>
                </div>
                <div class="traffic-item">
                  <span class="traffic-label">Total Traffic This Month</span>
                  <span class="traffic-val">{{ systemStats.monthTraffic }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 🔒 Security Tab -->
          <div v-if="activeTab === 'security' && !loading" class="tab-panel">
            <h3>Security Configuration</h3>

            <div class="setting-section">
              <h4>JWT Authentication</h4>
              <div class="setting-row">
                <span class="setting-label">Algorithm</span>
                <code class="setting-value">{{ security.jwtAlgorithm }}</code>
              </div>
              <div class="setting-row">
                <span class="setting-label">Token Expiry</span>
                <span class="setting-value">{{ security.jwtExpiry }}</span>
              </div>
              <div class="setting-row">
                <span class="setting-label">Issuer</span>
                <code class="setting-value">{{ security.jwtIssuer }}</code>
              </div>
              <div class="setting-row">
                <span class="setting-label">Key Rotation</span>
                <span class="setting-value">{{ security.jwtKeyRotation }}</span>
              </div>
            </div>

            <div class="setting-section">
              <h4>CORS</h4>
              <div class="cors-list">
                <div v-for="(origin, i) in security.corsOrigins" :key="i" class="cors-origin">
                  <code>{{ origin }}</code>
                </div>
                <div v-if="security.corsOrigins.length === 0" class="cors-origin empty">
                  <code>— No origins configured —</code>
                </div>
              </div>
            </div>

            <div class="setting-section">
              <h4>Rate Limiting</h4>
              <div class="setting-row">
                <span class="setting-label">Requests / min</span>
                <span class="setting-value">{{ security.rateLimit }}</span>
              </div>
              <div class="setting-row">
                <span class="setting-label">Login Attempts</span>
                <span class="setting-value">{{ security.loginAttempts }} before lockout</span>
              </div>
            </div>
          </div>
        </div>
      </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import api from '../api/index'

interface Tab { key: string; label: string; icon: string }

const tabs: Tab[] = [
  { key: 'general', label: 'General', icon: '⚙️' },
  { key: 'system',  label: 'System',  icon: '📊' },
  { key: 'security', label: 'Security', icon: '🔒' },
]

const activeTab = ref('general')
const loading = ref(false)
const error = ref('')

interface ServiceHealth {
  name: string
  status: 'healthy' | 'degraded' | 'unreachable'
}

interface SystemStats {
  totalUsers: number
  activeNodes: number
  totalProducts: number
  dbSize: string
  uptime: string
  todayTraffic: string
  monthTraffic: string
}

interface SecurityConfig {
  jwtAlgorithm: string
  jwtExpiry: string
  jwtIssuer: string
  jwtKeyRotation: string
  corsOrigins: string[]
  rateLimit: string
  loginAttempts: number
}

const general = reactive({
  apiBase: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  // TODO: Read apiVersion from env or package.json instead of hardcoding
  apiVersion: 'v1.0.0',
  appVersion: '0.0.1',
})

// Real health check against the Worker's /health endpoint (baseURL minus /api/v1).
const services = ref<ServiceHealth[]>([
  { name: 'API', status: 'unreachable' },
])

const systemStats = reactive<SystemStats>({
  totalUsers: 0,
  activeNodes: 0,
  totalProducts: 0,
  dbSize: '—',
  uptime: '—',
  todayTraffic: '—',
  monthTraffic: '—',
})

const security = reactive<SecurityConfig>({
  jwtAlgorithm: 'HS256',
  jwtExpiry: '24h',
  jwtIssuer: 'rfplay-admin',
  jwtKeyRotation: 'Every 90 days',
  corsOrigins: ['*'],
  rateLimit: '60 / min',
  loginAttempts: 5,
})

async function loadUsersCount(skipCache = false): Promise<number> {
  try {
    const res = await api.get('/admin/users', {
      params: { limit: 1 },
      cache: skipCache ? { skipCache: true } : undefined,
    })
    return res.data?.total ?? res.data?.users?.length ?? 0
  } catch {
    return 0
  }
}

async function loadNodesCount(skipCache = false): Promise<number> {
  try {
    const res = await api.get('/admin/nodes', { cache: skipCache ? { skipCache: true } : undefined })
    const data = res.data
    const nodes: any[] = Array.isArray(data) ? data : (Array.isArray(data.data) ? data.data : [])
    return nodes.filter(n => n.status === 'active').length
  } catch {
    return 0
  }
}

// /admin/traffic/stats returns { data: [{ node_id, user_id, total_upload, total_download }] }
// aggregated over the since/until window — sum the rows for today / this month.
async function loadTrafficStats(skipCache = false) {
  const dayStart = new Date()
  dayStart.setHours(0, 0, 0, 0)
  const monthStart = new Date()
  monthStart.setDate(1)
  monthStart.setHours(0, 0, 0, 0)
  const opts = (since: Date) => ({
    params: { since: since.toISOString() },
    cache: skipCache ? { skipCache: true } : undefined,
  })
  const sum = (res: any) =>
    (res.data?.data ?? []).reduce(
      (acc: number, r: any) => acc + Number(r.total_upload ?? 0) + Number(r.total_download ?? 0),
      0,
    )
  try {
    const [today, month] = await Promise.all([
      api.get('/admin/traffic/stats', opts(dayStart)),
      api.get('/admin/traffic/stats', opts(monthStart)),
    ])
    systemStats.todayTraffic = formatBytes(sum(today))
    systemStats.monthTraffic = formatBytes(sum(month))
  } catch {
    // keep '—' placeholders
  }
}

async function checkHealth() {
  try {
    const healthURL = (general.apiBase.replace(/\/+$/, '').replace(/\/api\/v1$/, '') || '') + '/health'
    const res = await api.get(healthURL, { cache: { skipCache: true } })
    services.value[0].status = res.data?.status === 'ok' ? 'healthy' : 'degraded'
  } catch {
    services.value[0].status = 'unreachable'
  }
}

async function loadProductsCount(skipCache = false): Promise<number> {
  try {
    const res = await api.get('/admin/products', { cache: skipCache ? { skipCache: true } : undefined })
    const products: any[] = Array.isArray(res.data?.products) ? res.data.products : []
    return products.length
  } catch {
    return 0
  }
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024
    i++
  }
  return val.toFixed(i > 0 ? 2 : 0) + ' ' + units[i]
}

// `skipCache` is set by the explicit Refresh button so a manual refresh
// always talks to the server instead of reusing the TTL cache.
async function loadAll(skipCache = false) {
  loading.value = true
  error.value = ''

  // Try hitting a health/status endpoint
  try {
    const [usersCount, nodesCount, productsCount] = await Promise.all([
      loadUsersCount(skipCache),
      loadNodesCount(skipCache),
      loadProductsCount(skipCache),
      loadTrafficStats(skipCache),
      checkHealth(),
    ])

    systemStats.totalUsers = usersCount
    systemStats.activeNodes = nodesCount
    systemStats.totalProducts = productsCount
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to load some settings'
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
.settings-page { min-height: 100vh; background: #12141a; color: #e0e0e0; }
.btn-sm { padding: 0.45rem 0.9rem; border: 1px solid #4a9eff; border-radius: 6px; background: transparent; color: #4a9eff; cursor: pointer; font-size: 0.85rem; }
.btn-sm:hover { background: #4a9eff22; }
.loading { padding: 3rem; text-align: center; color: #888; }
.error-msg { padding: 1rem 2rem; color: #ff6b6b; background: #2a1515; margin: 1rem 2rem; border-radius: 8px; }

/* Settings layout: tabs on left, content on right */
.settings-layout {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.settings-tabs {
  width: 180px;
  background: #1a1d23;
  border-right: 1px solid #2a2d35;
  padding: 1rem 0;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  width: 100%;
  padding: 0.7rem 1.25rem;
  background: transparent;
  border: none;
  color: #888;
  font-size: 0.9rem;
  cursor: pointer;
  text-align: left;
  transition: 0.15s;
}
.tab-btn:hover { color: #fff; background: #2a2d3555; }
.tab-btn.active { color: #4a9eff; background: #4a9eff15; border-right: 2px solid #4a9eff; }
.tab-icon { font-size: 1rem; }
.tab-label { font-weight: 500; }

.settings-content {
  flex: 1;
  padding: 2rem;
  overflow-y: auto;
}

.tab-panel h3 {
  color: #fff;
  font-size: 1.15rem;
  margin: 0 0 1.5rem;
}

.setting-section {
  margin-bottom: 2rem;
  background: #1a1d23;
  border: 1px solid #2a2d35;
  border-radius: 10px;
  padding: 1.25rem;
}
.setting-section h4 {
  color: #aaa;
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin: 0 0 1rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid #2a2d35;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.6rem 0;
}
.setting-row + .setting-row {
  border-top: 1px solid #22252b;
}
.setting-label {
  color: #ccc;
  font-size: 0.9rem;
}
.setting-value {
  color: #e0e0e0;
  font-size: 0.9rem;
}
.setting-value code,
code.setting-value {
  background: #12141a;
  padding: 0.2rem 0.6rem;
  border-radius: 4px;
  font-size: 0.85rem;
  color: #4a9eff;
}

/* Health cards */
.health-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 0.75rem;
}
.health-card {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.85rem 1rem;
  border-radius: 8px;
  background: #12141a;
  border: 1px solid #2a2d35;
}
.health-card.healthy { border-color: #1a4a1a; }
.health-card.degraded { border-color: #4a4a1a; }
.health-card.unreachable { border-color: #4a1a1a; }
.health-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.health-card.healthy .health-dot { background: #4caf50; box-shadow: 0 0 6px #4caf5066; }
.health-card.degraded .health-dot { background: #ffc107; box-shadow: 0 0 6px #ffc10766; }
.health-card.unreachable .health-dot { background: #ff6b6b; box-shadow: 0 0 6px #ff6b6b66; }
.health-info { display: flex; flex-direction: column; }
.health-name { color: #e0e0e0; font-size: 0.9rem; font-weight: 500; }
.health-status { color: #888; font-size: 0.78rem; }

/* Stats grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1rem;
  margin-bottom: 2rem;
}
.stat-card {
  background: #1a1d23;
  border: 1px solid #2a2d35;
  border-radius: 10px;
  padding: 1.25rem;
  display: flex;
  align-items: center;
  gap: 1rem;
}
.stat-icon { font-size: 1.8rem; }
.stat-body { display: flex; flex-direction: column; }
.stat-num { font-size: 1.5rem; font-weight: 700; color: #fff; }
.stat-card .stat-label { font-size: 0.78rem; color: #888; text-transform: uppercase; letter-spacing: 0.3px; margin-top: 0.2rem; }

/* Traffic summary */
.traffic-summary {
  display: flex;
  gap: 1.5rem;
}
.traffic-item {
  flex: 1;
}
.traffic-label { display: block; font-size: 0.8rem; color: #888; margin-bottom: 0.3rem; }
.traffic-val { display: block; font-size: 1.1rem; color: #4a9eff; font-weight: 600; }

/* CORS list */
.cors-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.cors-origin {
  padding: 0.35rem 0.7rem;
  background: #12141a;
  border: 1px solid #2a2d35;
  border-radius: 6px;
}
.cors-origin code {
  color: #4a9eff;
  font-size: 0.85rem;
}
.cors-origin.empty code {
  color: #666;
}
</style>
