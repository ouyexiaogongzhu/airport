<template>
  <section class="plans-section" id="plans">
    <h2 class="section-title">Plans &amp; Pricing</h2>
    <p class="subtitle">Choose a plan that fits your needs</p>

    <div v-if="loading" class="loading">Loading plans…</div>
    <div v-if="error" class="error-msg">{{ error }}</div>

    <div v-if="hasActiveSub" class="renew-banner">
      <span class="renew-icon">🔄</span>
      <span>You have an active subscription. Purchasing a plan will <strong>extend</strong> your current subscription period.</span>
    </div>

    <div v-if="plans.length" class="plan-grid">
      <div v-for="p in plans" :key="p.id" class="plan-card">
        <div class="plan-header">
          <h3>{{ p.name }}</h3>
          <p class="price">{{ formatPrice(p.price, p.currency) }}<span v-if="p.duration_days"> / {{ formatDuration(p.duration_days) }}</span></p>
        </div>
        <div class="plan-features">
          <div class="feature">
            <span class="feature-label">Traffic</span>
            <span class="feature-value">{{ formatTraffic(p.traffic_bytes) }}<template v-if="p.traffic_bytes > 0"> / 30 days</template></span>
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
        <button class="btn buy-btn" type="button" @click="goCheckout(p)">
          {{ hasActiveSub ? 'Renew / Extend' : 'Purchase' }}
        </button>
      </div>
    </div>

    <div v-if="!plans.length && !loading && !error" class="empty">
      <p>No plans available at the moment. Please check back later.</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api/index'
import { formatPrice } from '../utils/price'

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

const props = withDefaults(defineProps<{
  /** When provided, skip a second profile fetch (e.g. Dashboard already loaded it). */
  subscriptionStatus?: string | null
}>(), {
  subscriptionStatus: undefined,
})

const router = useRouter()
const plans = ref<Plan[]>([])
const loading = ref(true)
const error = ref('')
const profile = ref<any>({})

const hasActiveSub = computed(() => {
  if (props.subscriptionStatus !== undefined && props.subscriptionStatus !== null) {
    return props.subscriptionStatus === 'active'
  }
  return profile.value.subscription_status === 'active'
})

async function fetchProfile() {
  if (props.subscriptionStatus !== undefined) return
  try {
    const res = await api.get('/user/profile')
    profile.value = res.data
  } catch {
    // Non-critical — just won't show renewal banner
  }
}

async function fetchPlans() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/products')
    plans.value = Array.isArray(res.data) ? res.data : (res.data.products || [])
  } catch (e: any) {
    if (!e.response || e.response.status === 401) {
      try {
        const res = await api.get('/products', {
          headers: { Authorization: '' },
        })
        plans.value = Array.isArray(res.data) ? res.data : (res.data.products || [])
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

onMounted(() => {
  fetchPlans()
  fetchProfile()
})
</script>

<style scoped>
.plans-section {
  scroll-margin-top: 1.5rem;
}
.section-title { margin: 0; font-size: 1.5rem; color: #f0f0f0; }
.subtitle { color: #a0a0b0; margin: 0.25rem 0 2rem; font-size: 0.9rem; }
.loading { color: #a0a0b0; font-size: 0.9rem; padding: 1rem 0; }
.error-msg { color: #ff6b6b; background: rgba(255,107,107,0.1); border-radius: 8px; padding: 0.75rem 1rem; margin-bottom: 1rem; font-size: 0.9rem; }
.empty { color: #a0a0b0; text-align: center; padding: 3rem 0; font-size: 0.9rem; }

.renew-banner {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  background: linear-gradient(135deg, #1b4332, #16213e);
  border: 1px solid #68d391;
  border-radius: 10px;
  padding: 0.85rem 1.25rem;
  margin-bottom: 1.5rem;
  font-size: 0.9rem;
  color: #e0e0e0;
}
.renew-icon { font-size: 1.2rem; }
.renew-banner strong { color: #68d391; }

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
