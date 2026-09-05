<template>
  <div class="page guide-page">
    <nav class="topbar">
      <span class="brand">RFPlay</span>
      <div class="nav-links">
        <router-link to="/dashboard">Dashboard</router-link>
        <router-link to="/account">Account</router-link>
        <router-link to="/products">Products</router-link>
        <router-link to="/subscription">Subscription</router-link>
        <a href="#" @click.prevent="auth.logout(); $router.push('/')">Logout</a>
      </div>
      <span class="user-badge">{{ auth.username }}</span>
    </nav>

    <main class="content">
      <h2>Setup Guide</h2>
      <p class="subtitle">Follow the steps for your device to get connected.</p>

      <!-- Subscription links (Clash + Base64) -->
      <div v-if="clashUrl" class="sub-links">
        <div class="sub-link-row">
          <div class="sub-link-info">
            <span class="sub-link-label">Clash Subscription — Clash Verge (clash-verge-rev) / Stash</span>
            <code class="sub-link-url">{{ clashUrl }}</code>
          </div>
          <button class="method-btn" @click="copyUrl(clashUrl, 'clash')">
            {{ copiedKind === 'clash' ? 'Copied!' : 'Copy' }}
          </button>
        </div>
        <div class="sub-link-row">
          <div class="sub-link-info">
            <span class="sub-link-label">Base64 Subscription — V2rayNG / v2rayA / OpenWrt</span>
            <code class="sub-link-url">{{ base64Url }}</code>
          </div>
          <button class="method-btn" @click="copyUrl(base64Url, 'base64')">
            {{ copiedKind === 'base64' ? 'Copied!' : 'Copy' }}
          </button>
        </div>
      </div>

      <div class="tabs">
        <button
          v-for="t in tabs"
          :key="t.key"
          :class="['tab', { active: activeTab === t.key }]"
          @click="activeTab = t.key"
        >
          {{ t.label }}
        </button>
      </div>

      <!-- V2rayNG -->
      <div v-if="activeTab === 'v2rayng'" class="guide-section">
        <div class="guide-header">
          <span class="platform-badge android">Android</span>
          <h3>V2rayNG</h3>
        </div>
        <div class="install-methods">
          <a href="https://play.google.com/store/apps/details?id=com.v2ray.ang" target="_blank" class="method-btn">Google Play</a>
          <a href="https://github.com/2dust/v2rayNG/releases" target="_blank" class="method-btn">APK Download</a>
        </div>
        <ol class="steps">
          <li>Open V2rayNG app</li>
          <li>Tap <strong>+</strong> icon in the top-right corner</li>
          <li>Select <strong>Import subscription from clipboard</strong></li>
          <li>Paste your <strong>Base64</strong> subscription URL (copied above or from the Account page)</li>
          <li>Tap <strong>✓</strong> to confirm</li>
          <li>Select a node and tap <strong>Connect</strong></li>
        </ol>
        <div class="screenshot-placeholder">
          <span>📱 V2rayNG Screenshot</span>
        </div>
      </div>

      <!-- Shadowrocket -->
      <div v-if="activeTab === 'shadowrocket'" class="guide-section">
        <div class="guide-header">
          <span class="platform-badge ios">iOS</span>
          <h3>Shadowrocket</h3>
        </div>
        <div class="install-methods">
          <a href="https://apps.apple.com/app/shadowrocket/id932747118" target="_blank" class="method-btn">App Store</a>
        </div>
        <ol class="steps">
          <li>Open Shadowrocket app</li>
          <li>Tap the <strong>+</strong> icon in the top-right corner</li>
          <li>Select type: <strong>Subscribe</strong></li>
          <li>Paste your subscription URL</li>
          <li>Tap <strong>Save</strong> (top-right)</li>
          <li>Select a node and toggle <strong>Connect</strong></li>
        </ol>
        <div class="screenshot-placeholder">
          <span>📱 Shadowrocket Screenshot</span>
        </div>
      </div>

      <!-- Clash Verge -->
      <div v-if="activeTab === 'clash-verge'" class="guide-section">
        <div class="guide-header">
          <span class="platform-badge desktop">Desktop</span>
          <h3>Clash Verge</h3>
        </div>
        <div class="install-methods">
          <a href="https://github.com/clash-verge-rev/clash-verge-rev/releases" target="_blank" class="method-btn">GitHub Releases</a>
        </div>
        <ol class="steps">
          <li>Download and install Clash Verge for your OS</li>
          <li>Open Clash Verge → go to <strong>Profiles</strong></li>
          <li>Click <strong>Import</strong> (or paste URL)</li>
          <li>Paste your Clash subscription URL (<code>/clash</code>)</li>
          <li>Click <strong>Import</strong> to confirm</li>
          <li>Go to <strong>Proxies</strong> and select a node</li>
          <li>Toggle <strong>System Proxy</strong> or <strong>TUN Mode</strong></li>
        </ol>
        <div class="screenshot-placeholder">
          <span>🖥️ Clash Verge Screenshot</span>
        </div>
      </div>

      <!-- Sing-box -->
      <div v-if="activeTab === 'singbox'" class="guide-section">
        <div class="guide-header">
          <span class="platform-badge desktop">Desktop</span>
          <h3>Sing-box</h3>
        </div>
        <div class="install-methods">
          <a href="https://github.com/SagerNet/sing-box/releases" target="_blank" class="method-btn">GitHub Releases</a>
        </div>
        <ol class="steps">
          <li>Download and install Sing-box</li>
          <li>Open the config directory</li>
          <li>In the app, go to <strong>Remote File</strong> or <strong>Subscription</strong></li>
          <li>Paste your Sing-box subscription URL (<code>/singbox</code>)</li>
          <li>Save and apply the configuration</li>
          <li>Enable the proxy</li>
        </ol>
        <div class="screenshot-placeholder">
          <span>🖥️ Sing-box Screenshot</span>
        </div>
      </div>

      <!-- Flutter App -->
      <div v-if="activeTab === 'flutter'" class="guide-section">
        <div class="guide-header">
          <span class="platform-badge mobile">Mobile</span>
          <h3>RFPlay App (Flutter)</h3>
        </div>
        <div class="install-methods">
          <span class="method-btn disabled">Coming Soon</span>
        </div>
        <ol class="steps">
          <li>Open the RFPlay app</li>
          <li>Tap <strong>Token Import</strong> (or skip to Account tab)</li>
          <li>Paste your <strong>Client Token</strong> from the Account page</li>
          <li>Tap <strong>Connect</strong></li>
          <li>All set — no subscription URL needed!</li>
        </ol>
        <div class="screenshot-placeholder">
          <span>📱 RFPlay App Screenshot</span>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '../stores/auth'
