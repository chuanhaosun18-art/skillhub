import { createRouter, createWebHistory } from 'vue-router'
import { isLoggedIn } from '../api/auth'

const routes = [
  {
    path: '/',
    name: 'home',
    component: () => import('../views/Home.vue'),
    meta: { title: 'SkillHub - 技能发现与分享平台' },
  },
  {
    path: '/search',
    name: 'search',
    component: () => import('../views/SearchResults.vue'),
    meta: { title: '搜索结果' },
  },
  {
    path: '/skill/:id',
    name: 'skill-detail',
    component: () => import('../views/SkillDetail.vue'),
    meta: { title: '技能详情' },
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/Login.vue'),
    meta: { title: '登录 / 注册' },
  },
  {
    path: '/publish',
    name: 'publish',
    component: () => import('../views/Publish.vue'),
    meta: { title: '发布技能', requiresAuth: true },
  },
  {
    path: '/profile',
    name: 'profile',
    component: () => import('../views/Profile.vue'),
    meta: { title: '个人中心', requiresAuth: true },
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 登录守卫：需要登录的页面跳转去登录页，并带回跳地址
router.beforeEach((to) => {
  if (to.meta.requiresAuth && !isLoggedIn()) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  return true
})

router.afterEach((to) => {
  document.title = to.meta.title || 'SkillHub'
})

export default router
