import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '../api'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/register',
      name: 'register',
      component: () => import('../views/RegisterView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/chat',
      name: 'chat',
      component: () => import('../views/ChatView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/',
      redirect: '/chat'
    }
  ]
})

router.beforeEach((to, from, next) => {
  const isLoggedIn = !!getToken()
  const requiresAuth = to.meta.requiresAuth

  if (requiresAuth && !isLoggedIn) {
    next('/login')
  } else if (!requiresAuth && isLoggedIn && (to.path === '/login' || to.path === '/register')) {
    next('/chat')
  } else {
    next()
  }
})

export default router
