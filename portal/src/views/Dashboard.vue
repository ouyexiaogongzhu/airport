<template>
  <div class="page dashboard">
    <nav class="topbar">
      <span class="brand">RFPlay</span>
      <div class="nav-links">
        <router-link to="/dashboard">Dashboard</router-link>
        <router-link to="/products">Products</router-link>
        <a href="#" @click.prevent="auth.logout(); $router.push('/')">Logout</a>
      </div>
      <span class="user-badge">{{ auth.username }}</span>
    </nav>

    <main class="content">
      <h2>Welcome back, {{ auth.username }}!</h2>
      <p class="greeting">Here's what's happening with your account today.</p>

      <div class="cards">
        <div class="card">
          <h3>Active Products</h3>
          <span class="num">3</span>
        </div>
        <div class="card">
          <h3>Total Orders</h3>
          <span class="num">12</span>
        </div>
        <div class="card">
          <h3>Account Credit</h3>
          <span class="num">$45.80</span>
        </div>
      </div>

      <section class="recent">
        <h3>Recent Activity</h3>
        <table>
          <tr><td>Order #1024</td><td>Completed</td><td>2 hours ago</td></tr>
          <tr><td>Order #1023</td><td>Processing</td><td>Yesterday</td></tr>
          <tr><td>Product "Express VPN"</td><td>Activated</td><td>3 days ago</td></tr>
        </table>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '../stores/auth'
const auth = useAuthStore()
</script>

<style scoped>
.dashboard {
  min-height: 100vh;
  background: #f5f7fa;
}
.topbar {
  display: flex;
  align-items: center;
  padding: 0.75rem 2rem;
  background: white;
  border-bottom: 1px solid #e0e0e0;
  gap: 2rem;
}
.brand {
  font-weight: 700;
  color: #1a73e8;
  font-size: 1.2rem;
}
.nav-links { display: flex; gap: 1.25rem; flex: 1; }
.nav-links a {
  color: #555;
  text-decoration: none;
  font-size: 0.9rem;
  font-weight: 500;
}
.nav-links a:hover, .nav-links a.router-link-active { color: #1a73e8; }
.user-badge {
  background: #e8f0fe;
  color: #1a73e8;
  padding: 0.3rem 0.8rem;
  border-radius: 20px;
  font-size: 0.8rem;
  font-weight: 600;
}
.content {
  max-width: 960px;
  margin: 0 auto;
  padding: 2rem;
}
h2 { margin: 0; font-size: 1.5rem; color: #222; }
.greeting { color: #666; margin: 0.25rem 0 2rem; }
.cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 1.25rem;
  margin-bottom: 2rem;
}
.card {
  background: white;
  border-radius: 10px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
}
.card h3 { margin: 0 0 0.5rem; font-size: 0.85rem; color: #888; text-transform: uppercase; letter-spacing: 0.5px; }
.card .num { font-size: 2rem; font-weight: 700; color: #1a73e8; }
.recent {
  background: white;
  border-radius: 10px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
}
.recent h3 { margin: 0 0 1rem; font-size: 1rem; color: #333; }
table { width: 100%; border-collapse: collapse; }
td { padding: 0.6rem 0; border-bottom: 1px solid #f0f0f0; color: #444; font-size: 0.9rem; }
</style>
