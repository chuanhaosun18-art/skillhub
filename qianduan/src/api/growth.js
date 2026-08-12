// 成长闭环 API 层：目标识别 → 任务工作台 → Skill Creator → 门禁 → 路由 → Trust Card → 反馈升级
import { getToken, clearAuth } from './auth'

const BASE = import.meta.env.VITE_API_BASE || ''

async function req(path, { method = 'GET', body } = {}) {
  const headers = {}
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  const resp = await fetch(`${BASE}/api/growth${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  if (resp.status === 401) {
    clearAuth()
    throw new Error('登录已过期，请重新登录')
  }

  let data = null
  try {
    data = await resp.json()
  } catch (e) {
    data = null
  }

  if (!resp.ok) {
    // 门禁类错误（409）要把 blocked / still_missing 一起抛出，界面需要逐条展示
    const err = new Error((data && data.error) || `请求失败：${resp.status}`)
    err.status = resp.status
    err.payload = data
    throw err
  }
  return data
}

/* ---------- 单一入口 Agent ---------- */

/**
 * 现在该做什么。纯规则推导，不调模型——所以每次结果都一样。
 * 任何一次操作完成后都应该重新调它，拿下一张卡。
 */
export function agentState() {
  return req('/agent/state')
}

/** 用户新说了一句话。这是唯一会调模型的入口 */
export function agentSay(utterance) {
  return req('/agent/say', { method: 'POST', body: { utterance } })
}

/**
 * 执行卡片上的 action。
 * agent 只告诉前端该打哪个端点，业务逻辑仍在原端点里——
 * 所以门禁、硬约束、口径都不会因为走了 agent 而被绕过。
 */
export async function runAction(action, body) {
  const path = String(action.path || '').replace(/^\/api\/growth/, '')
  if (!path) return null
  return req(path, { method: action.method || 'POST', body })
}

/* ---------- F1 目标识别 ---------- */

/** 输入一句人话，返回任务卡 / 澄清 / 拒绝 / 手选兜底 */
export function interpretGoal(utterance) {
  return req('/goals/interpret', { method: 'POST', body: { utterance } })
}

/* ---------- F4 任务工作台 ---------- */

export function createExecution(payload) {
  return req('/executions', { method: 'POST', body: payload })
}

export function listMyExecutions() {
  return req('/executions')
}

export function getExecution(id) {
  return req(`/executions/${id}`)
}

/** 推进一步：返回 action / decision / tool / handoff / degraded */
export function advanceExecution(id) {
  return req(`/executions/${id}/advance`, { method: 'POST' })
}

/** 在关键判断点做出选择——这一步产生的是最有价值的数据 */
export function recordDecision(id, stepIndex, choice, note = '') {
  return req(`/executions/${id}/decide`, {
    method: 'POST',
    body: { step_index: stepIndex, choice, note },
  })
}

/** 改写某一步的输出，用于统计人工修正率 */
export function recordEdit(id, stepIndex, editedOutput) {
  return req(`/executions/${id}/edit`, {
    method: 'POST',
    body: { step_index: stepIndex, edited_output: editedOutput },
  })
}

export function completeExecution(id, { exported = false, finalArtifact = '' } = {}) {
  return req(`/executions/${id}/complete`, {
    method: 'POST',
    body: { exported, final_artifact: finalArtifact },
  })
}

export function abandonExecution(id) {
  return req(`/executions/${id}/abandon`, { method: 'POST' })
}

/* ---------- F5 Skill Creator ---------- */

/** 从执行轨迹生成 Skill 草稿 */
export function distillExecution(id) {
  return req(`/executions/${id}/distill`, { method: 'POST' })
}

export function getDraft(versionId) {
  return req(`/drafts/${versionId}`)
}

export function updateDraft(versionId, patch) {
  return req(`/drafts/${versionId}`, { method: 'PATCH', body: patch })
}

export function upsertDecision(versionId, decision) {
  return req(`/drafts/${versionId}/decisions`, { method: 'POST', body: decision })
}

export function deleteDecision(id) {
  return req(`/decisions/${id}`, { method: 'DELETE' })
}

/** 蒸馏度不足时的正确出口：存成经验笔记，不是判定失败 */
export function downgradeToInsight(versionId) {
  return req(`/drafts/${versionId}/downgrade`, { method: 'POST' })
}

/** 生成六 slot 文件夹并打 zip */
export function generateFolder(versionId) {
  return req(`/drafts/${versionId}/generate-folder`, { method: 'POST' })
}

/* ---------- F6 发布前四问与门禁 ---------- */

export function runEvals(skillId, type = '') {
  const q = type ? `?type=${encodeURIComponent(type)}` : ''
  return req(`/skills/${skillId}/evals/run${q}`, { method: 'POST' })
}

export function getGateStatus(skillId) {
  return req(`/skills/${skillId}/gate`)
}

export function publishSkill(skillId) {
  return req(`/skills/${skillId}/publish`, { method: 'POST' })
}

/* ---------- F8 路由 ---------- */

/** 两段式路由：先硬过滤候选集，再排序，并且必须给出解释 */
export function routeSkills(utterance, taskIntent = '') {
  return req('/route', { method: 'POST', body: { utterance, task_intent: taskIntent } })
}

/* ---------- F10 Trust Card ---------- */

export function getTrustCard(skillId) {
  return req(`/skills/${skillId}/trust-card`)
}

/** 判断级溯源（脱敏，不含原始材料） */
export function getDecisionTrace(decisionId) {
  return req(`/decisions/${decisionId}/trace`)
}

/* ---------- F12 反馈闭环 ---------- */

export function submitFeedback(executionId, payload) {
  return req(`/executions/${executionId}/feedback`, { method: 'POST', body: payload })
}

export function listVersionCandidates(skillId) {
  return req(`/skills/${skillId}/version-candidates`)
}

export function acceptVersionCandidate(candidateId, payload) {
  return req(`/version-candidates/${candidateId}/accept`, { method: 'POST', body: payload })
}

/* ---------- F13 成长路径与成长身份 ---------- */

/** 我的成长身份：当前位置、成长路线、四阶状态、能力资产、影响力、复盘 */
export function getMyGrowthProfile() {
  return req('/my-profile')
}

/** 看别人的成长身份（按其可见性设置过滤） */
export function getUserGrowthProfile(userId) {
  return req(`/profile/${userId}`)
}

/** 可见性开关，默认全部不公开 */
export function updateVisibility(patch) {
  return req('/my-profile/visibility', { method: 'PATCH', body: patch })
}

/* ---------- F17 编排态 ---------- */

/** 前置检查：这条路有没有人走完过。没有就不生成编排 */
export function probeOrchestration(utterance, orchestrationIntent) {
  return req('/orch-probe', {
    method: 'POST',
    body: { utterance, orchestration_intent: orchestrationIntent },
  })
}

/** 上下文访谈一轮 */
export function interviewOrchestration(payload) {
  return req('/orch-interview', { method: 'POST', body: payload })
}

export function createOrchestration(payload) {
  return req('/orchestrations', { method: 'POST', body: payload })
}

export function listMyOrchestrations() {
  return req('/orchestrations')
}

export function getOrchestration(id) {
  return req(`/orchestrations/${id}`)
}

export function adoptOrchestration(id) {
  return req(`/orchestrations/${id}/adopt`, { method: 'POST' })
}

export function updateOrchItem(id, itemId, patch) {
  return req(`/orchestrations/${id}/items/${itemId}`, { method: 'PATCH', body: patch })
}

/** 周复核：产出「节奏有没有跟上」这个行为信号 */
export function reviewOrchestration(id, payload) {
  return req(`/orchestrations/${id}/reviews`, { method: 'POST', body: payload })
}

/* ---------- F5.3b 轨迹补录 ---------- */

/** 在平台外做完的事补录进来。蒸馏度上限 0.85 */
export function backfillExecution(payload) {
  return req('/backfill', { method: 'POST', body: payload })
}

/* ---------- 常量 ---------- */

export const ORCHESTRATION_INTENTS = [
  { value: 'postgrad_recommend', label: '保研准备' },
  { value: 'postgrad_exam', label: '考研准备' },
  { value: 'study_abroad', label: '出国申请' },
  { value: 'job_season', label: '求职季' },
  { value: 'research_entry', label: '进组做科研' },
  { value: 'competition_season', label: '竞赛季' },
]

/** 允许进入任务流的任务类型（与后端 AllowedIntents 保持一致） */
export const TASK_INTENTS = [
  { value: 'thesis_topic', label: '论文选题打磨与收敛' },
  { value: 'resume_rewrite', label: '把科研经历改成产研岗位简历' },
  { value: 'resume_jd_align', label: '简历与具体 JD 对齐' },
  { value: 'report_structure', label: '组会汇报与答辩陈述结构' },
  { value: 'mock_interview', label: '模拟面试' },
  { value: 'interview_review', label: '面试复盘' },
  { value: 'project_convergence', label: '项目与竞赛方案收敛' },
  { value: 'literature_review', label: '文献综述入门与检索策略' },
  { value: 'content_script', label: '内容脚本与选题结构' },
]

export const DECISION_SLOTS = [
  { slot: 'when_to_check', prompt: '在哪一步你会停下来回头验证？' },
  { slot: 'when_to_probe', prompt: '什么情况下你会要求补充信息而不是直接动手？' },
  { slot: 'when_to_use_tool', prompt: '哪一步必须查、必须跑，不能靠判断？' },
  { slot: 'when_to_switch', prompt: '什么现象一出现，你就知道当前这条路走不通？' },
]

export const DIMENSION_LABELS = {
  real_task: '真实任务',
  outcome: '明确结果',
  workflow: '核心流程',
  decisions: '关键判断',
  failures: '失败案例',
  boundary: '适用边界',
}
