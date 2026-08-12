<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getSkillById, explainSkill, submitReview, getReviews, createIssue, getIssues, updateIssueStatus } from '../api/skills'
import { createDirectChat } from '../api/chat'
import { getPublicPersona, createPersonaChat } from '../api/persona'
import { authState } from '../api/auth'
import AppNavbar from '../components/AppNavbar.vue'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const skill = ref(null)
const notFound = ref(false)

// AI 个性化解读状态
const explaining = ref(false)
const explainText = ref('')
const explainLevel = ref('')
const explainCached = ref(false)

// 文件大小格式化
function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

// 用户问卷显示「电脑上未安装 AI 编码助手」时，展示安装引导
const userMissingAgent = computed(() => {
  const u = authState.user
  if (!u?.ai_quiz) return false
  try {
    const q = JSON.parse(u.ai_quiz)
    return q.has_agent_installed === false
  } catch (e) {
    return false
  }
})

const totalSizeText = computed(() => formatSize(skill.value?.total_size || 0))

// 文件清单折叠：默认只展示前 2 条，点击展开全部
const showAllFiles = ref(false)
const visibleFiles = computed(() => {
  if (!skill.value?.files) return []
  return showAllFiles.value ? skill.value.files : skill.value.files.slice(0, 2)
})

// ===== 评分 / 评价 =====
const reviews = ref([])
const myReview = ref(null)
const reviewRating = ref(0)
const reviewComment = ref('')
const submittingReview = ref(false)
const reviewLoading = ref(false)

// ===== Issue 反馈（类 GitHub issue） =====
const issues = ref([])
const issueTitle = ref('')
const issueBody = ref('')
const submittingIssue = ref(false)
const issueLoading = ref(false)

