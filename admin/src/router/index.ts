import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'login', component: () => import('../views/Login.vue') },
    {
      path: '/admin',
      component: () => import('../layouts/AdminLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: '/admin/dashboard' },
        { path: 'dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue') },
        { path: 'users', name: 'users', component: () => import('../views/Users.vue') },
        { path: 'products', name: 'products', component: () => import('../views/Products.vue') },
        { path: 'orders', name: 'orders', component: () => import('../views/Orders.vue') },
        { path: 'nodes', name: 'nodes', component: () => import('../views/Nodes.vue') },
        { path: 'tokens', name: 'tokens', component: () => import('../views/Tokens.vue') },
        { path: 'settings', name: 'settings', component: () => import('../views/Settings.vue') },
        { path: 'plans', name: 'plans', component: () => import('../views/Plans.vue') },
      ],
    },
  ],
})

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('admin_token')
  const requiresAuth = to.matched.some(record => record.meta.requiresAuth)
  if (requiresAuth && !token) {
    next('/')
  } else {
    next()
  }
})

export default router
