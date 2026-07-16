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
          <label>Email</label>
          <input v-model="email" type="email" placeholder="Enter your email" required />
        </div>
        <div class="field">
          <label>Password</label>
          <input v-model="password" type="password" placeholder="Create a password" required />
        </div>
        <div class="field">
          <label>Confirm Password</label>
          <input v-model="confirmPassword" type="password" placeholder="Confirm your password" required />
        </div>
        <div v-if="captchaQuestion" class="field captcha-field">
          <label>Security Check</label>
          <div class="captcha-row">
            <span class="captcha-question">{{ captchaQuestion }}</span>
            <input v-model="captchaAnswer" type="text" placeholder="Answer" class="captcha-input" required />
            <button type="button" class="btn-refresh" @click="fetchCaptcha" :disabled="captchaLoading">⟳</button>
          </div>
        </div>
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
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../api/index'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const email = ref('')
const password = ref('')
const confirmPassword = ref('')
const error = ref('')
const loading = ref(false)

// Captcha
const captchaQuestion = ref('')
const captchaToken = ref('')
const captchaAnswer = ref('')
const captchaLoading = ref(false)

async function fetchCaptcha() {
  captchaLoading.value = true
  try {
    const res = await api.get('/captcha')
    captchaQuestion.value = res.data.question
    captchaToken.value = res.data.token
    captchaAnswer.value = ''
  } catch {
    // Captcha unavailable, allow registration without it
    captchaQuestion.value = ''
    captchaToken.value = ''
  } finally {
    captchaLoading.value = false
  }
}

async function handleRegister() {
  error.value = ''
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match'
    return
  }
  if (password.value.length < 6) {
    error.value = 'Password must be at least 6 characters'
    return
  }
  if (captchaToken.value && !captchaAnswer.value) {
    error.value = 'Please answer the security question'
    return
  }
  loading.value = true
  const res = await auth.register(username.value, email.value, password.value, captchaToken.value, captchaAnswer.value)
  loading.value = false
  if (res.success) {
    router.push('/dashboard')
  } else {
    error.value = res.error || 'Registration failed'
    if (captchaToken.value) fetchCaptcha() // Refresh captcha on failure
  }
}

onMounted(fetchCaptcha)
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
.captcha-field { margin-bottom: 0.5rem; }
.captcha-row { display: flex; align-items: center; gap: 0.5rem; }
.captcha-question { background: #f0f4ff; border: 1px solid #d0d8e8; border-radius: 6px; padding: 0.5rem 0.75rem; font-size: 0.95rem; font-weight: 600; color: #333; white-space: nowrap; }
.captcha-input { flex: 1; padding: 0.55rem 0.75rem; border: 1px solid #d0d8e8; border-radius: 6px; font-size: 0.9rem; outline: none; }
.captcha-input:focus { border-color: #1a73e8; }
.btn-refresh { background: transparent; border: 1px solid #d0d8e8; border-radius: 6px; padding: 0.45rem 0.65rem; font-size: 1.1rem; cursor: pointer; }
.btn-refresh:hover { border-color: #1a73e8; color: #1a73e8; }
</style>
