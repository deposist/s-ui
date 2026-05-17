// Composables
import { createRouter, createWebHistory } from 'vue-router'
import Login from '@/views/Login.vue'
import Data from '@/store/modules/data'
import Ws from '@/store/ws'
import { getBaseUrl } from '@/plugins/base-url'

const routes = [
  {
    path: '/login',
    name: 'pages.login',
    component: Login,
  },
  {
    path: '/',
    component: () => import('@/layouts/default/Default.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '/',
        name: 'pages.home',
        component: () => import('@/views/Home.vue'),
      },
      {
        path: '/inbounds',
        name: 'pages.inbounds',
        component: () => import('@/views/Inbounds.vue'),
      },
      {
        path: '/clients',
        name: 'pages.clients',
        component: () => import('@/views/Clients.vue'),
      },
      {
        path: '/outbounds',
        name: 'pages.outbounds',
        component: () => import('@/views/Outbounds.vue'),
      },
      {
        path: '/services',
        name: 'pages.services',
        component: () => import('@/views/Services.vue'),
      },
      {
        path: '/endpoints',
        name: 'pages.endpoints',
        component: () => import('@/views/Endpoints.vue'),
      },
      {
        path: '/rules',
        name: 'pages.rules',
        component: () => import('@/views/Rules.vue'),
      },
      {
        path: '/tls',
        name: 'pages.tls',
        component: () => import('@/views/Tls.vue'),
      },
      {
        path: '/basics',
        name: 'pages.basics',
        component: () => import('@/views/Basics.vue'),
      },
      {
        path: '/dns',
        name: 'pages.dns',
        component: () => import('@/views/Dns.vue'),
      },
      {
        path: '/admins',
        name: 'pages.admins',
        component: () => import('@/views/Admins.vue'),
      },
      {
        path: '/telegram',
        name: 'pages.telegram',
        component: () => import('@/views/TelegramSettings.vue'),
      },
      {
        path: '/audit',
        name: 'pages.audit',
        component: () => import('@/views/Audit.vue'),
      },
      {
        path: '/settings',
        name: 'pages.settings',
        component: () => import('@/views/Settings.vue'),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(getBaseUrl()),
  routes,
})

let intervalId: any

// The session cookie is HttpOnly (set by api/session.go) so it cannot be
// observed from the client; auth is enforced server-side. Every API call
// returns `Invalid login` when the cookie is missing or expired, and
// httputil._handleMsg redirects the user to /login in that case. The router
// guard below only handles the UX detail of pulling fresh data on first
// navigation to a protected page and stopping the polling timer when we
// land on /login.
router.beforeEach((to) => {
  if (to.path !== '/login') {
    loadDataInterval()
    Ws().connect()
  } else if (intervalId) {
    clearInterval(intervalId)
    intervalId = undefined
    Ws().disconnect()
  }
})

const loadDataInterval = () => {
  if (intervalId) return
  Data().loadData()
  intervalId = setInterval(() => {
    Data().loadData()
  }, 10000)
}

export default router
