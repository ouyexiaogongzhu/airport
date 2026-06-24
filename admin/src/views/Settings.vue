<template>
  <div class="page settings-page">
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
        <h2>Settings</h2>
      </header>
      <div class="page-content">
        <p>RFPlay Admin - Settings</p>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '../stores/auth'
const auth = useAuthStore()
</script>

<style scoped>
.settings-page { display: flex; min-height: 100vh; background: #12141a; color: #e0e0e0; }
.sidebar { width: 220px; background: #1a1d23; padding: 1.5rem 0; display: flex; flex-direction: column; border-right: 1px solid #2a2d35; }
.brand { color: #4a9eff; font-size: 1.1rem; padding: 0 1.25rem; margin: 0 0 2rem; }
.nav { display: flex; flex-direction: column; flex: 1; }
.nav-item { color: #888; text-decoration: none; padding: 0.7rem 1.25rem; font-size: 0.9rem; transition: 0.15s; display: block; }
.nav-item:hover, .nav-item.router-link-active { color: #fff; background: #2a2d35; }
.sidebar-footer { padding: 1rem 1.25rem; border-top: 1px solid #2a2d35; }
.badge { display: block; color: #aaa; font-size: 0.8rem; margin-bottom: 0.5rem; }
.logout { color: #ff6b6b; text-decoration: none; font-size: 0.85rem; }
.main { flex: 1; display: flex; flex-direction: column; }
.topbar { display: flex; align-items: center; justify-content: space-between; padding: 1.25rem 2rem; border-bottom: 1px solid #2a2d35; }
.topbar h2 { margin: 0; font-size: 1.3rem; color: #fff; }
.page-content { padding: 2rem; }
.page-content p { color: #888; }
</style>
