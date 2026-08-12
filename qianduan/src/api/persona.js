// 虚拟自己（Persona）API 层：引导对话保留/蒸馏/开关/访客聊天
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

/** 保存引导对话（兜底；guideChat 已自动保存）。返回 conversation_id */
export async function saveConversation(messages, conversationId = 0, title = '') {
  const body = await authRequest('/api/persona/conversations', {
    method: 'POST',
    body: JSON.stringify({ conversation_id: conversationId, title, messages }),
  })
  return body.conversation_id
}

/** 蒸馏对话为"虚拟自己"（需登录，仅属主） */
export async function distillConversation(conversationId) {
  const body = await authRequest(`/api/persona/conversations/${conversationId}/distill`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
  return body.persona_text
}

/** 我的虚拟自己（需登录） */
export async function getMyPersona() {
  const body = await authRequest('/api/persona/me')
  return body.persona || {}
}

/** 开关虚拟自己（需登录） */
export async function updateMyPersona(chatEnabled) {
  const body = await authRequest('/api/persona/me', {
    method: 'PATCH',
    body: JSON.stringify({ chat_enabled: chatEnabled }),
  })
  return body
}

/** 查看某用户的虚拟自己（游客可看开关状态与摘要） */
export async function getPublicPersona(userId) {
  const token = getToken()
  const resp = await fetch(`${BASE}/api/persona/public/${userId}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!resp.ok) {
    throw new Error(`请求失败：${resp.status}`)
  }
  const body = await resp.json()
  return body.persona || {}
}

/** 访客发起与虚拟自己的聊天（需登录） */
export async function createPersonaChat(userId, allowOwnerView = false) {
  const body = await authRequest(`/api/persona/public/${userId}/chats`, {
    method: 'POST',
    body: JSON.stringify({ allow_owner_view: allowOwnerView }),
  })
  return body
}

/** 发送消息给虚拟自己（需登录） */
export async function sendPersonaMessage(chatId, content) {
  const body = await authRequest(`/api/persona/chat/${chatId}/messages`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  })
  return body.reply
}

/** 拉取与虚拟自己的聊天记录（需登录，访客或对方允许时的主人） */
export async function getPersonaMessages(chatId) {
  const body = await authRequest(`/api/persona/chat/${chatId}/messages`)
  return { messages: body.messages || [], allow_owner_view: body.allow_owner_view }
}

/** 主人查看"别人与虚拟我的聊天"（需登录） */
export async function listMyPersonaChats() {
  const body = await authRequest('/api/persona/me/chats')
  return body.data || []
}
