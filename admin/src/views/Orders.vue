<template>
  <div class="page orders-page">
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
    <div v-if="detailOrder" class="modal-overlay" role="dialog" aria-modal="true" @click.self="closeDetail">
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
import api from '../api/index'

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

/* ---------- Mock data fallback ---------- */
function mockOrders() {
  return [
    { id: 1, username: 'john.doe', product_name: 'Pro VPN', amount: 19.99, payment_provider: 'stripe', status: 'paid', created_at: '2026-06-20T10:30:00Z' },
    { id: 2, username: 'jane.smith', product_name: 'Starter VPN', amount: 9.99, payment_provider: 'alipay', status: 'pending', created_at: '2026-06-21T14:00:00Z' },
    { id: 3, username: 'bob.wilson', product_name: 'Proxy Pack M', amount: 39.99, payment_provider: 'stripe', status: 'paid', created_at: '2026-06-22T09:15:00Z' },
    { id: 4, username: 'alice.j', product_name: 'Dedicated IP', amount: 4.99, payment_provider: 'paypal', status: 'expired', created_at: '2026-06-18T16:45:00Z' },
    { id: 5, username: 'charlie.k', product_name: 'Pro VPN', amount: 19.99, payment_provider: 'stripe', status: 'refunded', created_at: '2026-06-15T08:00:00Z' },
  ]
}

async function loadOrders() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/admin/orders', {
      params: {
        status: filterStatus.value || undefined,
        search: search.value || undefined,
      },
    })
    // Backend returns { data, total, page, per_page }
    const data = res.data
    orders.value = Array.isArray(data) ? data :
                   Array.isArray(data.data) ? data.data :
                   data.orders ? data.orders : []
  } catch (e: any) {
    error.value = e?.response?.data?.error || 'Failed to load orders'
    orders.value = []
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
    error.value = e?.response?.data?.error || 'Failed to refund order'
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
.orders-page { min-height: 100vh; background: #12141a; color: #e0e0e0; }
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
