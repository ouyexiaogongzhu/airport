<template>
  <div class="page plans-page">
    <main class="main">
      <header class="topbar">
        <h2>Plans</h2>
        <div class="topbar-right">
          <button class="btn-sm" @click="loadPlans">🔄 Refresh</button>
          <button class="btn-primary" @click="openAddModal">+ Add Plan</button>
        </div>
      </header>

      <div v-if="loading" class="loading">Loading plans…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <div v-if="!loading && plans.length === 0" class="empty-state">
        <p>No plans yet. Click <strong>"+ Add Plan"</strong> to create your first pricing plan.</p>
      </div>

      <div v-if="!loading && plans.length > 0" class="plan-grid">
        <div
          v-for="p in plans"
          :key="p.id"
          :class="['plan-card', { 'plan-inactive': p.status === 'inactive', 'plan-popular': p.status === 'popular' }]"
        >
          <div class="plan-card-header">
            <h3 class="plan-name">{{ p.name }}</h3>
            <span :class="['plan-badge', p.type.toLowerCase()]">{{ p.type }}</span>
          </div>

          <div class="plan-price">
            <span class="price-currency">$</span>
            <span class="price-amount">{{ p.price.toFixed(2) }}</span>
            <span class="price-period">/mo</span>
          </div>

          <div class="plan-meta">
            <div class="meta-item">
              <span class="meta-label">Stock</span>
              <span :class="['meta-val', p.stock <= 0 ? 'out' : '']">
                {{ p.stock > 0 ? p.stock : 'Out of stock' }}
              </span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Type</span>
              <span class="meta-val">{{ p.type }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-label">Created</span>
              <span class="meta-val">{{ formatDate(p.created_at) }}</span>
            </div>
          </div>

          <div class="plan-status-bar">
            <label class="toggle-label">
              <input
                type="checkbox"
                :checked="p.status === 'active' || p.status === 'popular'"
                @change="togglePlanStatus(p)"
              />
              <span class="toggle-track">
                <span class="toggle-thumb"></span>
              </span>
              <span class="toggle-text">{{ p.status === 'active' || p.status === 'popular' ? 'Active' : 'Inactive' }}</span>
            </label>
            <div class="plan-actions">
              <button class="btn-tiny" @click="openEditModal(p)">✏️ Edit</button>
              <button class="btn-tiny danger" @click="archivePlan(p)">📦 Archive</button>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Add/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal-content">
        <h3>{{ editingPlan ? 'Edit Plan' : 'Add Plan' }}</h3>
        <form @submit.prevent="savePlan">
          <div class="field">
            <label>Plan Name</label>
            <input v-model="form.name" type="text" placeholder="e.g. Pro VPN, Starter Pack" required />
          </div>
          <div class="field">
            <label>Type</label>
            <select v-model="form.type" required>
              <option value="">-- Select type --</option>
              <option value="VPN">VPN</option>
              <option value="Proxy">Proxy</option>
              <option value="IP">IP</option>
              <option value="Bundle">Bundle</option>
            </select>
          </div>
          <div class="field-row">
            <div class="field">
              <label>Price ($)</label>
              <input v-model.number="form.price" type="number" step="0.01" min="0" placeholder="0.00" required />
            </div>
            <div class="field">
              <label>Stock</label>
              <input v-model.number="form.stock" type="number" min="0" placeholder="Unlimited" />
            </div>
          </div>
          <p v-if="formError" class="error">{{ formError }}</p>
          <div class="modal-actions">
            <button type="button" class="btn-cancel" @click="closeModal">Cancel</button>
            <button type="submit" class="btn-primary" :disabled="saving">
              {{ saving ? 'Saving…' : 'Save' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '../api/index'

interface Plan {
  id: number
  name: string
  type: string
  price: number
  stock: number
  status: string
  created_at?: string
  updated_at?: string
}

const plans = ref<Plan[]>([])
const loading = ref(false)
const error = ref('')
const showModal = ref(false)
const saving = ref(false)
const formError = ref('')
const editingPlan = ref<Plan | null>(null)

const form = ref({ name: '', type: '', price: 0, stock: 0 })

function resetForm() {
  form.value = { name: '', type: '', price: 0, stock: 0 }
  formError.value = ''
}

function openAddModal() {
  resetForm()
  editingPlan.value = null
  showModal.value = true
}

function openEditModal(p: Plan) {
  form.value = { name: p.name, type: p.type, price: p.price, stock: p.stock }
  editingPlan.value = p
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  resetForm()
}

function formatDate(dateStr: string | undefined): string {
  if (!dateStr) return '—'
  const d = new Date(dateStr)
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
}

async function loadPlans() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/admin/products')
    plans.value = Array.isArray(res.data.products) ? res.data.products : []
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to load plans'
    plans.value = []
  } finally {
    loading.value = false
  }
}

async function savePlan() {
  if (!form.value.name || !form.value.type || form.value.price <= 0) {
    formError.value = 'Name, type, and price are required'
    return
  }
  saving.value = true
  formError.value = ''
  try {
    const payload = {
      name: form.value.name,
      type: form.value.type,
      price: form.value.price,
      stock: form.value.stock || 0,
    }
    if (editingPlan.value) {
      const res = await api.put(`/admin/products/${editingPlan.value.id}`, payload)
      const updated = res.data.product || res.data
      const idx = plans.value.findIndex(p => p.id === editingPlan.value!.id)
      if (idx !== -1) plans.value[idx] = updated
    } else {
      const res = await api.post('/admin/products', payload)
      const created = res.data.product || res.data
      plans.value.unshift(created)
    }
    closeModal()
  } catch (e: any) {
    formError.value = e.response?.data?.error || e.message || 'Failed to save plan'
  } finally {
    saving.value = false
  }
}

async function togglePlanStatus(p: Plan) {
  const newStatus = (p.status === 'active' || p.status === 'popular') ? 'inactive' : 'active'
  try {
    const res = await api.put(`/admin/products/${p.id}`, { status: newStatus })
    const updated = res.data.product || res.data
    const idx = plans.value.findIndex(x => x.id === p.id)
    if (idx !== -1) plans.value[idx] = updated
    else p.status = newStatus
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to update plan status'
  }
}

async function archivePlan(p: Plan) {
  if (!confirm(`Archive plan "${p.name}"? It will be marked inactive.`)) return
  try {
    await api.put(`/admin/products/${p.id}`, { status: 'inactive' })
    p.status = 'inactive'
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to archive plan'
  }
}

onMounted(loadPlans)
</script>

<style scoped>
.btn-sm { padding: 0.45rem 0.9rem; border: 1px solid #4a9eff; border-radius: 6px; background: transparent; color: #4a9eff; cursor: pointer; font-size: 0.85rem; }
.btn-sm:hover { background: #4a9eff22; }
.btn-primary { padding: 0.45rem 0.9rem; background: #4a9eff; color: #fff; border: none; border-radius: 6px; cursor: pointer; font-size: 0.85rem; }
.btn-primary:hover { background: #3a8ef0; }
.btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-cancel { padding: 0.45rem 0.9rem; border: 1px solid #444; border-radius: 6px; background: transparent; color: #aaa; cursor: pointer; font-size: 0.85rem; }
.btn-cancel:hover { border-color: #888; color: #fff; }
.loading { padding: 3rem; text-align: center; color: #888; }
.error-msg { padding: 1rem 2rem; color: #ff6b6b; background: #2a1515; margin: 1rem 2rem; border-radius: 8px; }
.empty-state { padding: 4rem 2rem; text-align: center; color: #888; }
.empty-state strong { color: #4a9eff; }

/* Plan Card Grid */
.plan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1.25rem;
  padding: 1.5rem 2rem;
}

.plan-card {
  background: #1a1d23;
  border: 1px solid #2a2d35;
  border-radius: 14px;
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  transition: border-color 0.2s, box-shadow 0.2s;
  position: relative;
}
.plan-card:hover {
  border-color: #4a9eff44;
  box-shadow: 0 4px 20px rgba(74, 158, 255, 0.08);
}
.plan-card.plan-inactive {
  opacity: 0.65;
}
.plan-card.plan-inactive:hover {
  border-color: #ff6b6b44;
  box-shadow: 0 4px 20px rgba(255, 107, 107, 0.08);
}
.plan-card.plan-popular {
  border-color: #4a9eff;
  box-shadow: 0 0 0 1px #4a9eff66;
}

/* Card Header */
.plan-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1rem;
}
.plan-name {
  margin: 0;
  font-size: 1.15rem;
  color: #fff;
  font-weight: 600;
}
.plan-badge {
  font-size: 0.7rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 0.2rem 0.55rem;
  border-radius: 4px;
}
.plan-badge.vpn { background: #1a3a5a; color: #4a9eff; }
.plan-badge.proxy { background: #2a3a1a; color: #8bc34a; }
.plan-badge.ip { background: #3a2a1a; color: #ff9800; }
.plan-badge.bundle { background: #3a1a3a; color: #ce93d8; }

/* Price */
.plan-price {
  text-align: center;
  padding: 1.25rem 0;
  margin-bottom: 0.75rem;
  border-top: 1px solid #22252b;
  border-bottom: 1px solid #22252b;
}
.price-currency {
  font-size: 1.3rem;
  color: #aaa;
  vertical-align: top;
  line-height: 2rem;
}
.price-amount {
  font-size: 2.5rem;
  font-weight: 700;
  color: #fff;
}
.price-period {
  font-size: 0.85rem;
  color: #888;
  margin-left: 0.2rem;
}

/* Meta */
.plan-meta {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin-bottom: 1rem;
  flex: 1;
}
.meta-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.meta-label {
  color: #888;
  font-size: 0.8rem;
}
.meta-val {
  color: #ccc;
  font-size: 0.85rem;
  font-weight: 500;
}
.meta-val.out {
  color: #ff6b6b;
}

/* Status bar at bottom */
.plan-status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-top: 0.75rem;
  border-top: 1px solid #22252b;
}

/* Toggle switch */
.toggle-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  user-select: none;
}
.toggle-label input {
  display: none;
}
.toggle-track {
  width: 36px;
  height: 20px;
  border-radius: 10px;
  background: #2a2d35;
  position: relative;
  transition: background 0.2s;
  display: inline-block;
}
.toggle-label input:checked + .toggle-track {
  background: #4a9eff;
}
.toggle-thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #888;
  transition: left 0.2s, background 0.2s;
}
.toggle-label input:checked + .toggle-track .toggle-thumb {
  left: 18px;
  background: #fff;
}
.toggle-text {
  font-size: 0.8rem;
  color: #aaa;
  font-weight: 500;
}

/* Actions */
.plan-actions {
  display: flex;
  gap: 0.35rem;
}
.btn-tiny {
  padding: 0.2rem 0.5rem;
  border: 1px solid #444;
  border-radius: 4px;
  background: transparent;
  color: #aaa;
  cursor: pointer;
  font-size: 0.75rem;
}
.btn-tiny:hover { border-color: #4a9eff; color: #4a9eff; }
.btn-tiny.danger:hover { border-color: #ff6b6b; color: #ff6b6b; }

/* Modal */
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.modal-content {
  background: #1a1d23;
  border: 1px solid #2a2d35;
  border-radius: 12px;
  padding: 2rem;
  width: 100%;
  max-width: 480px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.4);
}
.modal-content h3 { margin: 0 0 1.5rem; color: #fff; font-size: 1.15rem; }
.field { margin-bottom: 1rem; flex: 1; }
.field-row { display: flex; gap: 1rem; }
.field label { display: block; margin-bottom: 0.35rem; color: #ccc; font-size: 0.85rem; font-weight: 500; }
.field input, .field select {
  width: 100%;
  padding: 0.55rem 0.75rem;
  border: 1px solid #444;
  border-radius: 6px;
  background: #12141a;
  color: #e0e0e0;
  font-size: 0.9rem;
  box-sizing: border-box;
  outline: none;
}
.field input:focus, .field select:focus { border-color: #4a9eff; }
.field select { cursor: pointer; }
.modal-actions { display: flex; gap: 0.75rem; justify-content: flex-end; margin-top: 1.5rem; }
.error { color: #ff6b6b; font-size: 0.85rem; margin: 0.5rem 0; }
</style>
