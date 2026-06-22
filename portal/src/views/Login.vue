<template>
  <div class="page login-page">
    <div class="login-card">
      <h1>RFPlay</h1>
      <p class="subtitle">Sign in to your account</p>

      <form @submit.prevent="handleLogin">
        <div class="field">
          <label>Username</label>
          <input v-model="username" type="text" placeholder="Enter username" required />
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

      <p class="switch">
        Don't have an account?
        <router-link to="/register">Register</router-link>
      </p>
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
    router.push('/dashboard')
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
  background: linear-gradient(135deg, #e8f0fe 0%, #ffffff 100%);
}
.login-card {
  background: white;
  border-radius: 12px;
  padding: 2.5rem;
  width: 100%;
  max-width: 400px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.08);
}
h1 {
  text-align: center;
  color: #1a73e8;
  margin: 0 0 0.25rem;
  font-size: 1.75rem;
}
.subtitle {
  text-align: center;
  color: #666;
  margin: 0 0 1.5rem;
  font-size: 0.9rem;
}
.field {
  margin-bottom: 1rem;
}
.field label {
  display: block;
  margin-bottom: 0.35rem;
  color: #333;
  font-weight: 500;
  font-size: 0.85rem;
}
.field input {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border: 1px solid #ddd;
  border-radius: 8px;
  font-size: 0.95rem;
  box-sizing: border-box;
}
.field input:focus {
  outline: none;
  border-color: #1a73e8;
  box-shadow: 0 0 0 2px rgba(26,115,232,0.15);
}
.btn {
  width: 100%;
  padding: 0.65rem;
  background: #1a73e8;
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 1rem;
  cursor: pointer;
  margin-top: 0.5rem;
}
.btn:hover { background: #1557b0; }
.btn:disabled { opacity: 0.6; cursor: not-allowed; }
.error { color: #d93025; font-size: 0.85rem; margin: 0.5rem 0; }
.switch { text-align: center; margin-top: 1.25rem; font-size: 0.85rem; color: #666; }
.switch a { color: #1a73e8; text-decoration: none; font-weight: 500; }
</style>
