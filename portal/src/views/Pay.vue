<template>
  <div class="page pay-page">
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
      <!-- Error state -->
      <div v-if="error" class="error-msg">{{ error }}</div>

      <!-- Payment pending — polling -->
      <div v-if="status === 'pending'" class="pay-status-card">
        <div class="status-icon spinning">⏳</div>
        <h2>Waiting for Payment</h2>
        <p class="status-hint">Complete the payment using the link or QR code below.</p>

        <!-- QR Code for USDT -->
        <div v-if="orderData?.payment_qr" class="qr-section">
          <QrCode :url="orderData.payment_qr" />
        </div>

        <!-- Payment link -->
        <div v-if="orderData?.payment_url" class="link-section">
          <p class="link-label">Payment link:</p>
          <a :href="orderData.payment_url" target="_blank" class="pay-link" @click="openPaymentWindow">
            {{ orderData.payment_url.length > 60 ? orderData.payment_url.slice(0, 60) + '…' : orderData.payment_url }}
          </a>
          <button class="btn-outline open-btn" @click="openPaymentWindow">
            Open Payment Page
          </button>
        </div>

        <div class="poll-status">
          <span class="poll-dot"></span>
          Checking payment status… {{ Math.floor(pollElapsed / 1000) }}s
        </div>
        <div v-if="pollElapsed > maxPollTime" class="timeout-warning">
          This is taking longer than usual. The page will auto-refresh. You can also check your order status in <router-link to="/account">Account</router-link>.
        </div>
      </div>

      <!-- Payment completed — success -->
      <div v-if="status === 'paid'" class="pay-status-card success">
        <div class="status-icon">✅</div>
        <h2>Payment Successful!</h2>
        <p class="status-hint">Your subscription is now active. You can start using the service immediately.</p>
        <div class="success-actions">
          <button class="btn" @click="router.push('/account')">Go to Account</button>
          <button class="btn-outline" @click="router.push('/dashboard')">Dashboard</button>
        </div>
      </div>

      <!-- Payment failed -->
      <div v-if="status === 'failed'" class="pay-status-card failed">
        <div class="status-icon">❌</div>
        <h2>Payment Failed</h2>
        <p class="status-hint">{{ failReason || 'The payment was not completed. Please try again.' }}</p>
        <button class="btn" @click="router.push('/plans')">Back to Plans</button>
      </div>

      <!-- Order not found -->
      <div v-if="status === 'not_found'" class="pay-status-card">
        <div class="status-icon">❓</div>
        <h2>Order Not Found</h2>
        <p class="status-hint">This order could not be found. It may have expired or been cancelled.</p>
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
import { clearApiCache } from '../api/cache'
import QrCode from '../components/QrCode.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

type PayStatus = 'pending' | 'paid' | 'failed' | 'not_found' | 'loading'

const status = ref<PayStatus>('loading')
const orderData = ref<any>(null)
const error = ref('')
const failReason = ref('')

// Polling
const pollInterval = 2000 // 2 seconds
const maxPollTime = 120_000 // 2 minutes max polling
const pollElapsed = ref(0)
let pollTimer: ReturnType<typeof setInterval> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null

async function fetchOrder() {
  const orderId = route.params.order_id as string
  if (!orderId) {
    error.value = 'No order ID provided'
    return
  }
  try {
    // Bypass the GET cache: this endpoint is polled every 2s and must reflect
    // live status transitions.
    const res = await api.get(`/user/orders/${orderId}`, { cache: { skipCache: true } })
    orderData.value = res.data
    const s = res.data.status
    if (s === 'paid' || s === 'completed' || s === 'active') {
      status.value = 'paid'
      clearApiCache()
      stopPolling()
    } else if (s === 'failed' || s === 'cancelled' || s === 'expired') {
      status.value = 'failed'
      failReason.value = res.data.fail_reason || ''
      stopPolling()
    } else if (s === 'not_found' || s === '404') {
      status.value = 'not_found'
      stopPolling()
    } else {
      status.value = 'pending'
    }
  } catch (e: any) {
    if (e.response?.status === 404) {
      status.value = 'not_found'
      stopPolling()
    } else {
      error.value = e.response?.data?.error || 'Failed to check order status'
      stopPolling()
    }
  }
}

function openPaymentWindow() {
  if (orderData.value?.payment_url) {
    window.open(orderData.value.payment_url, '_blank', 'noopener,noreferrer')
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(fetchOrder, pollInterval)
  elapsedTimer = setInterval(() => {
    pollElapsed.value += 1000
    if (pollElapsed.value >= maxPollTime) {
      stopPolling()
    }
  }, 1000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
}

onMounted(async () => {
  await fetchOrder()
  if (status.value === 'pending') {
    startPolling()
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.pay-page {
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

/* QR */
.qr-section {
  background: #0f3460;
  border-radius: 8px;
  padding: 1rem;
  margin-bottom: 1.25rem;
  display: inline-block;
}

/* Link */
.link-section { margin-bottom: 1.5rem; }
.link-label { color: #a0a0b0; font-size: 0.85rem; margin-bottom: 0.5rem; }
.pay-link {
  display: block;
  color: #1a73e8;
  word-break: break-all;
  font-size: 0.85rem;
  margin-bottom: 0.75rem;
  text-decoration: none;
}
.pay-link:hover { text-decoration: underline; }
.open-btn {
  padding: 0.55rem 1.2rem;
  border: 1px solid #e94560;
  border-radius: 8px;
  color: #e94560;
  background: transparent;
  cursor: pointer;
  font-size: 0.9rem;
  transition: all 0.2s;
}
.open-btn:hover { background: rgba(233,69,96,0.1); }

/* Poll status */
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

.timeout-warning { margin-top: 1rem; color: #ffd54f; font-size: 0.85rem; }
.timeout-warning a { color: #e94560; }

/* Success actions */
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
