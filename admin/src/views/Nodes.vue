<template>
  <div class="page nodes-page">
    <main class="main">
      <header class="topbar">
        <h2>Nodes</h2>
        <div class="topbar-right">
          <button class="btn-sm" @click="loadNodes">🔄 Refresh</button>
          <button class="btn-primary" @click="openAddModal">+ Add Node</button>
        </div>
      </header>

      <div v-if="loading" class="loading">Loading nodes…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <div v-if="!loading" class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Name</th>
              <th>Type</th>
              <th>Address</th>
              <th>Port</th>
              <th>Protocol</th>
              <th>Status</th>
              <th>Traffic</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="n in nodes" :key="n.id">
              <td>{{ n.id }}</td>
              <td><strong>{{ n.name }}</strong></td>
              <td><span class="tag">{{ n.type }}</span></td>
              <td><code class="addr-text">{{ n.address }}</code></td>
              <td>{{ n.port }}</td>
              <td><span class="tag">{{ n.protocol }}</span></td>
              <td><span :class="['status', n.status]">{{ n.status }}</span></td>
              <td class="traffic-cell">
                <span class="traffic-up">▲ {{ formatBytes(n.traffic_up) }}</span>
                <span class="traffic-down">▼ {{ formatBytes(n.traffic_down) }}</span>
              </td>
              <td class="actions-cell">
                <button class="btn-tiny" @click="openEditModal(n)">✏️ Edit</button>
                <button class="btn-tiny danger" @click="deleteNode(n)">🗑️ Delete</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>

    <!-- Add/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal-content">
        <h3>{{ editingNode ? 'Edit Node' : 'Add Node' }}</h3>
        <form @submit.prevent="saveNode">
          <div class="field-row">
            <div class="field">
              <label>Name</label>
              <input v-model="form.name" type="text" placeholder="Node name" required />
            </div>
            <div class="field">
              <label>Type</label>
              <select v-model="form.type" required>
                <option value="">-- Select --</option>
                <option value="v2ray">v2ray</option>
                <option value="xray">xray</option>
              </select>
            </div>
          </div>
          <div class="field-row">
            <div class="field field-wide">
              <label>Address</label>
              <input v-model="form.address" type="text" placeholder="IP or domain" required />
            </div>
            <div class="field field-narrow">
              <label>Port</label>
              <input v-model.number="form.port" type="number" min="1" max="65535" placeholder="443" required />
            </div>
          </div>
          <div class="field-row">
            <div class="field">
              <label>Protocol</label>
              <select v-model="form.protocol" required>
                <option value="">-- Select --</option>
                <option value="vless">vless</option>
                <option value="vmess">vmess</option>
                <option value="shadowsocks">shadowsocks</option>
                <option value="trojan">trojan</option>
              </select>
            </div>
            <div class="field">
              <label>User ID</label>
              <input v-model.number="form.user_id" type="number" min="0" placeholder="1" required />
            </div>
          </div>
          <div v-if="editingNode" class="field">
            <label>Status</label>
            <select v-model="form.status">
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
              <option value="disabled">Disabled</option>
            </select>
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

interface Node {
  id: number
  name: string
  type: string
  address: string
  port: number
  protocol: string
  status: string
  traffic_up: number
  traffic_down: number
  user_id: number
  created_at?: string
  updated_at?: string
}

const nodes = ref<Node[]>([])
const loading = ref(false)
const error = ref('')
const showModal = ref(false)
const saving = ref(false)
const formError = ref('')
const editingNode = ref<Node | null>(null)

const form = ref({ name: '', type: '', address: '', port: 443, protocol: '', user_id: 1, status: 'inactive' })

function resetForm() {
  form.value = { name: '', type: '', address: '', port: 443, protocol: '', user_id: 1, status: 'inactive' }
  formError.value = ''
}

function openAddModal() {
  resetForm()
  editingNode.value = null
  showModal.value = true
}

function openEditModal(n: Node) {
  form.value = {
    name: n.name,
    type: n.type,
    address: n.address,
    port: n.port,
    protocol: n.protocol,
    user_id: n.user_id,
    status: n.status,
  }
  editingNode.value = n
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  resetForm()
}

function formatBytes(bytes: number | undefined): string {
  if (!bytes || bytes <= 0) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
}

async function loadNodes() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/admin/nodes')
    // Backend returns { data, total, page, per_page }
    const data = res.data
    nodes.value = Array.isArray(data) ? data :
                  Array.isArray(data.data) ? data.data : []
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to load nodes'
    nodes.value = []
  } finally {
    loading.value = false
  }
}

async function saveNode() {
  if (!form.value.name || !form.value.type || !form.value.address || !form.value.port || !form.value.protocol) {
    formError.value = 'All fields are required'
    return
  }
  saving.value = true
  formError.value = ''
  try {
    if (editingNode.value) {
      const payload: Record<string, any> = {
        name: form.value.name,
        type: form.value.type,
        address: form.value.address,
        port: form.value.port,
        protocol: form.value.protocol,
        status: form.value.status,
      }
      const res = await api.put(`/admin/nodes/${editingNode.value.id}`, payload)
      const updated = res.data
      const idx = nodes.value.findIndex(n => n.id === editingNode.value!.id)
      if (idx !== -1) nodes.value[idx] = updated
    } else {
      const payload = {
        name: form.value.name,
        type: form.value.type,
        address: form.value.address,
        port: form.value.port,
        protocol: form.value.protocol,
        user_id: form.value.user_id,
      }
      const res = await api.post('/admin/nodes', payload)
      const created = res.data
      nodes.value.unshift(created)
    }
    closeModal()
  } catch (e: any) {
    formError.value = e.response?.data?.error || e.message || 'Failed to save node'
  } finally {
    saving.value = false
  }
}

async function deleteNode(n: Node) {
  if (!confirm(`Delete node "${n.name}" (ID: ${n.id})? This cannot be undone.`)) return
  try {
    await api.delete(`/admin/nodes/${n.id}`)
    nodes.value = nodes.value.filter(x => x.id !== n.id)
  } catch (e: any) {
    error.value = e.response?.data?.error || e.message || 'Failed to delete node'
  }
}

onMounted(loadNodes)
</script>

<style scoped>
.nodes-page { background: #12141a; color: #e0e0e0; }
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
.status.disabled { background: #3a1a1a; color: #ff6b6b; }
.actions-cell { white-space: nowrap; }
.traffic-cell { white-space: nowrap; font-size: 0.8rem; }
.traffic-up { color: #4caf50; display: block; }
.traffic-down { color: #ffa726; display: block; }
.addr-text { font-size: 0.8rem; color: #4a9eff; background: #1e2028; padding: 0.1rem 0.3rem; border-radius: 3px; }
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
  max-width: 560px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.4);
}
.modal-content h3 { margin: 0 0 1.5rem; color: #fff; font-size: 1.15rem; }
.field { margin-bottom: 1rem; flex: 1; }
.field-wide { flex: 2; }
.field-narrow { flex: 1; }
.field-wide { flex: 2; }
.field-narrow { flex: 1; }
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
