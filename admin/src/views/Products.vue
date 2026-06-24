<template>
  <div class="page products-page">
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
        <h2>Products</h2>
        <div class="topbar-right">
          <button class="btn-sm" @click="loadProducts">🔄 Refresh</button>
          <button class="btn-primary" @click="openAddModal">+ Add Product</button>
        </div>
      </header>

      <div v-if="loading" class="loading">Loading products…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <div v-if="!loading" class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Name</th>
              <th>Type</th>
              <th>Price</th>
              <th>Stock</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in products" :key="p.id">
              <td>{{ p.id }}</td>
              <td><strong>{{ p.name }}</strong></td>
              <td><span class="tag">{{ p.type }}</span></td>
              <td>${{ p.price.toFixed(2) }}</td>
              <td>{{ p.stock }}</td>
              <td><span :class="['status', p.status]">{{ p.status }}</span></td>
              <td class="actions-cell">
                <button class="btn-tiny" @click="openEditModal(p)">✏️ Edit</button>
                <button class="btn-tiny danger" @click="archiveProduct(p)">📦 Archive</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>

    <!-- Add/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal-content">
        <h3>{{ editingProduct ? 'Edit Product' : 'Add Product' }}</h3>
        <form @submit.prevent="saveProduct">
          <div class="field">
            <label>Name</label>
            <input v-model="form.name" type="text" placeholder="Product name" required />
          </div>
          <div class="field">
            <label>Type</label>
            <select v-model="form.type" required>
              <option value="">-- Select --</option>
              <option value="VPN">VPN</option>
              <option value="Proxy">Proxy</option>
              <option value="IP">IP</option>
            </select>
          </div>
          <div class="field-row">
            <div class="field">
              <label>Price ($)</label>
              <input v-model.number="form.price" type="number" step="0.01" min="0" placeholder="0.00" required />
            </div>
            <div class="field">
              <label>Stock</label>
              <input v-model.number="form.stock" type="number" min="0" placeholder="0" />
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
import { useAuthStore } from '../stores/auth'
import api from '../api/index'

const auth = useAuthStore()

interface Product {
  id: number
  name: string
  type: string
  price: number
  stock: number
  status: string
  created_at?: string
  updated_at?: string
}

const products = ref<Product[]>([])
const loading = ref(false)
const error = ref('')
const showModal = ref(false)
const saving = ref(false)
const formError = ref('')
const editingProduct = ref<Product | null>(null)

const form = ref({ name: '', type: '', price: 0, stock: 0 })

function resetForm() {
  form.value = { name: '', type: '', price: 0, stock: 0 }
  formError.value = ''
}

function openAddModal() {
  resetForm()
  editingProduct.value = null
  showModal.value = true
}

function openEditModal(p: Product) {
  form.value = { name: p.name, type: p.type, price: p.price, stock: p.stock }
  editingProduct.value = p
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  resetForm()
}

async function loadProducts() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/admin/products')
    products.value = Array.isArray(res.data.products) ? res.data.products : []
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to load products'
    products.value = []
  } finally {
    loading.value = false
  }
}

async function saveProduct() {
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
    if (editingProduct.value) {
      const res = await api.put(`/admin/products/${editingProduct.value.id}`, payload)
      const updated = res.data.product
      const idx = products.value.findIndex(p => p.id === editingProduct.value!.id)
      if (idx !== -1) products.value[idx] = updated
    } else {
      const res = await api.post('/admin/products', payload)
      const created = res.data.product
      products.value.unshift(created)
    }
    closeModal()
  } catch (e: any) {
    formError.value = e.response?.data?.error || e.message || 'Failed to save product'
  } finally {
    saving.value = false
  }
}

async function archiveProduct(p: Product) {
  if (!confirm(`Archive product "${p.name}"?`)) return
  try {
    await api.put(`/admin/products/${p.id}`, { status: 'inactive' })
    p.status = 'inactive'
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to archive product'
  }
}

onMounted(loadProducts)
</script>

<style scoped>
.products-page { display: flex; min-height: 100vh; background: #12141a; color: #e0e0e0; }
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
.btn-sm { padding: 0.45rem 0.9rem; border: 1px solid #4a9eff; border-radius: 6px; background: transparent; color: #4a9eff; cursor: pointer; font-size: 0.85rem; }
.btn-sm:hover { background: #4a9eff22; }
.btn-primary { padding: 0.45rem 0.9rem; background: #4a9eff; color: #fff; border: none; border-radius: 6px; cursor: pointer; font-size: 0.85rem; }
.btn-primary:hover { background: #3a8ef0; }
.btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-cancel { padding: 0.45rem 0.9rem; border: 1px solid #444; border-radius: 6px; background: transparent; color: #aaa; cursor: pointer; font-size: 0.85rem; }
.btn-cancel:hover { border-color: #888; color: #fff; }
.loading { padding: 3rem; text-align: center; color: #888; }
.error-msg { padding: 1rem 2rem; color: #ff6b6b; background: #2a1515; margin: 1rem 2rem; border-radius: 8px; }
.table-wrap { padding: 1.5rem 2rem; flex: 1; }
.data-table { width: 100%; border-collapse: collapse; }
.data-table th { text-align: left; padding: 0.75rem 0.5rem; color: #888; font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid #2a2d35; }
.data-table td { padding: 0.75rem 0.5rem; border-bottom: 1px solid #22252b; font-size: 0.9rem; }
.data-table tr:hover td { background: #1a1d2322; }
.tag { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; background: #2a2d35; color: #aaa; font-size: 0.8rem; }
.status { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 4px; font-size: 0.8rem; text-transform: capitalize; }
.status.active { background: #1a3a1a; color: #4caf50; }
.status.inactive { background: #2a2d35; color: #888; }
.actions-cell { white-space: nowrap; }
.btn-tiny { padding: 0.2rem 0.5rem; border: 1px solid #444; border-radius: 4px; background: transparent; color: #aaa; cursor: pointer; font-size: 0.75rem; margin: 0 0.15rem; }
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
