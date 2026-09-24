import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: () => import('../views/Home.vue') },
    { path: '/login', name: 'login', component: () => import('../views/Login.vue') },
    { path: '/register', name: 'register', component: () => import('../views/Register.vue') },
    { path: '/subscription', redirect: '/account' },
    { path: '/dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { requiresAuth: true } },
    { path: '/products', redirect: { path: '/dashboard', hash: '#plans' } },
    { path: '/plans', redirect: { path: '/dashboard', hash: '#plans' } },
    { path: '/checkout/:plan_id', name: 'checkout', component: () => import('../views/Checkout.vue'), meta: { requiresAuth: true } },
    { path: '/pay/:order_id', name: 'pay', component: () => import('../views/Pay.vue'), meta: { requiresAuth: true } },
    { path: '/pay/result', name: 'pay-result', component: () => import('../views/PayResult.vue'), meta: { requiresAuth: true } },
    { path: '/account', name: 'account', component: () => import('../views/Account.vue'), meta: { requiresAuth: true } },
    { path: '/account/guide', redirect: '/account' },
    { path: '/setup', redirect: '/account' },
    { path: '/account/devices', redirect: { path: '/account', hash: '#devices' } },
  ],
  scrollBehavior(to) {
    if (to.hash) {
      return { el: to.hash, behavior: 'smooth' }
    }
    return { top: 0 }
  },
})

router.beforeEach(async (to, _from, next) => {
  const auth = useAuthStore()
  if (!auth.bootstrapped) {
    await auth.authReady
  }
  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next('/login')
  } else {
    next()
  }
})

export default router
