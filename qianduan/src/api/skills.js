// 后端 API 层：Go + Gin + SQLite。
// BASE 为空字符串 = 同源请求，由 vite dev server 代理 /api 与 /uploads 到本机后端，
// 这样队友通过局域网 IP 访问 5173 时无需知道后端地址。
import { getToken } from './auth'

const BASE = ''

// 后端返回的 skill 字段转换成前端友好的结构
function normalizeSkill(s) {
  let tags = []
  if (typeof s.tags === 'string') {
    try {
      tags = JSON.parse(s.tags)
    } catch (e) {
      tags = []
    }
  } else if (Array.isArray(s.tags)) {
    tags = s.tags
  }
  // 评估指标证明图片：后端返回相对路径数组，这里补全为可访问 URL
  let proofImages = []
  if (typeof s.proof_images === 'string') {
    try {
      proofImages = JSON.parse(s.proof_images)
    } catch (e) {
      proofImages = []
    }
  } else if (Array.isArray(s.proof_images)) {
    proofImages = s.proof_images
  }
  proofImages = proofImages
    .filter((p) => p && typeof p === 'string')
    .map((p) => (p.startsWith('http') ? p : `${BASE}${p}`))
  return {
    ...s,
    tags,
    proofImages,
    // 后端暂无 likes，用下载量作为“最受欢迎”的排序指标
    likes: s.download_count || 0,
    owner: s.owner_name || '匿名',
  }
}

async function request(path) {
  const resp = await fetch(`${BASE}${path}`)
  if (!resp.ok) {
    throw new Error(`请求失败：${resp.status}`)
  }
  return resp.json()
}

/**
 * 搜索技能（后端 LIKE 匹配名称/描述/分类/标签）
 * @param {string} keyword 搜索关键词
 * @param {{category?: string, sort?: string}} [opts] 附加筛选
 * @returns {Promise<Array>} 匹配的技能列表
 */
export async function searchSkills(keyword = '', opts = {}) {
  const params = new URLSearchParams()
  if (keyword) params.set('keyword', keyword)
  if (opts.category && opts.category !== '全部') params.set('category', opts.category)
  if (opts.sort) params.set('sort', opts.sort)
  const qs = params.toString()
  const body = await request(`/api/skills${qs ? `?${qs}` : ''}`)
  return (body.data || []).map(normalizeSkill)
}

/**
 * 获取技能详情
 * @param {number|string} id
 * @returns {Promise<Object|undefined>}
 */
export async function getSkillById(id) {
  const body = await request(`/api/skills/${id}`)
  return body.data ? normalizeSkill(body.data) : undefined
}

/**
 * 获取热门搜索词（静态维护，后续可按搜索热度从后端统计）
 * @returns {Promise<Array>}
 */
export async function getHotKeywords() {
  return ['Vue', 'Python', '机器学习', 'Go', 'UI设计', 'Flutter']
}

/**
 * 获取推荐技能（暂按下载量排序取前 6，后续按用户画像个性化）
 * @returns {Promise<Array>}
 */
export async function getRecommendedSkills() {
  const body = await request('/api/skills?sort=downloads')
  return (body.data || []).slice(0, 6).map(normalizeSkill)
}

/**
 * 发布技能（需登录）。form 为 FormData，可含 archive 文件
 * @param {FormData} form
 * @returns {Promise<Object>}
 */
export async function createSkill(form) {
  const token = getToken()
  const resp = await fetch(`${BASE}/api/skills`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: form, // FormData，浏览器自动设置 multipart boundary
  })
  let data = null
  try {
    data = await resp.json()
  } catch (e) {
    /* ignore */
  }
  if (!resp.ok) {
    throw new Error((data && data.error) || `发布失败：${resp.status}`)
  }
  return normalizeSkill(data.data)
}

/**
 * AI 个性化解读（需登录）：按用户 AI 熟练度生成技能介绍
 * @param {number|string} id
 * @returns {Promise<{data: string, ai_level: string, level_label: string, cached: boolean}>}
 */
