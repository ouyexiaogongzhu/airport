import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/Login.vue') },
    { path: '/', name: 'dashboard', component: () => import('../views/Dashboard.vue') },
    { path: '/nodes', name: 'nodes', component: () => import('../views/Nodes.vue') },
    { path: '/users', name: 'users', component: () => import('../views/Users.vue') },
    { path: '/tokens', name: 'tokens', component: () => import('../views/Tokens.vue') },
    { path: '/plans', name: 'plans', component: () => import('../views/Plans.vue') },
    { path: '/orders', name: 'orders', component: () => import('../views/Orders.vue') },
    { path: '/settings', name: 'settings', component: () => import('../views/Settings.vue') },
  ]
})

export default router
