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
        <button class="btn-sm">+ Add Product</button>
      </header>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr><th>ID</th><th>Name</th><th>Type</th><th>Price</th><th>Period</th><th>Status</th><th>Actions</th></tr>
          </thead>
          <tbody>
            <tr v-for="p in products" :key="p.id">
              <td>{{ p.id }}</td>
              <td>{{ p.name }}</td>
              <td><span class="tag">{{ p.type }}</span></td>
              <td>${{ p.price }}</td>
              <td>{{ p.period }}</td>
              <td><span :class="['status', p.status]">{{ p.status }}</span></td>
              <td><a href="#" class="action">Edit</a> <a href="#" class="action danger">Archive</a></td>
            </tr>
          </tbody>
        </table>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '../stores/auth'
const auth = useAuthStore()

const products = [
  { id: 1, name: 'Starter VPN', type: 'VPN', price: 9.99, period: 'month', status: 'active' },
  { id: 2, name: 'Pro VPN', type: 'VPN', price: 19.99, period: 'month', status: 'active' },
  { id: 3, name: 'Business VPN', type: 'VPN', price: 49.99, period: 'month', status: 'active' },
  { id: 4, name: 'Proxy Pack S', type: 'Proxy', price: 14.99, period: 'month', status: 'active' },
  { id: 5, name: 'Proxy Pack M', type: 'Proxy', price: 39.99, period: 'month', status: 'active' },
  { id: 6, name: 'Proxy Pack L', type: 'Proxy', price: 99.99, period: 'month', status: 'inactive' },
  { id: 7, name: 'Dedicated IP', type: 'IP', price: 4.99, period: 'month', status: 'active' },
  { id: 8, name: 'Static IP Bundle', type: 'IP', price: 12.99, period: 'month', status: 'inactive' },
]
</script>

<style scoped>
.products-page {
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
.btn-sm {
  padding: 0.45rem 0.9rem;
  background: #4a9eff;
  color: #fff;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.85rem;
}
.btn-sm:hover { background: #3a8ef0; }
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
.tag {
  background: #2a2d35;
  color: #aaa;
  padding: 0.15rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
}
.status {
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: capitalize;
}
.status.active { background: #1a3a1a; color: #4caf50; }
.status.inactive { background: #2a2a2a; color: #888; }
.action { color: #4a9eff; text-decoration: none; font-size: 0.8rem; margin-right: 0.5rem; }
.action.danger { color: #ff6b6b; }
</style>