function formatDateTime(s) {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return ''
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function loadReviews() {
  reviewLoading.value = true
  try {
    const body = await getReviews(route.params.id)
    reviews.value = body.data || []
    myReview.value = body.my_review || null
    if (myReview.value) {
      reviewRating.value = myReview.value.rating
      reviewComment.value = myReview.value.comment || ''
    }
  } catch (e) {
    /* 静默失败，区块内展示空态 */
  } finally {
    reviewLoading.value = false
  }
}

async function loadIssues() {
  issueLoading.value = true
  try {
    issues.value = await getIssues(route.params.id)
  } catch (e) {
    /* 静默失败 */
  } finally {
    issueLoading.value = false
  }
}

async function submitReviewHandler() {
  if (!authState.token) {
    ElMessage.warning('请先登录')
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  if (!reviewRating.value) {
    ElMessage.warning('请先选择星级评分')
    return
  }
  submittingReview.value = true
  try {
    await submitReview(route.params.id, reviewRating.value, reviewComment.value.trim())
    ElMessage.success(myReview.value ? '评价已更新' : '评价已提交，感谢你的反馈！')
    // 刷新评价与技能评分
    await loadReviews()
    skill.value = await getSkillById(route.params.id)
  } catch (e) {
    ElMessage.error(e.message || '评价提交失败')
  } finally {
    submittingReview.value = false
  }
}

async function submitIssueHandler() {
  if (!authState.token) {
    ElMessage.warning('请先登录')
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  if (!issueTitle.value.trim()) {
    ElMessage.warning('请填写反馈标题')
    return
  }
  submittingIssue.value = true
  try {
    await createIssue(route.params.id, issueTitle.value.trim(), issueBody.value.trim())
    ElMessage.success('反馈已提交，作者会看到你的建议！')
    issueTitle.value = ''
    issueBody.value = ''
    await loadIssues()
  } catch (e) {
    ElMessage.error(e.message || '反馈提交失败')
  } finally {
    submittingIssue.value = false
  }
}

async function toggleIssueStatus(issue) {
  try {
    const target = issue.status === 'open' ? 'closed' : 'open'
    await updateIssueStatus(issue.id, target)
    ElMessage.success(target === 'closed' ? '已关闭该反馈' : '已重新打开')
    await loadIssues()
  } catch (e) {
    ElMessage.error(e.message || '操作失败')
  }
}

const createdText = computed(() => {
  const d = skill.value?.created_at
  if (!d) return ''
  return new Date(d).toLocaleDateString('zh-CN')
})

onMounted(async () => {
  loading.value = true
  try {
    skill.value = await getSkillById(route.params.id)
    if (!skill.value) notFound.value = true
  } catch (e) {
    notFound.value = true
  } finally {
    loading.value = false
  }
  if (skill.value) {
    loadReviews()
    loadIssues()
  }
})

// ===== AI 个性化解读 =====
const aiLevelLabelMap = {
  never: '从未用过',
  beginner: '初级',
  intermediate: '中级',
  advanced: '高级'
}

async function generateExplain() {
  if (!authState.token) {
    ElMessage.warning('请先登录')
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  explaining.value = true
  explainText.value = ''
  try {
    const data = await explainSkill(route.params.id)
    explainText.value = data.data || ''
    explainLevel.value = data.level_label || ''
    explainCached.value = !!data.cached
  } catch (e) {
    ElMessage.error(e.response?.data?.error || 'AI 解读生成失败，请稍后重试')
  } finally {
    explaining.value = false
  }
}

// 安全的 mini markdown 渲染：先转义 HTML，再处理标题/加粗/列表/引用/代码
function escapeHtml(s) {
  return (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

function renderMarkdown(text) {
  if (!text) return ''
  const lines = String(text).replace(/\r\n/g, '\n').split('\n')
  let html = ''
  let inList = false
  for (const line of lines) {
    let t = escapeHtml(line).trim()
    if (!t) {
      if (inList) { html += '</ul>'; inList = false }
      continue
    }
    let m
    if ((m = t.match(/^(#{1,6})\s+(.*)$/))) {
      if (inList) { html += '</ul>'; inList = false }
      const level = m[1].length
      html += `<h${level}>${m[2]}</h${level}>`
    } else if ((m = t.match(/^[-*]\s+(.*)$/)) || (m = t.match(/^\d+\.\s+(.*)$/))) {
      if (!inList) { html += '<ul>'; inList = true }
      html += `<li>${m[1]}</li>`
    } else if ((m = t.match(/^>\s?(.*)$/))) {
      if (inList) { html += '</ul>'; inList = false }
      html += `<blockquote>${m[1]}</blockquote>`
    } else if ((m = t.match(/^```/))) {
      if (inList) { html += '</ul>'; inList = false }
      html += '<pre class="code-block"></pre>'
    } else {
      if (inList) { html += '</ul>'; inList = false }
      // 行内加粗 / 行内代码
      t = t.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      t = t.replace(/`([^`]+)`/g, '<code>$1</code>')
      html += `<p>${t}</p>`
    }
  }
  if (inList) html += '</ul>'
  return html
}

function download() {
  if (!skill.value) return
  window.open(`/api/skills/${skill.value.id}/download`, '_blank')
}

// 与技能创建者聊天：选择「实时在线」或「与虚拟的 TA 聊」（复用 /chat 与 /persona-chat）
const chatDialog = ref(false)
const chatting = ref(false)
const ownerId = ref(0)
const ownerPersona = ref({ chat_enabled: 0 })
const allowOwnerView = ref(false)

async function chatWithOwner() {
  if (!authState.token) {
    ElMessage.warning('请先登录')
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  ownerId.value = Number(skill.value?.owner_id)
  if (!ownerId.value) {
    ElMessage.info('这个技能暂无作者，无法发起聊天')
    return
  }
  if (ownerId.value === authState.user?.id) {
    ElMessage.info('这是你自己发布的技能')
    return
  }
  allowOwnerView.value = false
  ownerPersona.value = { chat_enabled: 0 }
  chatDialog.value = true
  // 后台拉取对方虚拟自己状态（失败不影响实时聊天入口）
  getPublicPersona(ownerId.value)
    .then((p) => { ownerPersona.value = p || { chat_enabled: 0 } })
    .catch(() => { /* ignore */ })
}

async function startRealChat() {
  chatting.value = true
  try {
    const res = await createDirectChat(ownerId.value)
    chatDialog.value = false
    router.push({ path: `/chat/${res.chat_id}`, query: { name: skill.value.owner } })
  } catch (e) {
    ElMessage.error(e.message || '发起聊天失败')
  } finally {
    chatting.value = false
  }
}

async function startPersonaChat() {
  chatting.value = true
  try {
    const res = await createPersonaChat(ownerId.value, allowOwnerView.value)
    chatDialog.value = false
    router.push({ path: `/persona-chat/${res.chat_id}`, query: { name: skill.value.owner } })
  } catch (e) {
    ElMessage.error(e.message || '开始聊天失败')
  } finally {
    chatting.value = false
  }
}

function goBack() {
  router.back()
}
</script>

<template>
  <div class="detail-page">
    <AppNavbar>
      <el-button size="small" @click="goBack">
        <el-icon style="margin-right: 4px"><ArrowLeft /></el-icon>返回
      </el-button>
    </AppNavbar>

    <main class="content">
      <el-skeleton :rows="8" animated v-if="loading" class="skeleton" />

      <el-result
        v-else-if="notFound"
        icon="warning"
        title="技能不存在或已被删除"
      >
        <template #extra>
          <el-button type="primary" @click="router.push('/')">返回首页</el-button>
        </template>
      </el-result>

      <template v-else-if="skill">
        <!-- 基本信息 -->
        <section class="card header-card">
          <div class="header-row">
            <h1 class="name">
              {{ skill.name }}
              <el-tag v-if="skill.version" size="small" effect="plain">v{{ skill.version }}</el-tag>
            </h1>
            <!-- 星级已移除：要判断能不能把活交给它，看的是证据不是分数 -->
            <router-link :to="'/trust/' + skill.id" class="trust-entry">
              看它的证据（Trust Card）
            </router-link>
          </div>
          <p class="desc">{{ skill.description }}</p>
          <div class="meta-row">
            <el-tag size="small" type="success" effect="light">{{ skill.category || '未分类' }}</el-tag>
            <el-tag v-for="tag in skill.tags" :key="tag" size="small" type="info" effect="plain">
              {{ tag }}
            </el-tag>
          </div>

          <!-- 统计 -->
          <div class="stats-row">
            <div class="stat-item">
              <div class="stat-num">{{ skill.file_count }}</div>
              <div class="stat-label">文件数</div>
            </div>
            <div class="stat-item">
              <div class="stat-num">{{ totalSizeText }}</div>
              <div class="stat-label">包大小</div>
            </div>
            <div class="stat-item secondary">
              <div class="stat-num">{{ skill.download_count }}</div>
              <div class="stat-label">下载量（注意力参考）</div>
            </div>
            <div class="stat-item secondary">
              <div class="stat-num">{{ skill.view_count }}</div>
              <div class="stat-label">浏览数（注意力参考）</div>
            </div>
          </div>

          <div class="action-row">
            <el-button type="primary" size="large" round @click="download">
              <el-icon style="margin-right: 4px"><Download /></el-icon>
              下载 Skill 包
            </el-button>
            <el-button size="large" round :loading="chatting" @click="chatWithOwner">
              <el-icon style="margin-right: 4px"><ChatDotRound /></el-icon>
              和创建者聊聊
            </el-button>
            <span class="owner-info">
              发布者：{{ skill.owner }} · {{ createdText }}
            </span>
          </div>
        </section>

        <!-- 选择聊天方式：实时在线 或 与虚拟的 TA 聊 -->
        <el-dialog v-model="chatDialog" :title="`和 ${skill.owner} 聊聊`" width="460px">
          <div class="chat-options">
            <div class="chat-option" :class="{ disabled: chatting }" @click="startRealChat">
              <div class="chat-option-icon"><el-icon size="22"><ChatDotRound /></el-icon></div>
              <div class="chat-option-body">
                <div class="chat-option-title">实时在线聊天</div>
                <div class="chat-option-desc">TA 本人实时回复（TA 在线时）</div>
              </div>
              <el-icon v-if="chatting" class="is-loading" style="color:#909399"><Loading /></el-icon>
            </div>

            <template v-if="ownerPersona.chat_enabled">
              <div class="chat-option" :class="{ disabled: chatting }" @click="startPersonaChat">
                <div class="chat-option-icon chat-option-icon-virtual"><el-icon size="22"><MagicStick /></el-icon></div>
                <div class="chat-option-body">
                  <div class="chat-option-title">与虚拟的 TA 聊天</div>
                  <div class="chat-option-desc">由 TA 的「虚拟自己」回复，随时可聊</div>
                </div>
              </div>
              <el-checkbox v-model="allowOwnerView" class="chat-allow-box">
                允许 TA 查看本次虚拟聊天记录
              </el-checkbox>
              <p class="chat-allow-hint">不勾选则只有你能看到这段对话。</p>
            </template>
            <p v-else class="chat-persona-off">TA 还没开启「虚拟自己」，目前只能实时在线聊天。</p>
          </div>
          <template #footer>
            <el-button @click="chatDialog = false">取消</el-button>
          </template>
        </el-dialog>

        <!-- AI 个性化解读 -->
        <section class="card ai-card">
          <div class="ai-header">
            <h2 class="section-title" style="margin-bottom: 0">
              <el-icon style="vertical-align: -2px; margin-right: 4px"><MagicStick /></el-icon>
              AI 个性化解读
            </h2>
            <el-tag v-if="explainLevel" size="small" type="warning" effect="light">
              适合：{{ explainLevel }}
            </el-tag>
            <el-tag v-else-if="authState.user?.ai_level" size="small" type="warning" effect="light">
              你的水平：{{ aiLevelLabelMap[authState.user.ai_level] || authState.user.ai_level }}
            </el-tag>
          </div>

          <!-- 未安装 AI 编码助手：引导下载安装 -->
          <el-alert
            v-if="authState.token && userMissingAgent"
            type="warning"
            :closable="false"
            show-icon
            class="agent-warning"
          >
            <template #title>你的电脑还没有安装 AI 编码助手</template>
            <p class="agent-warning-body">
              下载 Skill 包后，需要 <strong>Trae</strong> / <strong>Cursor</strong> / <strong>Codex</strong> 等 AI 编码助手才能运行。请先安装一个（推荐中文友好、免费的 Trae）：
            </p>
            <ul class="agent-warning-links">
              <li><a href="https://www.trae.ai" target="_blank" rel="noopener">Trae 官网（字节跳动出品 · 中文友好 · 免费）</a></li>
              <li><a href="https://www.cursor.com" target="_blank" rel="noopener">Cursor 官网（AI 代码编辑器）</a></li>
              <li>Codex（OpenAI）：在 ChatGPT 应用内开启 Codex 功能即可使用</li>
            </ul>
            <p class="agent-warning-body">
              安装完成后回到本页，点击下方「生成个性化解读」，AI 会结合你的电脑环境给出完整的上手步骤。
            </p>
          </el-alert>

          <template v-if="!authState.token">
            <p class="ai-tip">
              登录后，AI 会根据你的使用经验（从未用过 / 初级 / 中级 / 高级）为你生成一份专属的技能介绍——新手看得懂，老手看到干货。
            </p>
            <el-button size="small" round @click="router.push({ path: '/login', query: { redirect: route.fullPath } })">
              去登录
            </el-button>
          </template>

          <template v-else-if="!explainText && !explaining">
            <p class="ai-tip">
              点击生成一份针对你当前水平的技能解读（内容由 DeepSeek 生成）。
            </p>
            <el-button type="primary" round :loading="explaining" @click="generateExplain">
              <el-icon style="margin-right: 4px"><MagicStick /></el-icon>
              生成个性化解读
            </el-button>
          </template>

          <template v-else>
            <div v-if="explaining" class="ai-loading">
              <el-icon class="is-loading" style="font-size: 22px"><Loading /></el-icon>
              <span>AI 正在阅读技能包内容并生成解读…</span>
            </div>
            <template v-else>
              <div class="ai-meta">
                <el-tag v-if="explainCached" size="small" type="info" effect="plain">缓存命中 · 无需重复生成</el-tag>
                <el-button link size="small" @click="generateExplain">重新生成</el-button>
              </div>
              <div class="ai-content" v-html="renderMarkdown(explainText)"></div>
            </template>
          </template>
        </section>

        <!-- 文件清单 -->
        <section class="card" v-if="skill.files && skill.files.length">
          <h2 class="section-title">
            文件清单
            <span class="file-count">共 {{ skill.files.length }} 个文件</span>
          </h2>
          <el-table :data="visibleFiles" size="small" stripe>
            <el-table-column prop="file_path" label="文件路径" />
            <el-table-column label="大小" width="120">
              <template #default="{ row }">{{ formatSize(row.size) }}</template>
            </el-table-column>
            <el-table-column prop="sha256" label="SHA256" min-width="280" show-overflow-tooltip />
          </el-table>
          <div class="file-list-toggle" v-if="skill.files.length > 2">
            <el-button link type="primary" @click="showAllFiles = !showAllFiles">
              <el-icon class="toggle-icon" :style="{ transform: showAllFiles ? 'rotate(180deg)' : '' }">
                <ArrowDown />
              </el-icon>
              {{ showAllFiles ? '收起文件清单' : `显示全部 ${skill.files.length} 个文件` }}
            </el-button>
          </div>
        </section>

        <!-- 评分与评价 -->
        <section class="card">
          <h2 class="section-title">使用者评价</h2>
          <el-alert
            type="info"
            :closable="false"
            title="评价不参与排序，也不作为信任依据"
            description="一个总分没法告诉你它在什么情况下有效、在哪里会失败。要判断能不能把活交给它，请看 Trust Card 里的证据、边界和判断级溯源。"
            style="margin-bottom: 14px"
          />

          <div class="rating-summary" v-loading="reviewLoading">
            <el-rate :model-value="skill.rating" disabled text-color="#ff9900" />
            <span class="rating-summary-text">
              {{ skill.rating_count > 0 ? `${skill.rating.toFixed(1)} 分 · ${skill.rating_count} 人评价` : '暂无评分' }}
            </span>
          </div>

          <!-- 我的评价 / 提交评价 -->
          <div class="review-box" v-if="authState.token">
            <div class="review-box-label">{{ myReview ? '我的评价（可修改）' : '使用过这个 Skill 吗？来打个分吧' }}</div>
            <el-rate v-model="reviewRating" :max="5" show-text />
            <el-input
              v-model="reviewComment"
              type="textarea"
              :rows="3"
              maxlength="500"
              show-word-limit
              placeholder="说说你的使用感受，帮助其他人判断这个 Skill 是否适合自己（可选）"
            />
            <el-button type="primary" round :loading="submittingReview" @click="submitReviewHandler">
              {{ myReview ? '更新评价' : '提交评价' }}
            </el-button>
          </div>
          <div class="review-box review-box-guest" v-else>
            <span>登录后即可为这个 Skill 打分和写评价，评价会帮助平台更好地做搜索推荐。</span>
            <el-button size="small" round @click="router.push({ path: '/login', query: { redirect: route.fullPath } })">
              去登录
            </el-button>
          </div>

          <!-- 评价列表 -->
          <el-empty v-if="!reviewLoading && !reviews.length" description="还没有评价，来当第一个评价的人吧" :image-size="60" />
          <div v-else class="review-list" v-loading="reviewLoading">
            <div v-for="r in reviews" :key="r.id" class="review-item">
              <div class="review-head">
                <span class="review-user">{{ r.username }}</span>
                <el-rate :model-value="r.rating" disabled size="small" text-color="#ff9900" />
                <span class="review-time">{{ formatDateTime(r.updated_at) }}</span>
              </div>
              <p class="review-comment">{{ r.comment || '（无文字评价）' }}</p>
            </div>
          </div>
        </section>

        <!-- Issue 反馈（类 GitHub issue） -->
        <section class="card">
          <h2 class="section-title">反馈与建议</h2>
          <p class="section-sub">遇到问题、想提新功能，或想给 Skill 作者一些改进建议？像 GitHub Issue 一样提交反馈，作者会看到。</p>

          <div class="issue-box" v-if="authState.token">
            <el-input v-model="issueTitle" placeholder="反馈标题，一句话概括问题或建议" maxlength="100" show-word-limit />
            <el-input
              v-model="issueBody"
              type="textarea"
              :rows="3"
              maxlength="1000"
              show-word-limit
              placeholder="详细描述（可选）：期望的效果、遇到的问题、复现步骤等"
            />
            <el-button type="warning" round plain :loading="submittingIssue" @click="submitIssueHandler">
              提交反馈
            </el-button>
          </div>
          <div class="issue-box issue-box-guest" v-else>
            <span>登录后可以提交反馈，帮助作者改进这个 Skill。</span>
            <el-button size="small" round @click="router.push({ path: '/login', query: { redirect: route.fullPath } })">
              去登录
            </el-button>
          </div>

          <el-empty v-if="!issueLoading && !issues.length" description="暂无反馈" :image-size="60" />
          <div v-else class="issue-list" v-loading="issueLoading">
            <div v-for="iss in issues" :key="iss.id" class="issue-item">
              <div class="issue-head">
                <el-tag :type="iss.status === 'open' ? 'success' : 'info'" size="small" effect="light">
                  {{ iss.status === 'open' ? '待处理' : '已关闭' }}
                </el-tag>
                <span class="issue-title">{{ iss.title }}</span>
                <span class="issue-op" v-if="authState.user && iss.user_id === authState.user.id" @click="toggleIssueStatus(iss)">
                  {{ iss.status === 'open' ? '关闭' : '重新打开' }}
                </span>
              </div>
              <p class="issue-body">{{ iss.body || '（无详细描述）' }}</p>
              <div class="issue-meta">
                由 {{ iss.username }} 提交于 {{ formatDateTime(iss.created_at) }}
                <template v-if="iss.status === 'closed' && iss.closed_at"> · 关闭于 {{ formatDateTime(iss.closed_at) }}</template>
              </div>
            </div>
          </div>
        </section>
      </template>
    </main>

    <footer class="footer">
      <span>SkillHub © 2026 · 技能发现与分享平台</span>
    </footer>
  </div>
</template>

<style scoped>
.detail-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}

.content {
  flex: 1;
  max-width: 960px;
  width: 100%;
  margin: 0 auto;
  padding: 24px;
}

.skeleton {
  padding: 16px;
  background: #fff;
  border-radius: 8px;
}

.card {
  background: #fff;
  border-radius: 10px;
  padding: 24px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  margin-bottom: 16px;
}

.header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.name {
  font-size: 22px;
  font-weight: 700;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 8px;
}

.desc {
  font-size: 14px;
  color: #606266;
  line-height: 1.7;
  margin-bottom: 12px;
}

.meta-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 20px;
}

.stats-row {
  display: flex;
  gap: 40px;
  padding: 16px 0;
  border-top: 1px solid #f0f2f5;
  border-bottom: 1px solid #f0f2f5;
  margin-bottom: 20px;
}

.stat-num {
  font-size: 20px;
  font-weight: 700;
  color: #303133;
}

.stat-label {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.action-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.owner-info {
  font-size: 13px;
  color: #909399;
}

/* 聊天方式选择弹窗 */
.chat-options {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.chat-option {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid #e4e7ed;
  border-radius: 10px;
  cursor: pointer;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.chat-option:hover {
  border-color: #409eff;
  box-shadow: 0 2px 10px rgba(64, 158, 255, 0.12);
}
.chat-option.disabled {
  opacity: 0.6;
  pointer-events: none;
}
.chat-option-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  border-radius: 10px;
  color: #409eff;
  background: #ecf5ff;
}
.chat-option-icon-virtual {
  color: #67c23a;
  background: #f0f9eb;
}
.chat-option-body {
  flex: 1;
  min-width: 0;
}
.chat-option-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}
.chat-option-desc {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}
.chat-allow-box {
  margin: 2px 4px 0;
}
.chat-allow-hint {
  margin: 4px 4px 0;
  font-size: 12px;
  color: #c0c4cc;
}
.chat-persona-off {
  margin: 4px 4px 0;
  font-size: 12px;
  color: #909399;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 12px;
}

.file-count {
  font-size: 13px;
  font-weight: 400;
  color: #909399;
  margin-left: 8px;
}

.file-list-toggle {
  display: flex;
  justify-content: center;
  margin-top: 8px;
}

.toggle-icon {
  margin-right: 4px;
  transition: transform 0.2s;
}

/* AI 解读区块 */
.ai-card {
  border: 1px solid #f0e6d2;
  background: linear-gradient(180deg, #fffdf8 0%, #fff 100%);
}

.ai-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.ai-tip {
  font-size: 13px;
  color: #909399;
  line-height: 1.7;
  margin: 0 0 12px;
}

/* 未安装 Agent 引导条 */
.agent-warning {
  margin-bottom: 16px;
  text-align: left;
}

.agent-warning-body {
  font-size: 13px;
  line-height: 1.7;
  margin: 6px 0 0;
}

.agent-warning-links {
  margin: 6px 0 0;
  padding-left: 18px;
  font-size: 13px;
  line-height: 1.8;
}

.agent-warning-links a {
  color: #e6a23c;
  font-weight: 600;
  text-decoration: none;
}

.agent-warning-links a:hover {
  text-decoration: underline;
}

.ai-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #606266;
  font-size: 14px;
  padding: 8px 0;
}

.ai-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.ai-content {
  font-size: 14px;
  line-height: 1.8;
  color: #303133;
}

.ai-content h1, .ai-content h2, .ai-content h3 {
  font-size: 15px;
  font-weight: 700;
  margin: 14px 0 8px;
  color: #1f2329;
}

.ai-content h4, .ai-content h5, .ai-content h6 {
  font-size: 14px;
  font-weight: 600;
  margin: 12px 0 6px;
}

.ai-content p {
  margin: 6px 0;
}

.ai-content ul {
  margin: 6px 0;
  padding-left: 20px;
}

.ai-content li {
  margin: 3px 0;
}

.ai-content blockquote {
  margin: 8px 0;
  padding: 6px 12px;
  border-left: 3px solid #e4a853;
  background: #fffbf2;
  color: #8a6d3b;
  border-radius: 0 6px 6px 0;
}

.ai-content strong {
  color: #e0851a;
  font-weight: 600;
}

.ai-content code {
  background: #f5f7fa;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  padding: 1px 5px;
  font-size: 12px;
  color: #d14;
  font-family: Consolas, Monaco, 'Courier New', monospace;
}

.ai-content .code-block {
  margin: 8px 0;
  padding: 10px;
  background: #1e1e1e;
  color: #d4d4d4;
  border-radius: 6px;
  overflow-x: auto;
}

.ai-content em {
  color: #909399;
}

/* Trust Card 入口取代原来的星级 */
.trust-entry {
  font-size: 13px;
  color: #409eff;
  text-decoration: none;
  white-space: nowrap;
  border: 1px solid #409eff;
  border-radius: 14px;
  padding: 4px 12px;
}
.trust-entry:hover {
  background: #ecf5ff;
}
/* 热度类指标降级：视觉上弱化，避免被当成信任依据 */
.stat-item.secondary .stat-num {
  color: #909399;
  font-weight: 400;
}
.stat-item.secondary .stat-label {
  color: #c0c4cc;
}

/* 评分与评价 */
.rating-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0 4px;
}

.rating-summary-text {
  font-size: 13px;
  color: #909399;
}

.review-box {
  margin-top: 12px;
  padding: 16px;
  background: #fafafa;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: flex-start;
}

.review-box-label {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.review-box .el-input, .review-box .el-textarea {
  width: 100%;
}

.review-box-guest {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  color: #909399;
  flex-wrap: wrap;
  gap: 8px;
}

.review-list {
  margin-top: 16px;
}

.review-item {
  padding: 12px 0;
  border-bottom: 1px solid #f0f2f5;
}

.review-item:last-child {
  border-bottom: none;
}

.review-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.review-user {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.review-time {
  font-size: 12px;
  color: #c0c4cc;
}

.review-comment {
  font-size: 13px;
  color: #606266;
  line-height: 1.7;
  margin: 6px 0 0;
}

/* Issue 反馈 */
.section-sub {
  font-size: 13px;
  color: #909399;
  line-height: 1.7;
  margin: -6px 0 14px;
}

.issue-box {
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: flex-start;
  padding: 16px;
  background: #fafafa;
  border-radius: 8px;
  margin-bottom: 16px;
}

.issue-box .el-input, .issue-box .el-textarea {
  width: 100%;
}

.issue-box-guest {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  color: #909399;
  flex-wrap: wrap;
  gap: 8px;
}

.issue-item {
  padding: 12px 0;
  border-bottom: 1px solid #f0f2f5;
}

.issue-item:last-child {
  border-bottom: none;
}

.issue-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.issue-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.issue-op {
  font-size: 12px;
  color: #409eff;
  cursor: pointer;
  margin-left: auto;
}

.issue-op:hover {
  text-decoration: underline;
}

.issue-body {
  font-size: 13px;
  color: #606266;
  line-height: 1.7;
  margin: 6px 0 4px;
  white-space: pre-wrap;
}

.issue-meta {
  font-size: 12px;
  color: #c0c4cc;
}

.footer {
  text-align: center;
  padding: 24px;
  color: #c0c4cc;
  font-size: 13px;
}
</style>
