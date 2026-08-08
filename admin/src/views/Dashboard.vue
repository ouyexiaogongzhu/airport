<template>
  <div class="page dashboard">
    <header class="topbar">
      <h2>Dashboard</h2>
      <span class="date">{{ today }}</span>
    </header>

    <div v-if="error" class="error-msg">{{ error }}</div>

    <div class="stats">
      <div class="stat-card">
        <span class="stat-label">Total Users</span>
        <span class="stat-num">{{ formatNumber(stats.total_users) }}</span>
        <span class="stat-change up">{{ stats.active_users }} active</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Paid Orders</span>
        <span class="stat-num">{{ formatNumber(stats.active_orders) }}</span>
        <span class="stat-change">—</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Products</span>
        <span class="stat-num">{{ formatNumber(stats.total_products) }}</span>
        <span class="stat-change">—</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Revenue (MTD)</span>
        <span class="stat-num">${{ formatAmount(stats.revenue_mtd) }}</span>
        <span class="stat-change up">MTD</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Nodes</span>
        <span class="stat-num">{{ formatNumber(stats.total_nodes) }}</span>
        <span class="stat-change up">{{ stats.online_nodes }} online</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Node Traffic</span>
        <span class="stat-num stat-small">▲ {{ formatBytes(stats.node_traffic_up) }}</span>
        <span class="stat-change">▼ {{ formatBytes(stats.node_traffic_down) }}</span>
      </div>
    </div>

    <div class="chart-section">
      <h3>Traffic Trend (7 days)</h3>
      <div v-if="stats.traffic_trend && stats.traffic_trend.length" class="trend-chart">
        <div v-for="p in stats.traffic_trend" :key="p.day" class="trend-col">
          <div class="trend-bar">
            <div class="bar-up" :style="{ height: barHeight(p.upload, trendMax.upload) + '%' }" :title="'up: ' + formatBytes(p.upload)"></div>
            <div class="bar-down" :style="{ height: barHeight(p.download, trendMax.download) + '%' }" :title="'down: ' + formatBytes(p.download)"></div>
          </div>
          <span class="trend-day">{{ p.day }}</span>
        </div>
      </div>
      <div v-else class="empty-hint">No traffic recorded yet.</div>
    </div>

    <div class="chart-section">
      <h3>Recent Orders</h3>
      <table class="data-table">
        <thead><tr><th>ID</th><th>Customer</th><th>Amount</th><th>Status</th><th>Date</th></tr></thead>
        <tbody>
          <tr v-for="o in stats.recent_orders" :key="o.id">
            <td>#{{ o.id }}</td>
            <td>{{ o.username || '—' }}</td>
            <td>${{ formatAmount(o.amount) }}</td>
            <td><span :class="['status', o.status]">{{ o.status }}</span></td>
            <td>{{ formatDate(o.created_at) }}</td>
          </tr>
          <tr v-if="!stats.recent_orders || !stats.recent_orders.length">
            <td colspan="5" class="empty-hint">No orders yet.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import api from '../api/index'

const stats = ref<any>({})
const error = ref('')

// Rendered once per mount; hoisted so it is not re-created on every render.
const today = new Date().toLocaleDateString()

// Precomputed once per stats update instead of re-scanning the whole trend
// array for every bar on every render.
const trendMax = computed(() => {
  const trend: any[] = stats.value.traffic_trend || []
  let upload = 1
  let download = 1
  for (const p of trend) {
    const u = Number(p.upload) || 0
    const d = Number(p.download) || 0
    if (u > upload) upload = u
    if (d > download) download = d
  }
  return { upload, download }
})

async function loadStats() {
  error.value = ''
  try {
    const res = await api.get('/admin/stats')
    stats.value = res.data || {}
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Failed to load dashboard statistics'
  }
}

function formatNumber(n: any): string {
  const num = Number(n) || 0
  return num.toLocaleString()
}

function formatAmount(n: any): string {
  const num = Number(n) || 0
  return num.toFixed(2)
}

function formatBytes(bytes: any): string {
  const b = Number(bytes) || 0
  if (b < 1024) return b + ' B'
  if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
  if (b < 1024 * 1024 * 1024) return (b / (1024 * 1024)).toFixed(1) + ' MB'
  return (b / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
}

function barHeight(v: any, max: number): number {
  if (max <= 0) return 0
  return Math.max(2, (Number(v) || 0) / max * 100)
}

function formatDate(d: any): string {
  if (!d) return '—'
  if (typeof d === 'string') return d.split('T')[0] || d
  return String(d)
}

onMounted(loadStats)
</script>

<style scoped>
.stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
  padding: 1.5rem 2rem;
}
@media (max-width: 900px) {
  .stats { grid-template-columns: repeat(2, 1fr); }
}
.stat-card {
  background: #1a1d23;
  border-radius: 10px;
  padding: 1.25rem;
  border: 1px solid #2a2d35;
}
.stat-label { display: block; font-size: 0.8rem; color: #888; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 0.5rem; }
.stat-num { display: block; font-size: 1.75rem; font-weight: 700; color: #fff; }
.stat-num.stat-small { font-size: 1.15rem; }
.stat-change { display: inline-block; font-size: 0.75rem; margin-top: 0.3rem; }
.stat-change.up { color: #4caf50; }
.chart-section { padding: 0 2rem 2rem; }
.chart-section h3 { color: #fff; font-size: 1rem; margin: 0 0 1rem; }
.error-msg { padding: 1rem 2rem; color: #ff6b6b; background: #2a1515; margin: 1rem 2rem; border-radius: 8px; }

/* Trend chart */
.trend-chart { display: flex; gap: 0.75rem; align-items: flex-end; height: 180px; background: #1a1d23; border-radius: 10px; padding: 1rem; }
.trend-col { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: flex-end; height: 100%; }
.trend-bar { display: flex; gap: 2px; align-items: flex-end; height: 100%; width: 100%; justify-content: center; }
.bar-up { width: 12px; background: #4a9eff; border-radius: 3px 3px 0 0; min-height: 2px; }
.bar-down { width: 12px; background: #ffa726; border-radius: 3px 3px 0 0; min-height: 2px; }
.trend-day { color: #888; font-size: 0.7rem; margin-top: 0.4rem; }
.empty-hint { color: #666; font-size: 0.9rem; padding: 1.5rem 0; text-align: center; }

.data-table { width: 100%; border-collapse: collapse; background: #1a1d23; border-radius: 10px; overflow: hidden; }
.data-table th, .data-table td { text-align: left; padding: 0.75rem 1rem; border-bottom: 1px solid #2a2d35; font-size: 0.9rem; }
.data-table th { color: #888; font-weight: 600; text-transform: uppercase; font-size: 0.75rem; letter-spacing: 0.5px; }
.data-table td { color: #ccc; }
.status { padding: 0.2rem 0.5rem; border-radius: 4px; font-size: 0.75rem; font-weight: 600; text-transform: capitalize; }
.status.active, .status.paid { background: #1a3a1a; color: #4caf50; }
.status.pending { background: #3a3a1a; color: #ffc107; }
.status.failed, .status.expired, .status.cancelled { background: #3a1a1a; color: #ff6b6b; }
.status.refunded { background: #1a2a3a; color: #42a5f5; }
</style>
