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
  // ---------- 成长闭环（PRD P0）----------
  {
    path: '/grow',
    name: 'grow',
    component: () => import('../views/Grow.vue'),
    meta: { title: '我要成长', requiresAuth: true },
  },
  {
    path: '/workbench',
    name: 'workbench',
    component: () => import('../views/Workbench.vue'),
    meta: { title: '任务工作台', requiresAuth: true },
  },
  {
    path: '/creator',
    name: 'creator',
    component: () => import('../views/Creator.vue'),
    meta: { title: '固化为 Skill', requiresAuth: true },
  },
  {
    path: '/gate',
    name: 'gate',
    component: () => import('../views/Gate.vue'),
    meta: { title: '发布前四问', requiresAuth: true },
  },
  {
    path: '/trust/:id',
    name: 'trust-card',
    component: () => import('../views/TrustCard.vue'),
    meta: { title: 'Trust Card' },
  },
  {
    path: '/growth/:id',
    name: 'growth-profile',
    component: () => import('../views/GrowthProfile.vue'),
    meta: { title: '成长身份' },
  },
  {
    path: '/orchestration',
    name: 'orchestration',
    component: () => import('../views/Orchestration.vue'),
    meta: { title: '接下来几周该做什么', requiresAuth: true },
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