import api from '../api/index'
import { buildSubscriptionUrl } from '../utils/subscriptionUrl'

const auth = useAuthStore()

const activeTab = ref('v2rayng')

const tabs = [
  { key: 'v2rayng', label: 'V2rayNG' },
  { key: 'shadowrocket', label: 'Shadowrocket' },
  { key: 'clash-verge', label: 'Clash Verge' },
  { key: 'singbox', label: 'Sing-box' },
  { key: 'flutter', label: 'Flutter App' },
]

// Subscription links for the copy buttons at the top of the guide. Hidden
// until the client token loads (not logged in / no token yet).
const clientToken = ref('')
const clashUrl = computed(() => buildSubscriptionUrl(clientToken.value, 'clash'))
const base64Url = computed(() => buildSubscriptionUrl(clientToken.value))
const copiedKind = ref('')

async function copyUrl(url: string, kind: 'clash' | 'base64') {
  if (!url) return
  await navigator.clipboard.writeText(url)
  copiedKind.value = kind
  setTimeout(() => { copiedKind.value = '' }, 2000)
}

onMounted(async () => {
  try {
    const res = await api.get('/user/profile')
    clientToken.value = res.data.client_token || ''
  } catch {
    // Links stay hidden for unauthenticated visitors
  }
})
</script>

<style scoped>
.guide-page {
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
.subtitle { color: #a0a0b0; margin: 0.25rem 0 1.5rem; font-size: 0.9rem; }
.sub-links {
  background: #16213e;
  border-radius: 12px;
  padding: 1.25rem 1.5rem;
  margin-bottom: 2rem;
  display: grid;
  gap: 1rem;
}
.sub-link-row { display: flex; align-items: center; gap: 1rem; }
.sub-link-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 0.35rem; }
.sub-link-label { color: #a0a0b0; font-size: 0.8rem; font-weight: 600; }
.sub-link-url {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.85rem;
  color: #e0e0e0;
  word-break: break-all;
  user-select: all;
}
.tabs { display: flex; gap: 0.5rem; margin-bottom: 2rem; flex-wrap: wrap; }
.tab {
  padding: 0.5rem 1rem;
  border: 1px solid #0f3460;
  border-radius: 20px;
  background: transparent;
  color: #a0a0b0;
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.2s;
}
.tab:hover { background: rgba(233,69,96,0.1); border-color: #e94560; }
.tab.active { background: #e94560; color: white; border-color: #e94560; }
.guide-section { background: #16213e; border-radius: 12px; padding: 1.5rem; }
.guide-header { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 1rem; }
.guide-header h3 { margin: 0; font-size: 1.1rem; color: #f0f0f0; }
.platform-badge {
  display: inline-block;
  padding: 0.2rem 0.6rem;
  border-radius: 6px;
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
}
.platform-badge.android { background: #4caf50; color: white; }
.platform-badge.ios { background: #2196f3; color: white; }
.platform-badge.desktop { background: #ff9800; color: white; }
.platform-badge.mobile { background: #9c27b0; color: white; }
.install-methods { display: flex; gap: 0.75rem; margin-bottom: 1.25rem; flex-wrap: wrap; }
.method-btn {
  display: inline-block;
  padding: 0.4rem 0.9rem;
  background: #0f3460;
  border-radius: 8px;
  color: #e0e0e0;
  text-decoration: none;
  font-size: 0.85rem;
  transition: background 0.2s;
}
.method-btn:hover { background: #1a5276; }
.method-btn.disabled { opacity: 0.5; cursor: not-allowed; }
.steps { padding-left: 1.5rem; margin: 0 0 1.25rem; }
.steps li { margin-bottom: 0.6rem; line-height: 1.5; color: #c0c0d0; font-size: 0.9rem; }
.steps li strong { color: #e94560; }
code { background: #0f3460; padding: 0.1rem 0.4rem; border-radius: 4px; font-size: 0.85rem; }
.screenshot-placeholder {
  border: 2px dashed #0f3460;
  border-radius: 10px;
  padding: 3rem 2rem;
  text-align: center;
  color: #555;
  font-size: 0.9rem;
}
</style>
