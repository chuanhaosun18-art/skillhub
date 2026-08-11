// 认证 API 层 + 登录态管理（JWT token 存 localStorage）
import { reactive } from 'vue'

const BASE = 'http://localhost:8080'
const TOKEN_KEY = 'skillhub_token'
const USER_KEY = 'skillhub_user'

// 全局响应式登录态，组件可直接使用
export const authState = reactive({
  token: localStorage.getItem(TOKEN_KEY) || '',
  user: JSON.parse(localStorage.getItem(USER_KEY) || 'null'),
})

export function isLoggedIn() {
  return !!authState.token
}

export function getToken() {
  return authState.token
}

export function setAuth(token, user) {
  authState.token = token
  authState.user = user
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearAuth() {
  authState.token = ''
  authState.user = null
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

// 通用带 token 请求；401 时自动清空登录态
async function authRequest(path, options = {}) {
  const headers = { ...(options.headers || {}) }
  if (authState.token) headers['Authorization'] = `Bearer ${authState.token}`

  const resp = await fetch(`${BASE}${path}`, { ...options, headers })

  if (resp.status === 401 && !path.endsWith('/auth/login')) {
    clearAuth()
    throw new Error('登录已过期，请重新登录')
  }
  if (!resp.ok) {
    let msg = `请求失败：${resp.status}`
    try {
      const body = await resp.json()
      if (body.error) msg = body.error
    } catch (e) {
      /* ignore */
    }
    throw new Error(msg)
  }
  return resp.json()
}

/** 登录（用户名或邮箱） */
export async function login(account, password) {
  const body = await authRequest('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ account, password }),
  })
  setAuth(body.token, body.user)
  return body.user
}

/** 注册（含学校/年级/专业等画像字段） */
export async function register(form) {
  const body = await authRequest('/api/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(form),
  })
  setAuth(body.token, body.user)
  return body.user
}

/** 获取当前登录用户最新信息 */
export async function fetchMe() {
  const body = await authRequest('/api/auth/me')
  authState.user = body.data
  localStorage.setItem(USER_KEY, JSON.stringify(body.data))
  return body.data
}

/** 更新个人资料 */
export async function updateProfile(userId, form) {
  const body = await authRequest(`/api/users/${userId}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(form),
  })
  authState.user = body.data
  localStorage.setItem(USER_KEY, JSON.stringify(body.data))
  return body.data
}

/** 当前用户发布的技能列表 */
export async function fetchMySkills() {
  const body = await authRequest('/api/users/me/skills')
  return body.data || []
}
