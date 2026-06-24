<template>
  <div class="page orders-page">
    <aside class="sidebar">
      <h2 class="brand">RFPlay Admin</h2>
      <nav>
        <router-link to="/dashboard" class="nav-item">📊 Dashboard</router-link>
        <router-link to="/users" class="nav-item">👥 Users</router-link>
        <router-link to="/products" class="nav-item">📦 Products</router-link>
        <router-link to="/orders" class="nav-item">🛒 Orders</router-link>
        <router-link to="/nodes" class="nav-item">🖥️ Nodes</router-link>
        <router-link to="/tokens" class="nav-item">🔑 Tokens</router-link>
        <router-link to="/settings" class="nav-item">⚙️ Settings</router-link>
        <router-link to="/plans" class="nav-item">📋 Plans</router-link>
      </nav>
      <div class="sidebar-footer">
        <span class="badge">{{ auth.username }}</span>
        <a href="#" @click.prevent="auth.logout(); $router.push('/')" class="logout">Logout</a>
      </div>
    </aside>

    <main class="main">
      <header class="topbar">
        <h2>Orders</h2>
        <div class="topbar-right">
          <select v-model="filterStatus" class="filter-select">
            <option value="">All Status</option>
            <option value="paid">Paid</option>
            <option value="pending">Pending</option>
            <option value="failed">Failed</option>
            <option value="expired">Expired</option>
            <option value="cancelled">Cancelled</option>
            <option value="refunded">Refunded</option>
          </select>
          <input v-model="search" placeholder="Search order…" class="search-input" />
          <button class="btn-sm" @click="loadOrders">🔄 Refresh</button>
        </div>
      </header>

      <div v-if="loading" class="loading">Loading orders…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <div v-if="!loading" class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>Order ID</th>
              <th>Customer</th>
              <th>Product</th>
              <th>Amount</th>
              <th>Provider</th>
              <th>Status</th>
              <th>Date</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="o in filteredOrders" :key="o.id">
              <td>#{{ o.id }}</td>
              <td>{{ o.username || o.customer || '—' }}</td>
              <td>{{ o.product_name || o.product || '—' }}</td>
              <td>${{ formatAmount(o.amount) }}</td>
              <td><span class="tag">{{ o.payment_provider || o.gateway || '—' }}</span></td>
              <td><span :class="['status', 'status-' + (o.status || 'unknown')]">{{ statusLabel(o.status) }}</span></td>
              <td>{{ formatDate(o.created_at || o.date) }}</td>
              <td class="actions-cell">
                <button class="btn-tiny" @click="viewOrder(o)">👁</button>
                <button v-if="canRefund(o)" class="btn-tiny" @click="refundOrder(o)" :disabled="refundingId === o.id">
                  💰 Refund
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>

    <!-- Order Detail Modal -->
    <div v-if="detailOrder" class="modal-overlay" @click.self="closeDetail">
      <div class="modal">
        <div class="modal-header">
          <h3>Order #{{ detailOrder.id }}</h3>
          <button class="modal-close" @click="closeDetail">×</button>
        </div>
        <div class="modal-body">
          <div class="detail-row"><span class="dl">Customer</span><span class="dv">{{ detailOrder.username || detailOrder.customer || '—' }}</span></div>
          <div class="detail-row"><span class="dl">Product</span><span class="dv">{{ detailOrder.product_name || detailOrder.product || '—' }}</span></div>
          <div class="detail-row"><span class="dl">Amount</span><span class="dv">${{ formatAmount(detailOrder.amount) }}</span></div>
          <div class="detail-row"><span class="dl">Status</span><span :class="['status', 'status-' + (detailOrder.status || 'unknown')]">{{ statusLabel(detailOrder.status) }}</span></div>
          <div class="detail-row"><span class="dl">Provider</span><span class="dv">{{ detailOrder.payment_provider || detailOrder.gateway || '—' }}</span></div>
          <div class="detail-row"><span class="dl">Created</span><span class="dv">{{ formatDate(detailOrder.created_at || detailOrder.date) }}</span></div>
          <div class="detail-row"><span class="dl">Updated</span><span class="dv">{{ formatDate(detailOrder.updated_at) }}</span></div>
          <div v-if="detailOrder.payment_url" class="detail-row">
            <span class="dl">Payment URL</span>
            <a :href="detailOrder.payment_url" target="_blank" class="dv link">{{ detailOrder.payment_url.slice(0, 50) }}…</a>
          </div>
          <div v-if="detailOrder.transaction_id" class="detail-row"><span class="dl">TX ID</span><span class="dv mono">{{ detailOrder.transaction_id.slice(0, 20) }}…</span></div>
        </div>
        <div class="modal-footer">
          <button class="btn-sm" @click="closeDetail">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api/index.js'
const auth = useAuthStore()

const search = ref('')
const filterStatus = ref('')
const orders = ref<any[]>([])
const loading = ref(false)
const error = ref('')
const refundingId = ref<number | null>(null)
const detailOrder = ref<any | null>(null)

function formatAmount(amount: any): string {
  const n = parseFloat(amount)
  return isNaN(n) ? '0.00' : n.toFixed(2)
}

function statusLabel(status?: string): string {
  switch (status) {
    case 'paid': return 'Paid'
    case 'pending': return 'Pending'
    case 'failed': return 'Failed'
    case 'expired': return 'Expired'
    case 'cancelled': return 'Cancelled'
    case 'refunded': return 'Refunded'
    default: return status || 'Unknown'
  }
}

