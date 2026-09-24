<template>
  <div v-if="!auth.bootstrapped" class="boot-shell" aria-busy="true" aria-label="Loading">
    <div class="boot-inner">
      <span class="boot-brand">RFPlay</span>
      <span class="boot-pulse" />
    </div>
  </div>
  <router-view v-else />
</template>

<script setup lang="ts">
import { onErrorCaptured } from 'vue'
import { useAuthStore } from './stores/auth'

const auth = useAuthStore()

// NOTE: Vue's template engine auto-escapes interpolated values ({{ }}),
// preventing XSS from user-supplied data. No additional sanitization is needed
// for template interpolation. Only use v-html with trusted content.

onErrorCaptured((err, _instance, info) => {
  console.error('[Portal Error Boundary]', info, err)
  return false // prevent propagation
})
</script>

<style>
html,
body,
#app {
  margin: 0;
  min-height: 100%;
  background: #1a1a2e;
  color: #e0e0e0;
}
</style>

<style scoped>
.boot-shell {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background:
    radial-gradient(ellipse 80% 50% at 50% -10%, rgba(233, 69, 96, 0.12), transparent 55%),
    linear-gradient(180deg, #16213e 0%, #1a1a2e 55%, #12121f 100%);
}
.boot-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1.25rem;
}
.boot-brand {
  font-weight: 700;
  font-size: 1.75rem;
  letter-spacing: 0.04em;
  color: #e94560;
}
.boot-pulse {
  width: 2rem;
  height: 2rem;
  border-radius: 50%;
  border: 2px solid rgba(233, 69, 96, 0.25);
  border-top-color: #e94560;
  animation: boot-spin 0.8s linear infinite;
}
@keyframes boot-spin {
  to { transform: rotate(360deg); }
}
</style>
