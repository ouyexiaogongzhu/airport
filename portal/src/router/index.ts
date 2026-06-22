import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'login', component: () => import('../views/Login.vue') },
    { path: '/register', name: 'register', component: () => import('../views/Register.vue') },
    { path: '/dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { requiresAuth: true } },
    { path: '/products', name: 'products', component: () => import('../views/Products.vue'), meta: { requiresAuth: true } },
    { path: '/plans', name: 'plans', component: () => import('../views/Products.vue'), meta: { requiresAuth: true } },
    { path: '/checkout/:plan_id', name: 'checkout', component: () => import('../views/Checkout.vue'), meta: { requiresAuth: true } },
    { path: '/pay/:order_id', name: 'pay', component: () => import('../views/Pay.vue'), meta: { requiresAuth: true } },
    { path: '/pay/result', name: 'pay-result', component: () => import('../views/PayResult.vue'), meta: { requiresAuth: true } },
    { path: '/account', name: 'account', component: () => import('../views/Account.vue'), meta: { requiresAuth: true } },
    { path: '/account/guide', name: 'setup-guide', component: () => import('../views/SetupGuide.vue'), meta: { requiresAuth: true } },
    { path: '/account/devices', name: 'account-devices', component: () => import('../views/AccountDevices.vue'), meta: { requiresAuth: true } },
  ],
})

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('portal_token')
  if (to.meta.requiresAuth && !token) {
    next('/')
  } else {
    next()
  }
})

export default router
