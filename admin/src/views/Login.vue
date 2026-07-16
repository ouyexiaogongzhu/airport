<template>
  <div class="page login-page">
    <div class="login-card">
      <h1>RFPlay Admin</h1>
      <p class="subtitle">Management Console</p>

      <form @submit.prevent="handleLogin">
        <div class="field">
          <label>Username</label>
          <input v-model="username" type="text" placeholder="admin" required />
        </div>
        <div class="field">
          <label>Password</label>
          <input v-model="password" type="password" placeholder="Enter password" required />
        </div>
        <p v-if="error" class="error">{{ error }}</p>
        <button type="submit" class="btn" :disabled="loading">
          {{ loading ? 'Signing in…' : 'Sign In' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  error.value = ''
  loading.value = true
  const res = await auth.login(username.value, password.value)
  loading.value = false
  if (res.success) {
    router.push('/admin/dashboard')
  } else {
    error.value = res.error || 'Login failed'
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #1a1d23;
}
.login-card {
  background: #2a2d35;
  border-radius: 12px;
  padding: 2.5rem;
  width: 100%;
  max-width: 400px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.3);
}
h1 {
  text-align: center;
  color: #fff;
  margin: 0 0 0.25rem;
  font-size: 1.75rem;
}
.subtitle {
  text-align: center;
  color: #888;
  margin: 0 0 1.5rem;
  font-size: 0.9rem;
}
.field { margin-bottom: 1rem; }
.field label {
  display: block;
  margin-bottom: 0.35rem;
  color: #ccc;
  font-weight: 500;
  font-size: 0.85rem;
}
.field input {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid #444;
  border-radius: 8px;
  font-size: 0.95rem;
  background: #1a1d23;
  color: #eee;
  box-sizing: border-box;
}
.field input:focus {
  outline: none;
  border-color: #4a9eff;
  box-shadow: 0 0 0 2px rgba(74,158,255,0.2);
}
.btn {
  width: 100%;
  padding: 0.65rem;
  background: #4a9eff;
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 1rem;
  cursor: pointer;
  margin-top: 0.5rem;
}
.btn:hover { background: #3a8ef0; }
.btn:disabled { opacity: 0.6; cursor: not-allowed; }
.error { color: #ff6b6b; font-size: 0.85rem; margin: 0.5rem 0; }
</style>
