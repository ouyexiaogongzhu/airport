import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: () => import('../views/Home.vue') },
    { path: '/login', name: 'login', component: () => import('../views/Login.vue') },
    { path: '/register', name: 'register', component: () => import('../views/Register.vue') },
    { path: '/plans', name: 'plans', component: () => import('../views/Plans.vue') },
    { path: '/checkout/:plan_id', name: 'checkout', component: () => import('../views/Checkout.vue') },
    { path: '/pay/:order_id', name: 'pay', component: () => import('../views/Pay.vue') },
    { path: '/pay/result', name: 'pay-result', component: () => import('../views/PayResult.vue') },
    { path: '/account', name: 'account', component: () => import('../views/Account.vue') },
    { path: '/account/devices', name: 'devices', component: () => import('../views/AccountDevices.vue') },
  ]
})

export default router
