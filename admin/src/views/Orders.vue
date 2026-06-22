<template>
  <div class="page orders-page">
    <aside class="sidebar">
      <h2 class="brand">RFPlay Admin</h2>
      <nav>
        <router-link to="/dashboard" class="nav-item">📊 Dashboard</router-link>
        <router-link to="/users" class="nav-item">👥 Users</router-link>
        <router-link to="/products" class="nav-item">📦 Products</router-link>
        <router-link to="/orders" class="nav-item">🛒 Orders</router-link>
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
            <option value="active">Active</option>
            <option value="pending">Pending</option>
            <option value="expired">Expired</option>
            <option value="cancelled">Cancelled</option>
          </select>
          <input v-model="search" placeholder="Search order…" class="search-input" />
        </div>
      </header>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr><th>Order ID</th><th>Customer</th><th>Product</th><th>Amount</th><th>Status</th><th>Date</th><th>Actions</th></tr>
          </thead>
          <tbody>
            <tr v-for="o in filteredOrders" :key="o.id">
              <td>#{{ o.id }}</td>
              <td>{{ o.customer }}</td>
              <td>{{ o.product }}</td>
              <td>${{ o.amount }}</td>
              <td><span :class="['status', o.status]">{{ o.status }}</span></td>
              <td>{{ o.date }}</td>
              <td><a href="#" class="action">View</a> <a href="#" class="action danger">Cancel</a></td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAuthStore } from '../stores/auth'
const auth = useAuthStore()

const search = ref('')
const filterStatus = ref('')

const orders = [
  { id: 1024, customer: 'john.doe', product: 'Pro VPN', amount: 19.99, status: 'active', date: '2026-06-20' },
  { id: 1023, customer: 'jane.smith', product: 'Starter VPN', amount: 9.99, status: 'pending', date: '2026-06-19' },
  { id: 1022, customer: 'bob.wilson', product: 'Proxy Pack M', amount: 39.99, status: 'active', date: '2026-06-18' },
  { id: 1021, customer: 'alice.j', product: 'Dedicated IP', amount: 4.99, status: 'expired', date: '2026-05-15' },
  { id: 1020, customer: 'charlie.b', product: 'Business VPN', amount: 49.99, status: 'cancelled', date: '2026-06-10' },
  { id: 1019, customer: 'diana.r', product: 'Proxy Pack S', amount: 14.99, status: 'active', date: '2026-06-08' },
  { id: 1018, customer: 'john.doe', product: 'Starter VPN', amount: 9.99, status: 'expired', date: '2026-04-01' },
]

const filteredOrders = computed(() =>
  orders.filter(o => {
    const matchSearch = !search.value ||
      o.id.toString().includes(search.value) ||
      o.customer.toLowerCase().includes(search.value.toLowerCase())
    const matchStatus = !filterStatus.value || o.status === filterStatus.value
    return matchSearch && matchStatus
  })
)
</script>

<style scoped>
.orders-page {
  display: flex;
  min-height: 100vh;
  background: #12141a;
  color: #e0e0e0;
}
.sidebar {
  width: 220px;
  background: #1a1d23;
  padding: 1.5rem 0;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #2a2d35;
}
.brand { color: #4a9eff; font-size: 1.1rem; padding: 0 1.25rem; margin: 0 0 2rem; }
.nav { display: flex; flex-direction: column; flex: 1; }
.nav-item { color: #888; text-decoration: none; padding: 0.7rem 1.25rem; font-size: 0.9rem; transition: 0.15s; }
.nav-item:hover, .nav-item.router-link-active { color: #fff; background: #2a2d35; }
.sidebar-footer { padding: 1rem 1.25rem; border-top: 1px solid #2a2d35; }
.badge { display: block; color: #aaa; font-size: 0.8rem; margin-bottom: 0.5rem; }
.logout { color: #ff6b6b; text-decoration: none; font-size: 0.85rem; }
.main { flex: 1; display: flex; flex-direction: column; }
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.25rem 2rem;
  border-bottom: 1px solid #2a2d35;
}
.topbar h2 { margin: 0; font-size: 1.3rem; color: #fff; }
.topbar-right { display: flex; gap: 0.75rem; align-items: center; }
.search-input {
  padding: 0.45rem 0.75rem;
  border: 1px solid #444;
  border-radius: 6px;
  background: #1a1d23;
  color: #eee;
  font-size: 0.85rem;
  width: 180px;
}
.search-input:focus { outline: none; border-color: #4a9eff; }
.filter-select {
  padding: 0.45rem 0.75rem;
  border: 1px solid #444;
  border-radius: 6px;
  background: #1a1d23;
  color: #eee;
  font-size: 0.85rem;
}
.filter-select:focus { outline: none; border-color: #4a9eff; }
.table-wrap { padding: 1.5rem 2rem; }
.data-table {
  width: 100%;
  border-collapse: collapse;
  background: #1a1d23;
  border-radius: 10px;
  overflow: hidden;
}
.data-table th, .data-table td { text-align: left; padding: 0.75rem 1rem; border-bottom: 1px solid #2a2d35; font-size: 0.85rem; }
.data-table th { color: #888; font-weight: 600; text-transform: uppercase; font-size: 0.75rem; letter-spacing: 0.5px; }
.data-table td { color: #ccc; }
.status {
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: capitalize;
}
.status.active { background: #1a3a1a; color: #4caf50; }
.status.pending { background: #3a3a1a; color: #ffc107; }
.status.expired { background: #3a1a1a; color: #ff6b6b; }
.status.cancelled { background: #2a2a2a; color: #888; }
.action { color: #4a9eff; text-decoration: none; font-size: 0.8rem; margin-right: 0.5rem; }
.action.danger { color: #ff6b6b; }
</style>
