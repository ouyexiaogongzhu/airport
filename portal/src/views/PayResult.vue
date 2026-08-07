<template>
  <div class="page result-page">
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
      <div v-if="loading" class="pay-status-card">
        <div class="status-icon spinning">⏳</div>
        <h2>Checking Payment Status…</h2>
        <p class="status-hint">Please wait while we verify your payment.</p>
      </div>

      <div v-if="error" class="error-msg">{{ error }}</div>

      <!-- Success -->
      <div v-if="!loading && status === 'paid'" class="pay-status-card success">
        <div class="status-icon">✅</div>
        <h2>Payment Successful!</h2>
        <p class="status-hint">Your subscription has been activated. Enjoy the service!</p>
        <div class="success-actions">
          <button class="btn" @click="router.push('/account')">Go to Account</button>
          <button class="btn-outline" @click="router.push('/dashboard')">Dashboard</button>
        </div>
      </div>

      <!-- Failed -->
      <div v-if="!loading && status === 'failed'" class="pay-status-card failed">
        <div class="status-icon">❌</div>
        <h2>Payment Failed</h2>
        <p class="status-hint">{{ failReason || 'The payment was not completed. Please try again.' }}</p>
        <button class="btn" @click="router.push('/plans')">Try Again</button>
      </div>

      <!-- Pending / unknown — start polling -->
      <div v-if="!loading && status === 'pending'" class="pay-status-card">
        <div class="status-icon spinning">⏳</div>
        <h2>Payment Pending</h2>
        <p class="status-hint">Your payment is being processed. Checking for updates…</p>
        <div class="poll-status">
          <span class="poll-dot"></span>
          Checking… {{ Math.floor(pollElapsed / 1000) }}s
        </div>
      </div>

      <!-- Not found -->
      <div v-if="!loading && status === 'not_found'" class="pay-status-card">
        <div class="status-icon">❓</div>
        <h2>Order Not Found</h2>
        <p class="status-hint">We couldn't find this order. It may have expired or the ID is incorrect.</p>
        <button class="btn" @click="router.push('/plans')">Browse Plans</button>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api/index'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

type PayStatus = 'pending' | 'paid' | 'failed' | 'not_found' | 'loading'

const status = ref<PayStatus>('loading')
const loading = ref(true)
const error = ref('')
const failReason = ref('')
const pollElapsed = ref(0)

let pollTimer: ReturnType<typeof setInterval> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null

async function fetchOrderStatus() {
  // Get order_id from query param: ?order_id=xxx or ?id=xxx
  const orderId = (route.query.order_id as string) || (route.query.id as string) || ''
  if (!orderId) {
    error.value = 'No order ID found in URL. Please check your order status in Account.'
    loading.value = false
    return
  }
  try {
    const res = await api.get(`/user/orders/${orderId}`)
    const s = res.data.status
    if (s === 'paid' || s === 'completed' || s === 'active') {
      status.value = 'paid'
      loading.value = false
      stopPolling()
    } else if (s === 'failed' || s === 'cancelled' || s === 'expired') {
      status.value = 'failed'
      failReason.value = res.data.fail_reason || ''
      loading.value = false
      stopPolling()
    } else if (s === 'not_found') {
      status.value = 'not_found'
      loading.value = false
      stopPolling()
    } else {
      // Still pending
      status.value = 'pending'
      loading.value = false
    }
  } catch (e: any) {
    if (e.response?.status === 404) {
      status.value = 'not_found'
      loading.value = false
      stopPolling()
    } else {
      error.value = e.response?.data?.error || 'Failed to check order status'
      loading.value = false
      stopPolling()
    }
  }
}

function startPolling() {
  stopPolling()
  // Poll every 2s for up to 30s (short poll — user was already redirected from provider)
  pollTimer = setInterval(fetchOrderStatus, 2000)
  elapsedTimer = setInterval(() => {
    pollElapsed.value += 1000
    if (pollElapsed.value >= 30_000) {
      stopPolling()
    }
  }, 1000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
}

onMounted(async () => {
  await fetchOrderStatus()
  if (status.value === 'pending') {
    startPolling()
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.result-page {
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
.content { max-width: 560px; margin: 0 auto; padding: 2rem; }
.error-msg { color: #ff6b6b; background: rgba(255,107,107,0.1); border-radius: 8px; padding: 0.75rem 1rem; margin-bottom: 1rem; font-size: 0.9rem; }

.pay-status-card {
  background: #16213e;
  border-radius: 12px;
  padding: 2.5rem 2rem;
  text-align: center;
  border: 1px solid #0f3460;
}
.pay-status-card.success { border-color: #4caf50; }
.pay-status-card.failed { border-color: #ff6b6b; }

.status-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
  display: inline-block;
}
.status-icon.spinning {
  animation: pulse 1.5s ease-in-out infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 0.5; transform: scale(1); }
  50% { opacity: 1; transform: scale(1.15); }
}

h2 { margin: 0 0 0.5rem; font-size: 1.4rem; color: #f0f0f0; }
.status-hint { color: #a0a0b0; font-size: 0.9rem; margin: 0 0 1.5rem; line-height: 1.5; }

.poll-status {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  color: #a0a0b0;
  font-size: 0.85rem;
}
.poll-dot {
  width: 8px;
  height: 8px;
  background: #ffd54f;
  border-radius: 50%;
  animation: blink 1s step-end infinite;
}
@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.success-actions { display: flex; gap: 0.75rem; justify-content: center; }
.btn {
  padding: 0.6rem 1.5rem;
  background: #e94560;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 600;
  transition: background 0.2s;
}
.btn:hover { background: #d63851; }
.btn-outline {
  padding: 0.6rem 1.5rem;
  background: transparent;
  border: 1px solid #e94560;
  border-radius: 8px;
  color: #e94560;
  cursor: pointer;
  font-size: 0.9rem;
  transition: all 0.2s;
}
.btn-outline:hover { background: rgba(233,69,96,0.1); }
</style>