export async function explainSkill(id) {
  const token = getToken()
  const resp = await fetch(`${BASE}/api/skills/${id}/explain`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  let body = null
  try {
    body = await resp.json()
  } catch (e) {
    /* ignore */
  }
  if (!resp.ok) {
    throw new Error((body && body.error) || `生成失败：${resp.status}`)
  }
  return body
}

/**
 * 删除技能（需登录，仅属主可删）
 * @param {number|string} id
 */
export async function deleteSkill(id) {
  const token = getToken()
  const resp = await fetch(`${BASE}/api/skills/${id}`, {
    method: 'DELETE',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!resp.ok) {
    let msg = `删除失败：${resp.status}`
    try {
      const body = await resp.json()
      if (body.error) msg = body.error
    } catch (e) {
      /* ignore */
    }
    throw new Error(msg)
  }
}

// ---------- AI 引导创建 Skill ----------

async function guideRequest(path, payload, timeoutMs = 0) {
  const token = getToken()
  const controller = timeoutMs > 0 ? new AbortController() : null
  const timer = controller ? setTimeout(() => controller.abort(), timeoutMs) : null
  let resp
  try {
    resp = await fetch(`${BASE}${path}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify(payload),
      ...(controller ? { signal: controller.signal } : {}),
    })
  } catch (e) {
    if (controller && controller.signal.aborted) {
      throw new Error('生成超时，请稍后重试')
    }
    throw new Error('网络错误，请检查连接后重试')
  } finally {
    if (timer) clearTimeout(timer)
  }
  let data = null
  try {
    data = await resp.json()
  } catch (e) {
    /* ignore */
  }
  if (!resp.ok) {
    throw new Error((data && data.error) || `请求失败：${resp.status}`)
  }
  return data
}

/**
 * AI 引导对话（需登录）。messages 为完整对话历史，attachment 为当前消息附件（可选）
 * 后端每轮自动保存对话（虚拟自己蒸馏素材）并返回 conversation_id
 * @param {Array} messages [{role, content}]
 * @param {{type: 'image'|'file', name: string, mime: string, data: string}|null} [attachment]
 * @param {number} [conversationId] 上一轮返回的会话 id，首次传 0
 * @returns {Promise<{data: string, conversation_id: number}>}
 */
export async function guideChat(messages, attachment = null, conversationId = 0) {
  return guideRequest('/api/skills/guide/chat', {
    messages,
    attachment,
    conversation_id: conversationId,
  })
}

/**
 * 生成 skill 包（需登录）。依据对话历史生成完整 skill 包（JSON 描述 + zip base64）
 * @param {Array} messages 完整对话历史
 * @returns {Promise<{data: {name, title, description, category, tags, version, files, zip_base64}}>}
 */
export async function generateSkillPack(messages) {
  // 生成包含两阶段 LLM 调用（需求提炼 + 完整打包），给足 200 秒，超时明确报错而非无限转圈
  return guideRequest('/api/skills/guide/generate', { messages }, 200000)
}

// ---------- 评分 / 评价 ----------

/**
 * 提交评分与评价（需登录）。同一用户重复提交为更新
 * @param {number|string} skillId
 * @param {number} rating 1-5
 * @param {string} comment
 */
export async function submitReview(skillId, rating, comment) {
  const token = getToken()
  const resp = await fetch(`${BASE}/api/skills/${skillId}/reviews`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ rating, comment }),
  })
  let body = null
  try {
    body = await resp.json()
  } catch (e) {
    /* ignore */
  }
  if (!resp.ok) {
    throw new Error((body && body.error) || `提交失败：${resp.status}`)
  }
  return body
}

/**
 * 获取技能的评价列表（带 token 时额外返回当前用户的评价）
 * @param {number|string} skillId
 * @returns {Promise<{data: Array, my_review: Object|null, rating_avg: number, rating_count: number}>}
 */
export async function getReviews(skillId) {
  const token = getToken()
  const resp = await fetch(`${BASE}/api/skills/${skillId}/reviews`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!resp.ok) {
    throw new Error(`获取评价失败：${resp.status}`)
  }
  return resp.json()
}

// ---------- Issue 反馈（类 GitHub issue） ----------

/**
 * 提交 Issue 反馈（需登录），给生成 skill 的同学看
 * @param {number|string} skillId
 * @param {string} title
 * @param {string} body
 */
export async function createIssue(skillId, title, body) {
  const token = getToken()
  const resp = await fetch(`${BASE}/api/skills/${skillId}/issues`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ title, body }),
  })
  let data = null
  try {
    data = await resp.json()
  } catch (e) {
    /* ignore */
  }
  if (!resp.ok) {
    throw new Error((data && data.error) || `提交失败：${resp.status}`)
  }
  return data.data
}

/**
 * 获取技能的 Issue 列表（游客可用）
 * @param {number|string} skillId
 * @returns {Promise<Array>}
 */
export async function getIssues(skillId) {
  const resp = await fetch(`${BASE}/api/skills/${skillId}/issues`)
  if (!resp.ok) {
    throw new Error(`获取反馈失败：${resp.status}`)
  }
  const body = await resp.json()
  return body.data || []
}

/**
 * 关闭/重新打开 Issue（需登录，仅作者可操作）
 * @param {number|string} issueId
 * @param {'open'|'closed'} status
 */
export async function updateIssueStatus(issueId, status) {
  const token = getToken()
  const resp = await fetch(`${BASE}/api/issues/${issueId}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ status }),
  })
  let data = null
  try {
    data = await resp.json()
  } catch (e) {
    /* ignore */
  }
  if (!resp.ok) {
    throw new Error((data && data.error) || `操作失败：${resp.status}`)
  }
  return data.data
}