function canRefund(o: any): boolean {
  return o.status === 'paid' || o.status === 'active'
}

function formatDate(d: any): string {
  if (!d) return '—'
  if (typeof d === 'string' && d.includes('-')) return d.split('T')[0] || d
  if (typeof d === 'number' && d > 0) {
    const dt = new Date(d * 1000)
    return dt.toISOString().split('T')[0]
  }
  return String(d).slice(0, 10)
}

async function loadOrders() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/admin/orders')
    const data = res.data
    orders.value = Array.isArray(data) ? data :
                   data.orders ? data.orders : []
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to load orders'
  } finally {
    loading.value = false
  }
}

function viewOrder(o: any) {
  detailOrder.value = o
}

function closeDetail() {
  detailOrder.value = null
}

async function refundOrder(o: any) {
  if (!confirm(`Refund order #${o.id} for $${formatAmount(o.amount)}?`)) return
  refundingId.value = o.id
  try {
    await api.post(`/admin/orders/${o.id}/refund`)
    o.status = 'refunded'
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Refund failed'
  } finally {
    refundingId.value = null
  }
}

const filteredOrders = computed(() =>
  orders.value.filter((o: any) => {
    const matchSearch = !search.value ||
      String(o.id).includes(search.value) ||
      (o.username || '').toLowerCase().includes(search.value.toLowerCase()) ||
      (o.customer || '').toLowerCase().includes(search.value.toLowerCase())
    const matchStatus = !filterStatus.value || o.status === filterStatus.value
    return matchSearch && matchStatus
  })
)

onMounted(loadOrders)
</script>

<style scoped>
.orders-page { display: flex; min-height: 100vh; background: #12141a; color: #e0e0e0; }
.sidebar { width: 220px; background: #1a1d23; padding: 1.5rem 0; display: flex; flex-direction: column; border-right: 1px solid #2a2d35; }
.brand { color: #4a9eff; font-size: 1.1rem; padding: 0 1.25rem; margin: 0 0 2rem; }
.nav-item { color: #888; text-decoration: none; padding: 0.7rem 1.25rem; font-size: 0.9rem; transition: 0.15s; display: block; }
.nav-item:hover, .nav-item.router-link-active { color: #fff; background: #2a2d35; }
.sidebar-footer { padding: 1rem 1.25rem; border-top: 1px solid #2a2d35; }
.badge { display: block; color: #aaa; font-size: 0.8rem; margin-bottom: 0.5rem; }
.logout { color: #ff6b6b; text-decoration: none; font-size: 0.85rem; }
.main { flex: 1; display: flex; flex-direction: column; }
.topbar { display: flex; align-items: center; justify-content: space-between; padding: 1.25rem 2rem; border-bottom: 1px solid #2a2d35; }
.topbar h2 { margin: 0; font-size: 1.3rem; color: #fff; }
.topbar-right { display: flex; gap: 0.75rem; align-items: center; }
.search-input { padding: 0.45rem 0.75rem; border: 1px solid #444; border-radius: 6px; background: #1e2028; color: #e0e0e0; outline: none; width: 180px; }
.search-input:focus { border-color: #4a9eff; }
.filter-select { padding: 0.45rem 0.75rem; border: 1px solid #444; border-radius: 6px; background: #1e2028; color: #e0e0e0; outline: none; }
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
.status-paid { background: #1a3a1a; color: #4caf50; }
.status-pending { background: #2a2a1a; color: #ffa726; }
.status-failed { background: #3a1a1a; color: #ff6b6b; }
.status-expired { background: #2a2d35; color: #888; }
.status-cancelled { background: #2a2d35; color: #888; }
.status-refunded { background: #1a2a3a; color: #42a5f5; }
.btn-tiny { padding: 0.2rem 0.5rem; border: 1px solid #444; border-radius: 4px; background: transparent; color: #aaa; cursor: pointer; font-size: 0.75rem; margin: 0 0.15rem; }
.btn-tiny:hover { border-color: #4a9eff; color: #4a9eff; }
.btn-tiny:disabled { opacity: 0.4; cursor: not-allowed; }
.actions-cell { white-space: nowrap; }

/* Modal */
.modal-overlay { position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: #1a1d23; border: 1px solid #2a2d35; border-radius: 10px; max-width: 500px; width: 90%; }
.modal-header { display: flex; justify-content: space-between; align-items: center; padding: 1rem 1.25rem; border-bottom: 1px solid #2a2d35; }
.modal-header h3 { margin: 0; color: #fff; font-size: 1.1rem; }
.modal-close { background: none; border: none; color: #888; font-size: 1.3rem; cursor: pointer; }
.modal-close:hover { color: #fff; }
.modal-body { padding: 1.25rem; }
.modal-footer { padding: 0.75rem 1.25rem; border-top: 1px solid #2a2d35; text-align: right; }
.detail-row { display: flex; justify-content: space-between; padding: 0.5rem 0; border-bottom: 1px solid #22252b; }
.detail-row:last-child { border-bottom: none; }
.dl { color: #888; font-size: 0.85rem; }
.dv { color: #e0e0e0; font-size: 0.9rem; }
.mono { font-family: monospace; font-size: 0.8rem; }
.link { color: #4a9eff; text-decoration: none; }
.link:hover { text-decoration: underline; }
</style>
