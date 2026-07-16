import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../api/index'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('portal_token') || null)
  const user = ref(JSON.parse(localStorage.getItem('portal_user') || 'null'))

  const isLoggedIn = computed(() => !!token.value)
  const username = computed(() => user.value?.username || '')

  async function login(username: string, password: string) {
    try {
      const res = await api.post('/public/login', { username, password })
      const { token: t, user: u } = res.data
      token.value = t
      user.value = u
      localStorage.setItem('portal_token', t)
      localStorage.setItem('portal_user', JSON.stringify(u))
      return { success: true }
    } catch (e: any) {
      const apiError = e?.response?.data?.error || e?.message || 'Login failed'
      return { success: false, error: apiError }
    }
  }

  async function register(username: string, email: string, password: string) {
    try {
      const res = await api.post('/public/register', { username, email, password })
      const { token: t, user: u } = res.data
      token.value = t
      user.value = u
      localStorage.setItem('portal_token', t)
      localStorage.setItem('portal_user', JSON.stringify(u))
      return { success: true }
    } catch (e: any) {
      const apiError = e?.response?.data?.error || e?.message || 'Registration failed'
      return { success: false, error: apiError }
    }
  }

  function logout() {
    token.value = null
    user.value = null
    localStorage.removeItem('portal_token')
    localStorage.removeItem('portal_user')
  }

  return { token, user, isLoggedIn, username, login, register, logout }
})
