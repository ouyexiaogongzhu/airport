<template>
  <div class="page products">
    <nav class="topbar">
      <span class="brand">RFPlay</span>
      <div class="nav-links">
        <router-link to="/dashboard">Dashboard</router-link>
        <router-link to="/plans">Plans</router-link>
        <router-link to="/account">Account</router-link>
        <a href="#" @click.prevent="auth.logout(); $router.push('/')">Logout</a>
      </div>
      <span class="user-badge">{{ auth.username }}</span>
    </nav>

    <main class="content">
      <h2>Plans &amp; Pricing</h2>
      <p class="subtitle">Choose a plan that fits your needs</p>

      <div v-if="loading" class="loading">Loading plans…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <div v-if="plans.length" class="plan-grid">
        <div v-for="p in plans" :key="p.id" class="plan-card">
          <div class="plan-header">
            <h3>{{ p.name }}</h3>
            <p class="price">${{ formatPrice(p.price) }}<span v-if="p.duration_days"> / {{ formatDuration(p.duration_days) }}</span></p>
          </div>
          <div class="plan-features">
            <div class="feature">
              <span class="feature-label">Traffic</span>
              <span class="feature-value">{{ formatTraffic(p.traffic_bytes) }}</span>
            </div>
            <div class="feature">
              <span class="feature-label">Duration</span>
              <span class="feature-value">{{ formatDuration(p.duration_days) }}</span>
            </div>
            <div class="feature">
              <span class="feature-label">Speed Limit</span>
              <span class="feature-value">{{ formatSpeed(p.speed_limit_bps) }}</span>
            </div>
            <div v-if="p.description" class="feature-desc">{{ p.description }}</div>
          </div>
          <button class="btn buy-btn" @click="goCheckout(p)">Purchase</button>
        </div>
      </div>

      <div v-if="!plans.length && !loading && !error" class="empty">
        <p>No plans available at the moment. Please check back later.</p>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api/index.js'

const auth = useAuthStore()
const router = useRouter()

interface Plan {
  id: string
  name: string
  description?: string
  price: number
  traffic_bytes: number
  duration_days: number
  speed_limit_bps: number
  [key: string]: any
}

const plans = ref<Plan[]>([])
const loading = ref(true)
const error = ref('')

async function fetchPlans() {
  loading.value = true
  error.value = ''
  try {
    // Try authenticated first, fall back to public
    const res = await api.get('/web/plans')
    plans.value = Array.isArray(res.data) ? res.data : (res.data.plans || [])
  } catch (e: any) {
    if (!e.response || e.response.status === 401) {
      // Public endpoint — try without auth
      try {
        const res = await api.get('/web/plans', {
          headers: { Authorization: '' }
        })
        plans.value = Array.isArray(res.data) ? res.data : (res.data.plans || [])
      } catch (e2: any) {
        error.value = e2.response?.data?.error || 'Failed to load plans'
      }
    } else {
      error.value = e.response?.data?.error || 'Failed to load plans'
    }
  } finally {
    loading.value = false
  }
}

function formatPrice(cents: number): string {
  if (cents >= 100) return (cents / 100).toFixed(2)
  return String(cents)
}

function formatTraffic(bytes: number): string {
  if (!bytes || bytes <= 0) return 'Unlimited'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let b = bytes
  while (b >= 1024 && i < units.length - 1) { b /= 1024; i++ }
  return `${b.toFixed(i > 1 ? 1 : 0)} ${units[i]}`
}

function formatDuration(days: number): string {
  if (days <= 0) return '—'
  if (days >= 365) return `${Math.floor(days / 365)} year${days >= 730 ? 's' : ''}`
  if (days >= 30) return `${Math.floor(days / 30)} month${days >= 60 ? 's' : ''}`
  return `${days} day${days > 1 ? 's' : ''}`
}

function formatSpeed(bps: number): string {
  if (!bps || bps <= 0) return 'No limit'
  if (bps >= 1_000_000_000) return `${(bps / 1_000_000_000).toFixed(1)} Gbps`
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(0)} Mbps`
  if (bps >= 1_000) return `${(bps / 1_000).toFixed(0)} Kbps`
  return `${bps} bps`
}

function goCheckout(p: Plan) {
  router.push(`/checkout/${p.id}`)
}

onMounted(fetchPlans)
</script>

<style scoped>
.products {
  min-height: 100vh;
  background: #1a1a2e;
  color: #e0e0e0;
}
.topbar {
  display: flex;
  align-items: center;
  padding: 0.75rem 2rem;
  background: #16213e;
  border-bottom: 1px solid #0f3460;
  gap: 2rem;
}
.brand { font-weight: 700; color: #e94560; font-size: 1.2rem; }
.nav-links { display: flex; gap: 1.25rem; flex: 1; }
.nav-links a { color: #a0a0b0; text-decoration: none; font-size: 0.9rem; font-weight: 500; }
.nav-links a:hover, .nav-links a.router-link-active { color: #e94560; }
.user-badge { background: rgba(233,69,96,0.15); color: #e94560; padding: 0.3rem 0.8rem; border-radius: 20px; font-size: 0.8rem; font-weight: 600; }
.content { max-width: 960px; margin: 0 auto; padding: 2rem; }
h2 { margin: 0; font-size: 1.5rem; color: #f0f0f0; }
.subtitle { color: #a0a0b0; margin: 0.25rem 0 2rem; font-size: 0.9rem; }
.loading { color: #a0a0b0; font-size: 0.9rem; padding: 1rem 0; }
.error-msg { color: #ff6b6b; background: rgba(255,107,107,0.1); border-radius: 8px; padding: 0.75rem 1rem; margin-bottom: 1rem; font-size: 0.9rem; }
.empty { color: #a0a0b0; text-align: center; padding: 3rem 0; font-size: 0.9rem; }

.plan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 1.25rem;
}
.plan-card {
  background: #16213e;
  border-radius: 12px;
  border: 1px solid #0f3460;
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.plan-card:hover {
  border-color: #e94560;
  box-shadow: 0 4px 20px rgba(233,69,96,0.1);
}
.plan-header { margin-bottom: 1.25rem; }
.plan-header h3 { margin: 0 0 0.35rem; font-size: 1.15rem; color: #f0f0f0; }
.price { margin: 0; font-size: 1.5rem; font-weight: 700; color: #e94560; }
.price span { font-size: 0.85rem; font-weight: 400; color: #a0a0b0; }
.plan-features { flex: 1; display: flex; flex-direction: column; gap: 0.6rem; margin-bottom: 1.5rem; }
.feature { display: flex; justify-content: space-between; align-items: center; }
.feature-label { color: #a0a0b0; font-size: 0.85rem; }
.feature-value { color: #e0e0e0; font-size: 0.9rem; font-weight: 500; }
.feature-desc { color: #a0a0b0; font-size: 0.8rem; font-style: italic; margin-top: 0.3rem; }
.buy-btn {
  padding: 0.65rem;
  background: #e94560;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.95rem;
  font-weight: 600;
  transition: background 0.2s;
  margin-top: auto;
}
.buy-btn:hover { background: #d63851; }
</style>
