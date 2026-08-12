// 论坛 / 许愿池 API：looking_for = 挂愿望，like = 我也在等，reply = 走过的人。
import { getToken } from './auth'

const BASE = ''

async function req(path, { method = 'GET', body } = {}) {
  const headers = {}
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  const resp = await fetch(`${BASE}/api/forum${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  let data = null
  try {
    data = await resp.json()
  } catch (e) {
    data = null
  }
  if (!resp.ok) {
    throw new Error((data && data.error) || `请求失败：${resp.status}`)
  }
  return data
}

/** 论坛分类（与后端保持一致） */
export const FORUM_CATEGORIES = [
  { value: 'looking_for', label: '还没有这张卡' },
  { value: 'help', label: '求学长带一下' },
  { value: 'experience', label: '我走过可以沉淀' },
]

export function categoryLabel(value) {
  const hit = FORUM_CATEGORIES.find((c) => c.value === value)
  return hit ? hit.label : value
}

/**
 * 帖子列表（游客可用）
 * @param {{keyword?: string, category?: string}} [opts]
 * @returns {Promise<Array>}
 */
export async function listTopics(opts = {}) {
  const params = new URLSearchParams()
  if (opts.keyword) params.set('keyword', opts.keyword)
  if (opts.category && opts.category !== '全部') params.set('category', opts.category)
  const qs = params.toString()
  const body = await req(`/topics${qs ? `?${qs}` : ''}`)
  return (body.data || []).map((t) => ({ ...t, category_label: categoryLabel(t.category) }))
}

/** 发帖（需登录） */
export async function createTopic({ title, content = '', category = 'help' }) {
  const body = await req('/topics', { method: 'POST', body: { title, content, category } })
  return body.data
}

/** 帖子详情 + 回复列表（游客可用） */
export async function getTopic(id) {
  const body = await req(`/topics/${id}`)
  return { ...body.data, category_label: categoryLabel(body.data.category), replies: body.replies || [] }
}

/** 回复（需登录） */
export async function createReply(topicId, content) {
  const body = await req(`/topics/${topicId}/replies`, { method: 'POST', body: { content } })
  return body.data
}

/** 点赞 / 取消点赞帖子（需登录，toggle） */
export async function likeTopic(id) {
  const body = await req(`/topics/${id}/like`, { method: 'POST' })
  return body.data // { liked, like_count }
}

/** 点赞 / 取消点赞回复（需登录，toggle） */
export async function likeReply(id) {
  const body = await req(`/replies/${id}/like`, { method: 'POST' })
  return body.data // { liked, like_count }
}
