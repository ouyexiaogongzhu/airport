<template>
  <div class="page checkout-page">
    <nav class="topbar">
      <span class="brand">RFPlay</span>
      <div class="nav-links">
        <router-link to="/dashboard">Dashboard</router-link>
        <router-link to="/plans">Plans</router-link>
        <router-link to="/subscription">Subscription</router-link>
        <router-link to="/account">Account</router-link>
        <a href="#" @click.prevent="auth.logout(); $router.push('/')">Logout</a>
      </div>
      <span class="user-badge">{{ auth.username }}</span>
    </nav>

    <main class="content">
      <div v-if="loading" class="loading">Loading plan details…</div>
      <div v-if="error" class="error-msg">{{ error }}</div>

      <template v-if="plan">
        <h2>Checkout</h2>
        <p class="subtitle">Review your order and select a payment method.</p>

        <!-- Renewal notice -->
        <div v-if="hasActiveSub" class="renew-notice">
          <span class="renew-icon">🔄</span>
          <div class="renew-text">
            <strong>Renewal Order</strong>
            <p>You have an active subscription. This purchase will <strong>extend</strong> your current subscription period.</p>
          </div>
        </div>

        <div class="checkout-layout">
          <!-- Order Summary -->
          <div class="card summary-card">
            <h3>Order Summary</h3>
            <div class="summary-row">
              <span class="label">Plan</span>
              <span class="value">{{ plan.name }}</span>
            </div>
            <div v-if="plan.description" class="summary-row">
              <span class="label">Description</span>
              <span class="value">{{ plan.description }}</span>
            </div>
            <div class="summary-row">
              <span class="label">Traffic</span>
              <span class="value">{{ formatTraffic(plan.traffic_bytes) }}</span>
            </div>
            <div class="summary-row">
              <span class="label">Duration</span>
              <span class="value">{{ formatDuration(plan.duration_days) }}</span>
            </div>
            <div class="summary-row">
              <span class="label">Speed Limit</span>
              <span class="value">{{ formatSpeed(plan.speed_limit_bps) }}</span>
            </div>
            <div class="summary-divider"></div>
            <div class="summary-row total">
              <span class="label">Total</span>
              <span class="value price">${{ formatPrice(plan.price) }}</span>
            </div>
          </div>

          <!-- Payment Method -->
          <div class="card payment-card">
            <h3>Payment Method</h3>
            <div class="payment-options">
              <label
                :class="['payment-option', { selected: selectedProvider === 'bepusdt' }]"
                @click="selectedProvider = 'bepusdt'"
              >
                <input type="radio" name="provider" value="bepusdt" v-model="selectedProvider" />
                <span class="option-icon">₮</span>
                <div class="option-info">
                  <span class="option-name">USDT (TRC20 / BEP20)</span>
                  <span class="option-desc">Pay with cryptocurrency via BEpusdt</span>
                </div>
                <span class="option-check">✓</span>
              </label>
              <label
                :class="['payment-option', { selected: selectedProvider === 'payoneer' }]"
                @click="selectedProvider = 'payoneer'"
              >
                <input type="radio" name="provider" value="payoneer" v-model="selectedProvider" />
                <span class="option-icon">💳</span>
                <div class="option-info">
                  <span class="option-name">Credit / Debit Card</span>
                  <span class="option-desc">Visa, Mastercard via Payoneer Checkout</span>
                </div>
                <span class="option-check">✓</span>
              </label>
            </div>
            <button
              class="btn pay-btn"
              @click="placeOrder"
              :disabled="submitting"
            >
              {{ submitting ? 'Creating Order…' : `Pay $${formatPrice(plan.price)}` }}
            </button>
            <div v-if="submitError" class="error-msg" style="margin-top: 1rem;">{{ submitError }}</div>
          </div>
        </div>
      </template>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api/index'

const auth = useAuthStore()
const route = useRoute()
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

const plan = ref<Plan | null>(null)
const loading = ref(true)
const error = ref('')
const selectedProvider = ref('bepusdt')
const submitting = ref(false)
const submitError = ref('')

// Subscription info for renewal notice
const profile = ref<any>({})
const hasActiveSub = ref(false)

async function fetchProfile() {
  try {
    const res = await api.get('/user/profile')
    profile.value = res.data
    hasActiveSub.value = res.data.subscription_status === 'active'
  } catch {
    // Non-critical
  }
}

