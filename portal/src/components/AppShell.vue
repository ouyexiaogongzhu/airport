<template>
  <div class="app-shell">
    <nav class="topbar">
      <router-link to="/dashboard" class="brand">RFPlay</router-link>
      <div class="nav-links">
        <router-link to="/dashboard">Dashboard</router-link>
        <router-link to="/account">Account</router-link>
      </div>
      <div class="user-menu" ref="menuRef">
        <button
          type="button"
          class="user-badge"
          :aria-expanded="menuOpen"
          aria-haspopup="menu"
          @click="menuOpen = !menuOpen"
        >
          {{ auth.username || 'Account' }}
          <span class="chevron" :class="{ open: menuOpen }">▾</span>
        </button>
        <div v-if="menuOpen" class="dropdown" role="menu">
          <button type="button" class="dropdown-item" role="menuitem" @click="onLogout">
            Logout
          </button>
        </div>
      </div>
    </nav>
    <slot />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const menuOpen = ref(false)
const menuRef = ref<HTMLElement | null>(null)

function onDocClick(e: MouseEvent) {
  if (!menuRef.value?.contains(e.target as Node)) {
    menuOpen.value = false
  }
}

async function onLogout() {
  menuOpen.value = false
  await auth.logout()
  router.push('/login')
}

onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<style scoped>
.app-shell {
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
.brand {
  font-weight: 700;
  color: #e94560;
  font-size: 1.2rem;
  text-decoration: none;
}
.nav-links {
  display: flex;
  gap: 1.25rem;
  flex: 1;
}
.nav-links a {
  color: #a0a0b0;
  text-decoration: none;
  font-size: 0.9rem;
  font-weight: 500;
}
.nav-links a:hover,
.nav-links a.router-link-active {
  color: #e94560;
}
.user-menu {
  position: relative;
}
.user-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  background: rgba(233, 69, 96, 0.15);
  color: #e94560;
  padding: 0.3rem 0.8rem;
  border-radius: 20px;
  font-size: 0.8rem;
  font-weight: 600;
  border: none;
  cursor: pointer;
  font-family: inherit;
}
.user-badge:hover {
  background: rgba(233, 69, 96, 0.25);
}
.chevron {
  font-size: 0.7rem;
  transition: transform 0.15s ease;
}
.chevron.open {
  transform: rotate(180deg);
}
.dropdown {
  position: absolute;
  right: 0;
  top: calc(100% + 0.4rem);
  min-width: 140px;
  background: #16213e;
  border: 1px solid #0f3460;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
  padding: 0.35rem;
  z-index: 50;
}
.dropdown-item {
  display: block;
  width: 100%;
  text-align: left;
  background: transparent;
  border: none;
  color: #e0e0e0;
  padding: 0.55rem 0.75rem;
  border-radius: 6px;
  font-size: 0.85rem;
  cursor: pointer;
  font-family: inherit;
}
.dropdown-item:hover {
  background: rgba(233, 69, 96, 0.15);
  color: #e94560;
}
@media (max-width: 640px) {
  .topbar {
    padding: 0.75rem 1rem;
    gap: 1rem;
  }
}
</style>
