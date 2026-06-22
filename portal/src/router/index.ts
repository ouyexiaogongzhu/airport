import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'login', component: () => import('../views/Login.vue') },
    { path: '/register', name: 'register', component: () => import('../views/Register.vue') },
    { path: '/dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue'), meta: { requiresAuth: true } },
    { path: '/products', name: 'products', component: () => import('../views/Products.vue'), meta: { requiresAuth: true } },
    { path: '/account', name: 'account', component: () => import('../views/Account.vue'), meta: { requiresAuth: true } },
    { path: '/account/guide', name: 'setup-guide', component: () => import('../views/SetupGuide.vue'), meta: { requiresAuth: true } },
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
