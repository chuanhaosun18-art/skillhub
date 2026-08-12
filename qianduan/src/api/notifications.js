// 消息通知 API 层（需登录）：铃铛未读数 + 通知列表 + 标记已读
import { authState } from './auth'

const BASE = ''

async function authRequest(path, options = {}) {
  const headers = { ...(options.headers || {}) }
  if (authState.token) headers['Authorization'] = `Bearer ${authState.token}`
  const resp = await fetch(`${BASE}${path}`, { ...options, headers })
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

/** 未读通知数量 */
export async function getUnreadCount() {
  const body = await authRequest('/api/notifications/unread-count')
  return body.count || 0
}

/** 我的通知列表（倒序） */
export async function fetchNotifications() {
  const body = await authRequest('/api/notifications')
  return body.data || []
}

/** 全部标记已读 */
export async function markAllRead() {
  return authRequest('/api/notifications/read', { method: 'POST' })
}

/** 单条标记已读 */
export async function markRead(id) {
  return authRequest(`/api/notifications/${id}/read`, { method: 'POST' })
}
