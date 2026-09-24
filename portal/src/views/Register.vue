<template>
  <div class="page register-page">
    <div class="register-card">
      <h1>RFPlay</h1>
      <p class="subtitle">Create your account</p>

      <a v-if="googleEnabled" class="btn google" :href="googleStartUrl">
        <svg class="google-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
          <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
          <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
          <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
        </svg>
        Continue with Google
      </a>
      <div v-if="googleEnabled" class="divider"><span>or</span></div>

      <form @submit.prevent="handleRegister">
        <div class="field">
          <label>Email</label>
          <input
            v-model="email"
            type="email"
            autocomplete="email"
            placeholder="you@example.com"
            required
          />
        </div>
        <div class="field">
          <label>Username</label>
          <input v-model="username" type="text" autocomplete="username" placeholder="Choose a username" required />
        </div>
        <div class="field">
          <label>Password</label>
          <input v-model="password" type="password" autocomplete="new-password" placeholder="Create a password" required />
        </div>
        <div class="field">
          <label>Confirm Password</label>
          <input v-model="confirmPassword" type="password" autocomplete="new-password" placeholder="Confirm your password" required />
        </div>
        <Turnstile ref="turnstileRef" v-model="turnstileToken" />
        <p v-if="error" class="error">{{ error }}</p>
        <button type="submit" class="btn" :disabled="loading || turnstileBlocked">
          {{ loading ? 'Creating account…' : 'Create Account' }}
        </button>
      </form>

      <p class="switch">
        Already have an account?
        <router-link to="/login">Sign In</router-link>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import Turnstile from '../components/Turnstile.vue'
import { googleOAuthStartUrl, isGoogleSignInEnabled } from '../utils/googleAuth'

const router = useRouter()
const auth = useAuthStore()

const email = ref('')
const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const turnstileToken = ref('')
const turnstileRef = ref<InstanceType<typeof Turnstile> | null>(null)
const error = ref('')
const loading = ref(false)

const turnstileSiteKey = (import.meta.env.VITE_TURNSTILE_SITE_KEY as string | undefined)?.trim() || ''
const turnstileBlocked = computed(() => !!turnstileSiteKey && !turnstileToken.value)
const googleEnabled = isGoogleSignInEnabled()
const googleStartUrl = googleOAuthStartUrl()

async function handleRegister() {
  error.value = ''
  if (!email.value.trim()) {
    error.value = 'Email is required'
    return
  }
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match'
    return
  }
  if (password.value.length < 8) {
    error.value = 'Password must be at least 8 characters'
    return
  }
  if (turnstileBlocked.value) {
    error.value = '请等待自动校验完成'
    return
  }
  loading.value = true
  const res = await auth.register(username.value, password.value, turnstileToken.value, email.value.trim())
  loading.value = false
  if (res.success) {
    router.push('/dashboard')
  } else {
    error.value = res.error || 'Registration failed'
    turnstileRef.value?.reset()
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
  box-sizing: border-box;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  text-decoration: none;
}
.btn:hover { background: #1557b0; }
.btn:disabled { opacity: 0.6; cursor: not-allowed; }
.btn.google {
  margin-top: 0;
  background: #fff;
  color: #3c4043;
  border: 1px solid #dadce0;
  font-weight: 500;
}
.btn.google:hover { background: #f8f9fa; }
.google-icon { width: 1.1rem; height: 1.1rem; flex-shrink: 0; }
.divider {
  display: flex;
  align-items: center;
  margin: 1.25rem 0;
  color: #999;
  font-size: 0.8rem;
}
.divider::before,
.divider::after {
  content: '';
  flex: 1;
  border-top: 1px solid #e0e0e0;
}
.divider span { padding: 0 0.75rem; }
.error { color: #d93025; font-size: 0.85rem; margin: 0.5rem 0; }
.switch { text-align: center; margin-top: 1.25rem; font-size: 0.85rem; color: #666; }
.switch a { color: #1a73e8; text-decoration: none; font-weight: 500; }
</style>
