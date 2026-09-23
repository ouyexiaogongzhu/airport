<template>
  <div class="landing">
    <header class="topbar">
      <span class="brand">RFPlay</span>
      <nav class="top-actions">
        <router-link v-if="auth.isLoggedIn" to="/dashboard" class="login-link">Dashboard</router-link>
        <router-link v-else to="/login" class="login-link">Login</router-link>
      </nav>
    </header>

    <section class="hero">
      <p class="brand-hero">RFPlay</p>
      <h1>Private network access, made simple</h1>
      <p class="lead">
        Subscribe once, import on any device, and stay connected with a single link.
      </p>
      <div class="cta-row">
        <router-link v-if="auth.isLoggedIn" to="/dashboard" class="btn-primary">Open Dashboard</router-link>
        <template v-else>
          <router-link to="/register" class="btn-primary">Get Started</router-link>
          <router-link to="/login" class="btn-secondary">Sign In</router-link>
        </template>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
</script>

<style scoped>
.landing {
  min-height: 100vh;
  background:
    radial-gradient(ellipse 80% 50% at 50% -10%, rgba(26, 115, 232, 0.18), transparent 55%),
    linear-gradient(180deg, #16213e 0%, #1a1a2e 45%, #12121f 100%);
  color: #e2e8f0;
  display: flex;
  flex-direction: column;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 2rem;
}

.brand {
  font-weight: 700;
  color: #1a73e8;
  font-size: 1.15rem;
  letter-spacing: 0.02em;
}

.login-link {
  color: #a0aec0;
  text-decoration: none;
  font-size: 0.95rem;
  font-weight: 500;
  padding: 0.4rem 0.9rem;
  border: 1px solid #0f3460;
  border-radius: 6px;
  transition: color 0.15s, border-color 0.15s;
}

.login-link:hover {
  color: #1a73e8;
  border-color: #1a73e8;
}

.hero {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: flex-start;
  max-width: 720px;
  margin: 0 auto;
  padding: 2rem 2rem 4rem;
  width: 100%;
  box-sizing: border-box;
}

.brand-hero {
  margin: 0 0 0.75rem;
  font-size: clamp(2.75rem, 8vw, 4.5rem);
  font-weight: 800;
  letter-spacing: -0.03em;
  color: #1a73e8;
  line-height: 1;
}

.hero h1 {
  margin: 0 0 1rem;
  font-size: clamp(1.35rem, 3.5vw, 1.85rem);
  font-weight: 600;
  color: #e2e8f0;
  line-height: 1.35;
  max-width: 18ch;
}

.lead {
  margin: 0 0 2rem;
  font-size: 1.05rem;
  color: #a0aec0;
  line-height: 1.55;
  max-width: 36ch;
}

.cta-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.btn-primary {
  background: #1a73e8;
  color: white;
  text-decoration: none;
  padding: 0.7rem 1.35rem;
  border-radius: 8px;
  font-weight: 600;
  font-size: 0.95rem;
}

.btn-primary:hover {
  background: #1557b0;
}

.btn-secondary {
  background: transparent;
  color: #a0aec0;
  text-decoration: none;
  padding: 0.7rem 1.35rem;
  border-radius: 8px;
  border: 1px solid #0f3460;
  font-weight: 500;
  font-size: 0.95rem;
}

.btn-secondary:hover {
  color: #1a73e8;
  border-color: #1a73e8;
}

@media (max-width: 640px) {
  .topbar {
    padding: 0.85rem 1.25rem;
  }
  .hero {
    padding: 1.5rem 1.25rem 3rem;
  }
}
</style>
