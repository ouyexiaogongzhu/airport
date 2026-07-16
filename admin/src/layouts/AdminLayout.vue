<template>
  <div class="app-layout">
    <aside class="sidebar">
      <h2 class="brand">RFPlay Admin</h2>
      <nav>
        <router-link to="/admin/dashboard" class="nav-item" @click.prevent="navigateIfDiff('/admin/dashboard')">
          📊 Dashboard
        </router-link>
        <router-link to="/admin/users" class="nav-item" @click.prevent="navigateIfDiff('/admin/users')">
          👥 Users
        </router-link>
        <router-link to="/admin/products" class="nav-item" @click.prevent="navigateIfDiff('/admin/products')">
          📦 Products
        </router-link>
        <router-link to="/admin/orders" class="nav-item" @click.prevent="navigateIfDiff('/admin/orders')">
          🛒 Orders
        </router-link>
        <router-link to="/admin/nodes" class="nav-item" @click.prevent="navigateIfDiff('/admin/nodes')">
          🖥️ Nodes
        </router-link>
        <router-link to="/admin/tokens" class="nav-item" @click.prevent="navigateIfDiff('/admin/tokens')">
          🔑 Tokens
        </router-link>
        <router-link to="/admin/settings" class="nav-item" @click.prevent="navigateIfDiff('/admin/settings')">
          ⚙️ Settings
        </router-link>
        <router-link to="/admin/plans" class="nav-item" @click.prevent="navigateIfDiff('/admin/plans')">
          📋 Plans
        </router-link>
      </nav>
      <div class="sidebar-footer">
        <span class="badge">{{ auth.username }}</span>
        <a href="#" @click.prevent="auth.logout(); $router.push('/')" class="logout">Logout</a>
      </div>
    </aside>
    <main class="main">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

function navigateIfDiff(path: string) {
  if (route.path !== path) {
    router.push(path)
  }
}
</script>

<style scoped>
.app-layout {
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
  flex-shrink: 0;
}
.brand {
  color: #4a9eff;
  font-size: 1.1rem;
  padding: 0 1.25rem;
  margin: 0 0 2rem;
}
.nav-item {
  color: #888;
  text-decoration: none;
  padding: 0.7rem 1.25rem;
  font-size: 0.9rem;
  transition: 0.15s;
  display: block;
  cursor: pointer;
}
.nav-item:hover, .nav-item.router-link-active {
  color: #fff;
  background: #2a2d35;
}
.sidebar-footer {
  padding: 1rem 1.25rem;
  border-top: 1px solid #2a2d35;
}
.badge {
  display: block;
  color: #aaa;
  font-size: 0.8rem;
  margin-bottom: 0.5rem;
}
.logout {
  color: #ff6b6b;
  text-decoration: none;
  font-size: 0.85rem;
}
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
</style>
