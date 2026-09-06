<template>
  <div class="page register-page">
    <div class="register-card">
      <h1>RFPlay</h1>
      <p class="subtitle">Create your account</p>

      <form @submit.prevent="handleRegister">
        <div class="field">
          <label>Username</label>
          <input v-model="username" type="text" placeholder="Choose a username" required />
        </div>
        <div class="field">
          <label>Password</label>
          <input v-model="password" type="password" placeholder="Create a password" required />
        </div>
        <div class="field">
          <label>Confirm Password</label>
          <input v-model="confirmPassword" type="password" placeholder="Confirm your password" required />
        </div>
        <Turnstile v-model="turnstileToken" />
        <p v-if="error" class="error">{{ error }}</p>
        <button type="submit" class="btn" :disabled="loading">
          {{ loading ? 'Creating account…' : 'Create Account' }}
        </button>
      </form>

      <p class="switch">
        Already have an account?
        <router-link to="/">Sign In</router-link>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import Turnstile from '../components/Turnstile.vue'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const turnstileToken = ref('')
const error = ref('')
const loading = ref(false)

async function handleRegister() {
  error.value = ''
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match'
    return
  }
  if (password.value.length < 8) {
    error.value = 'Password must be at least 8 characters'
    return
  }
  loading.value = true
  const res = await auth.register(username.value, password.value, turnstileToken.value)
  loading.value = false
  if (res.success) {
    router.push('/dashboard')
  } else {
    error.value = res.error || 'Registration failed'
  }
}
</script>

<style scoped>
.register-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #e8f0fe 0%, #ffffff 100%);
}
.register-card {
  background: white;
  border-radius: 12px;
  padding: 2.5rem;
  width: 100%;
  max-width: 420px;
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
  margin-bottom: 0.85rem;
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
