// 在线聊天（Direct Chat）API 层：轮询实现的一对一聊天
import { getToken } from './auth'

const BASE = ''

async function authRequest(path, options = {}) {
  const token = getToken()
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const resp = await fetch(`${BASE}${path}`, { ...options, headers })
  let body = null
  try {
    body = await resp.json()
  } catch (e) {
    /* ignore */
  }
  if (!resp.ok) {
    throw new Error((body && body.error) || `请求失败：${resp.status}`)
  }
  return body
}

/** 获取/创建与某用户的一对一聊天（需登录） */
export async function createDirectChat(userId) {
  const body = await authRequest('/api/chat/direct', {
    method: 'POST',
    body: JSON.stringify({ user_id: userId }),
  })
  return body
}

/** 我的会话列表（需登录） */
export async function listDirectChats() {
  const body = await authRequest('/api/chat/direct')
  return body.data || []
}

/** 发送消息（需登录） */
export async function sendDirectMessage(chatId, content) {
  const body = await authRequest(`/api/chat/direct/${chatId}/messages`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  })
  return body
}

/** 轮询拉取 after 之后的新消息（需登录） */
export async function getDirectMessages(chatId, afterId = 0) {
  const body = await authRequest(`/api/chat/direct/${chatId}/messages?after=${afterId}`)
  return body.messages || []
}