async function fetchPlan() {
  loading.value = true
  error.value = ''
  try {
    // Fetch all plans and find the one matching plan_id
    const res = await api.get('/products')
    const plans: Plan[] = Array.isArray(res.data) ? res.data : (res.data.products || [])
    const found = plans.find(p => String(p.id) === route.params.plan_id)
    if (found) {
      plan.value = found
    } else {
      error.value = 'Plan not found'
    }
  } catch (e: any) {
    error.value = e.response?.data?.error || 'Failed to load plan details'
  } finally {
    loading.value = false
  }
}

async function placeOrder() {
  submitting.value = true
  submitError.value = ''
  try {
    const res = await api.post('/user/orders', {
      product_id: route.params.plan_id,
      provider: selectedProvider.value,
    })
    const order = res.data
    // If there's a payment_url, redirect the browser to the payment page
    if (order.payment_url) {
      // Save order_id for /pay page in case redirect fails
      sessionStorage.setItem('pay_order_id', order.order_id)
      window.location.href = order.payment_url
    } else {
      // No redirect URL — go to polling page
      router.push(`/pay/${order.order_id}`)
    }
  } catch (e: any) {
    submitError.value = e.response?.data?.error || 'Failed to create order. Please try again.'
  } finally {
    submitting.value = false
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

onMounted(() => {
  fetchPlan()
  fetchProfile()
})
</script>

<style scoped>
.checkout-page {
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
.content { max-width: 800px; margin: 0 auto; padding: 2rem; }
h2 { margin: 0; font-size: 1.5rem; color: #f0f0f0; }
.subtitle { color: #a0a0b0; margin: 0.25rem 0 2rem; font-size: 0.9rem; }
.loading { color: #a0a0b0; font-size: 0.9rem; padding: 1rem 0; }
.error-msg { color: #ff6b6b; background: rgba(255,107,107,0.1); border-radius: 8px; padding: 0.75rem 1rem; margin-bottom: 1rem; font-size: 0.9rem; }

/* Renewal notice */
.renew-notice {
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  background: linear-gradient(135deg, #1b4332, #16213e);
  border: 1px solid #68d391;
  border-radius: 10px;
  padding: 1rem 1.25rem;
  margin-bottom: 1.5rem;
}
.renew-icon { font-size: 1.3rem; line-height: 1.4; }
.renew-text { flex: 1; }
.renew-text strong { color: #68d391; font-size: 0.95rem; }
.renew-text p { margin: 0.25rem 0 0; color: #a0a0b0; font-size: 0.85rem; line-height: 1.4; }
.renew-text p strong { color: #e0e0e0; }

.checkout-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.5rem;
  align-items: start;
}
@media (max-width: 700px) {
  .checkout-layout { grid-template-columns: 1fr; }
}

.card {
  background: #16213e;
  border-radius: 12px;
  padding: 1.5rem;
  border: 1px solid #0f3460;
}
.card h3 { margin: 0 0 1.25rem; font-size: 1rem; color: #f0f0f0; }

/* Summary */
.summary-row { display: flex; justify-content: space-between; align-items: center; padding: 0.4rem 0; }
.summary-row .label { color: #a0a0b0; font-size: 0.85rem; }
.summary-row .value { color: #e0e0e0; font-size: 0.9rem; font-weight: 500; text-align: right; }
.summary-divider { height: 1px; background: #0f3460; margin: 0.75rem 0; }
.total .value.price { color: #e94560; font-size: 1.2rem; font-weight: 700; }

/* Payment Options */
.payment-options { display: flex; flex-direction: column; gap: 0.75rem; margin-bottom: 1.5rem; }
.payment-option {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem;
  border-radius: 8px;
  border: 1px solid #0f3460;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
}
.payment-option:hover { border-color: #1a5276; background: rgba(15,52,96,0.3); }
.payment-option.selected { border-color: #e94560; background: rgba(233,69,96,0.08); }
.payment-option input { display: none; }
.option-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.2rem;
  background: #0f3460;
  flex-shrink: 0;
}
.payment-option.selected .option-icon { background: rgba(233,69,96,0.2); }
.option-info { flex: 1; display: flex; flex-direction: column; }
.option-name { font-size: 0.9rem; color: #e0e0e0; font-weight: 500; }
.option-desc { font-size: 0.8rem; color: #a0a0b0; margin-top: 0.1rem; }
.option-check {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: 2px solid #0f3460;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.75rem;
  color: transparent;
  flex-shrink: 0;
}
.payment-option.selected .option-check {
  background: #e94560;
  border-color: #e94560;
  color: white;
}

.pay-btn {
  width: 100%;
  padding: 0.75rem;
  background: #e94560;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 1rem;
  font-weight: 600;
  transition: background 0.2s;
}
.pay-btn:hover:not(:disabled) { background: #d63851; }
.pay-btn:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
