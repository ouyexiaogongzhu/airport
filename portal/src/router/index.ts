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
    { path: '/products', name: 'products', component: () => import('../views/Products.vue'), meta: { requiresAuth: true } },
    { path: '/plans', name: 'plans', component: () => import('../views/Products.vue'), meta: { requiresAuth: true } },
    { path: '/checkout/:plan_id', name: 'checkout', component: () => import('../views/Checkout.vue'), meta: { requiresAuth: true } },
    { path: '/pay/:order_id', name: 'pay', component: () => import('../views/Pay.vue'), meta: { requiresAuth: true } },
    { path: '/pay/result', name: 'pay-result', component: () => import('../views/PayResult.vue'), meta: { requiresAuth: true } },
    { path: '/account', name: 'account', component: () => import('../views/Account.vue'), meta: { requiresAuth: true } },
    { path: '/account/guide', redirect: { path: '/account', hash: '#setup' } },
    { path: '/setup', redirect: { path: '/account', hash: '#setup' } },
    { path: '/account/devices', name: 'account-devices', component: () => import('../views/AccountDevices.vue'), meta: { requiresAuth: true } },
  ],
  scrollBehavior(to) {
    if (to.hash) {
      return { el: to.hash, behavior: 'smooth' }
    }
    return { top: 0 }
  },
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next('/login')
  } else {
    next()
  }
})

export default router
