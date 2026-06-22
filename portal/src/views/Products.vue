<template>
  <div class="page products">
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
      <h2>Our Products</h2>
      <p class="subtitle">Choose a plan that fits your needs</p>

      <div class="product-grid">
        <div v-for="p in products" :key="p.id" class="product-card">
          <div class="icon" :style="{ background: p.color + '18' }">
            <span :style="{ color: p.color }">{{ p.icon }}</span>
          </div>
          <h3>{{ p.name }}</h3>
          <p class="desc">{{ p.description }}</p>
          <p class="price">${{ p.price }}<span v-if="p.period"> / {{ p.period }}</span></p>
          <span class="tag">{{ p.type }}</span>
          <button class="btn" @click="selectProduct(p)">Select</button>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '../stores/auth'
const auth = useAuthStore()

const products = [
  { id: 1, name: 'Starter VPN', description: 'Basic VPN access for light browsing', price: 9.99, period: 'month', type: 'VPN', color: '#1a73e8', icon: '🛡' },
  { id: 2, name: 'Pro VPN', description: 'High-speed VPN with dedicated nodes', price: 19.99, period: 'month', type: 'VPN', color: '#34a853', icon: '🚀' },
  { id: 3, name: 'Business VPN', description: 'Team VPN with admin controls', price: 49.99, period: 'month', type: 'VPN', color: '#ea4335', icon: '🏢' },
  { id: 4, name: 'Proxy Pack S', description: '5 residential proxies', price: 14.99, period: 'month', type: 'Proxy', color: '#fbbc04', icon: '🌐' },
  { id: 5, name: 'Proxy Pack M', description: '20 residential proxies', price: 39.99, period: 'month', type: 'Proxy', color: '#ff6d01', icon: '🌍' },
  { id: 6, name: 'Dedicated IP', description: 'Static dedicated IP address', price: 4.99, period: 'month', type: 'IP', color: '#9334e6', icon: '📍' },
]

function selectProduct(p: typeof products[0]) {
  alert(`Selected: ${p.name} — API integration coming soon`)
}
</script>

<style scoped>
.products {
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
.brand { font-weight: 700; color: #1a73e8; font-size: 1.2rem; }
.nav-links { display: flex; gap: 1.25rem; flex: 1; }
.nav-links a { color: #555; text-decoration: none; font-size: 0.9rem; font-weight: 500; }
.nav-links a:hover, .nav-links a.router-link-active { color: #1a73e8; }
.user-badge { background: #e8f0fe; color: #1a73e8; padding: 0.3rem 0.8rem; border-radius: 20px; font-size: 0.8rem; font-weight: 600; }
.content { max-width: 960px; margin: 0 auto; padding: 2rem; }
h2 { margin: 0; font-size: 1.5rem; color: #222; }
.subtitle { color: #666; margin: 0.25rem 0 2rem; }
.product-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 1.25rem;
}
.product-card {
  background: white;
  border-radius: 12px;
  padding: 1.5rem;
  box-shadow: 0 2px 8px rgba(0,0,0,0.04);
  display: flex;
  flex-direction: column;
}
.icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.4rem;
  margin-bottom: 1rem;
}
.product-card h3 { margin: 0 0 0.35rem; font-size: 1.1rem; color: #222; }
.desc { margin: 0 0 0.75rem; font-size: 0.85rem; color: #777; flex: 1; }
.price { margin: 0 0 0.5rem; font-size: 1.35rem; font-weight: 700; color: #1a73e8; }
.price span { font-size: 0.85rem; font-weight: 400; color: #888; }
.tag {
  display: inline-block;
  background: #e8f0fe;
  color: #1a73e8;
  padding: 0.2rem 0.6rem;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 600;
  margin-bottom: 1rem;
  width: fit-content;
}
.btn {
  padding: 0.55rem;
  background: #1a73e8;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.9rem;
  margin-top: auto;
}
.btn:hover { background: #1557b0; }
</style>
