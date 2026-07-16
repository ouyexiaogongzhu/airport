import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../api/index'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('admin_token') || null)
  const user = ref(JSON.parse(localStorage.getItem('admin_user') || 'null'))

  const isLoggedIn = computed(() => !!token.value)
  const username = computed(() => user.value?.username || '')

  async function login(username: string, password: string) {
    try {
      const res = await api.post('/public/login', { username, password })
      const { token: t, user: u } = res.data
      token.value = t
      user.value = u
      localStorage.setItem('admin_token', t)
      localStorage.setItem('admin_user', JSON.stringify(u))
      return { success: true }
    } catch (e: any) {
      const apiError = e?.response?.data?.error || e?.message || 'Login failed'
      return { success: false, error: apiError }
    }
  }

  function logout() {
    token.value = null
    user.value = null
    localStorage.removeItem('admin_token')
    localStorage.removeItem('admin_user')
  }

  return { token, user, isLoggedIn, username, login, logout }
})
