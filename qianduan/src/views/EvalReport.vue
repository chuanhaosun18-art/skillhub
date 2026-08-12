<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  startEvalPipeline, getEvalReport, getHumanReviewCases, submitHumanReview,
} from '../api/growth'
import AppNavbar from '../components/AppNavbar.vue'

const route = useRoute()
const skillId = route.params.id

const report = ref(null)
const reviewItems = ref([])
const loading = ref(false)
const starting = ref(false)
let timer = null

// 管道是否进行中
const running = computed(() => {
  const s = report.value && report.value.run && report.value.run.status
  return s === 'running' || s === 'pending'
})

const stageLabel = {
  static_scan: '静态扫描',
  sandbox: '沙箱执行',
  agents: '评测 Agent',
  human_review: '人工复核',
  report: '报告',
}

const statusType = {
  running: 'warning',
  pending: 'info',
  passed: 'success',
  needs_review: 'warning',
  rejected: 'danger',
}

const statusLabel = {
  running: '评测中',
  pending: '排队中',
  passed: '通过',
  needs_review: '待人工复核',
  rejected: '拒绝上架',
}

// 四问（从 results 中提取）
const fourQuestions = computed(() => {
  if (!report.value || !report.value.results) return []
  return report.value.results.filter((r) => String(r.item).includes('四问'))
})

// 一票否决项：safety_redline / boundary_stop 且未通过
const vetoes = computed(() => {
  if (!report.value || !report.value.results) return []
  return report.value.results.filter((r) => !r.passed && ['safety_redline', 'boundary_stop'].includes(String(r.agent)))
})

// 强验证（F2P/P2P）断言明细：从沙箱记录 checks 字段聚合
const verifyData = computed(() => {
  if (!report.value || !report.value.sandbox_runs) return { checks: [], f2p: [], p2p: [], f2pPassed: 0, p2pPassed: 0 }
  const checks = []
  report.value.sandbox_runs.forEach((s, idx) => {
    let arr = []
    try {
      const v = JSON.parse(s.checks || '[]')
      arr = Array.isArray(v) ? v : []
    } catch (e) {
      /* 忽略损坏的 checks */
    }
    arr.forEach((c) => checks.push({ ...c, input: s.input, runIndex: idx + 1 }))
  })
  const f2p = checks.filter((c) => c.group === 'f2p')
  const p2p = checks.filter((c) => c.group === 'p2p')
  return {
    checks,
    f2p,
    p2p,
    f2pPassed: f2p.filter((c) => c.passed).length,
    p2pPassed: p2p.filter((c) => c.passed).length,
  }
})

// 契约中配置的强验证断言（配置预览）
const verifyConfig = computed(() => {
  if (!report.value || !report.value.contract || !report.value.contract.verification) return null
  try {
    return JSON.parse(report.value.contract.verification)
  } catch (e) {
    return null
  }
})

// 运行环境：技术栈 / 语言版本 / 解析出的 Docker 镜像
const envInfo = computed(() => {
  const e = report.value && report.value.env
  if (!e) return { runtime: '—', stack: '—', image: '—' }
  const stack = [e.language, e.language_version].filter(Boolean).join(' ')
  return {
    runtime: e.runtime || '—',
    stack: stack || '—',
    image: e.image || '（默认镜像）',
  }
})

function parseList(s) {
  if (!s) return []
  try {
    const v = JSON.parse(s)
    return Array.isArray(v) ? v : []
  } catch (e) {
    return []
  }
}

function parseEvidence(s) {
  if (!s || s === 'null' || s === '') return ''
  try {
    const v = JSON.parse(s)
    return typeof v === 'string' ? v : JSON.stringify(v)
  } catch (e) {
    return s
  }
}

async function fetchReport() {
  try {
    report.value = await getEvalReport(skillId)
  } catch (e) {
    ElMessage.error(e.message || '获取报告失败')
  }
}

async function fetchReview() {
  try {
    const d = await getHumanReviewCases(skillId)
    reviewItems.value = (d && d.items) || []
  } catch (e) {
    /* 忽略：非核心数据 */
  }
}

async function handleStart() {
  starting.value = true
  try {
    const d = await startEvalPipeline(skillId)
    ElMessage.success(d.message || '评测管道已启动')
    await fetchReport()
  } catch (e) {
    ElMessage.error(e.message || '启动评测失败')
  } finally {
    starting.value = false
  }
}

const reviewForm = ref({ result_id: null, decision: 'approve', note: '' })

function openReview(item) {
  reviewForm.value = { result_id: item.result_id, decision: 'approve', note: '' }
}

async function submitReview() {
  if (!reviewForm.value.result_id) return
  try {
    await submitHumanReview({
      result_id: reviewForm.value.result_id,
      decision: reviewForm.value.decision,
      note: reviewForm.value.note,
    })
    ElMessage.success('已提交人工复核结果')
    reviewForm.value.result_id = null
    await fetchReview()
    await fetchReport()
  } catch (e) {
    ElMessage.error(e.message || '提交失败')
  }
}

// 每 10 秒轮询一次（管道运行中）
function startPolling() {
  stopPolling()
  timer = setInterval(async () => {
    await fetchReport()
    await fetchReview()
    if (!running.value) stopPolling()
  }, 10000)
}
function stopPolling() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(async () => {
  await fetchReport()
  await fetchReview()
  if (running.value) startPolling()
})
onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="eval-page">
    <AppNavbar />

    <main class="eval-main">
      <!-- 状态头 -->
      <div class="eval-header">
        <div>
          <h1 class="page-title">评测报告 · Skill #{{ skillId }}</h1>
          <p class="page-sub">静态扫描 → 沙箱执行 → 评测 Agent → 人工复核 → 上架决策</p>
        </div>
        <div class="header-actions">
          <el-button type="primary" :loading="starting" :disabled="running" @click="handleStart">
            {{ running ? '评测进行中…' : '重新评测' }}
          </el-button>
        </div>
      </div>

      <el-skeleton v-if="!report" :rows="6" animated />

      <template v-else>
        <el-empty v-if="!report.run" description="尚未运行评测管道，点击「重新评测」开始" />

        <template v-else>
          <!-- 管道阶段 -->
          <div class="card">
            <div class="card-row">
              <span class="stage-tag">{{ stageLabel[report.run.stage] || report.run.stage }}</span>
              <el-tag :type="statusType[report.run.status] || 'info'" size="large">
                {{ statusLabel[report.run.status] || report.run.status }}
              </el-tag>
              <span class="run-meta">#{{ report.run.id }} · {{ report.run.started_at }}</span>
            </div>
            <p v-if="report.run.summary" class="run-summary">{{ report.run.summary }}</p>

            <el-steps :active="1" align-center class="pipe-steps">
              <el-step title="静态扫描" :status="report.run.stage === 'static_scan' ? 'process' : 'finish'" />
              <el-step title="沙箱执行" :status="report.run.stage === 'sandbox' ? 'process' : 'finish'" />
              <el-step title="评测 Agent" :status="report.run.stage === 'agents' ? 'process' : 'finish'" />
              <el-step title="人工复核" :status="report.run.stage === 'human_review' ? 'process' : 'finish'" />
              <el-step title="报告决策" :status="report.run.stage === 'report' ? 'process' : 'finish'" />
            </el-steps>
          </div>

          <!-- 契约概览 -->
          <div v-if="report.contract" class="card">
            <h3 class="card-title">测试契约</h3>
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="类型">
                <el-tag :type="report.contract.skill_type === '经验型' ? 'success' : 'primary'" size="small">
                  {{ report.contract.skill_type }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="运行方式">{{ envInfo.runtime }}</el-descriptions-item>
              <el-descriptions-item label="技术栈">{{ envInfo.stack }}</el-descriptions-item>
              <el-descriptions-item label="Docker 镜像">
                <code class="env-image">{{ envInfo.image }}</code>
              </el-descriptions-item>
              <el-descriptions-item label="何时被唤起" :span="2">{{ report.contract.trigger_description }}</el-descriptions-item>
              <el-descriptions-item label="做完的标准" :span="2">{{ report.contract.completion_definition }}</el-descriptions-item>
              <el-descriptions-item label="边界声明" :span="2">{{ report.contract.boundary_statement }}</el-descriptions-item>
              <el-descriptions-item label="变体输入">
                <el-tag v-for="(t, i) in parseList(report.contract.robustness_examples)" :key="i" size="small" class="tag-item">{{ t }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="过程检查表">
                <el-tag v-for="(t, i) in parseList(report.contract.process_checklist)" :key="i" size="small" type="success" class="tag-item">{{ t }}</el-tag>
              </el-descriptions-item>
            </el-descriptions>
          </div>

          <!-- 一票否决 -->
          <div v-if="vetoes.length" class="card veto-card">
            <h3 class="card-title veto-title">🚫 一票否决项</h3>
            <el-alert
              v-for="(v, i) in vetoes"
              :key="i"
              type="error"
              :closable="false"
              show-icon
              class="veto-item"
              :title="`${v.item}（${v.score} 分）`"
              :description="v.reason"
            />
          </div>

          <!-- 四问得分 -->
          <div class="card">
            <h3 class="card-title">四问测评</h3>
            <div class="four-grid">
              <div
                v-for="(q, i) in fourQuestions"
                :key="i"
                class="four-item"
                :class="q.passed ? 'pass' : 'fail'"
              >
                <div class="four-score">{{ Math.round(q.score * 100) }}</div>
                <div class="four-name">{{ q.item.replace('（四问之一）', '') }}</div>
                <div class="four-verdict">{{ q.passed ? '通过' : '未通过' }}</div>
              </div>
            </div>
          </div>

          <!-- 强验证（F2P/P2P 确定性断言） -->
          <div v-if="verifyData.checks.length || verifyConfig" class="card">
            <h3 class="card-title">强验证 · F2P/P2P 确定性断言</h3>

            <!-- 契约配置预览 -->
            <div v-if="verifyConfig" class="verify-config">
              <div class="verify-col">
                <span class="verify-col-title">F2P 验收（fail_to_pass）</span>
                <div class="verify-tags">
                  <el-tag v-for="(c, i) in verifyConfig.fail_to_pass || []" :key="'f' + i" size="small" type="primary" class="tag-item">
                    {{ c.name }}
                  </el-tag>
                  <span v-if="!(verifyConfig.fail_to_pass || []).length" class="verify-none">未配置</span>
                </div>
              </div>
              <div class="verify-col">
                <span class="verify-col-title">P2P 防退化（pass_to_pass）</span>
                <div class="verify-tags">
                  <el-tag v-for="(c, i) in verifyConfig.pass_to_pass || []" :key="'p' + i" size="small" type="warning" class="tag-item">
                    {{ c.name }}
                  </el-tag>
                  <span v-if="!(verifyConfig.pass_to_pass || []).length" class="verify-none">未配置</span>
                </div>
              </div>
            </div>

            <!-- 执行结果聚合 -->
            <template v-if="verifyData.checks.length">
              <div class="verify-summary">
                <span class="verify-badge">
                  F2P
                  <b :class="verifyData.f2pPassed === verifyData.f2p.length && verifyData.f2p.length ? 'ok' : 'bad'">
                    {{ verifyData.f2pPassed }}/{{ verifyData.f2p.length }}
                  </b>
                </span>
                <span class="verify-badge">
                  P2P
                  <b :class="verifyData.p2pPassed === verifyData.p2p.length && verifyData.p2p.length ? 'ok' : 'bad'">
                    {{ verifyData.p2pPassed }}/{{ verifyData.p2p.length }}
                  </b>
                </span>
                <span class="verify-hint">按用例执行：F2P 看 completion 用例，P2P 看 robustness 用例</span>
              </div>
              <el-table :data="verifyData.checks" size="small" border class="verify-table">
                <el-table-column label="组" width="70">
                  <template #default="{ row }">
                    <el-tag size="small" :type="row.group === 'f2p' ? 'primary' : 'warning'">{{ row.group.toUpperCase() }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="name" label="断言" min-width="130" />
                <el-table-column label="结论" width="80">
                  <template #default="{ row }">
                    <el-tag :type="row.passed ? 'success' : 'danger'" size="small">{{ row.passed ? '通过' : '失败' }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="detail" label="详情" min-width="190" />
                <el-table-column prop="input" label="用例输入" min-width="150" show-overflow-tooltip />
              </el-table>
            </template>
            <el-empty v-else description="已配置断言但尚无执行结果" :image-size="50" />
          </div>

          <!-- 静态扫描 -->
          <div class="card">
            <h3 class="card-title">静态扫描</h3>
            <el-table :data="report.static_scans" size="small" border>
              <el-table-column prop="item" label="检测项" min-width="140" />
              <el-table-column label="结论" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.verdict === 'ok' ? 'success' : row.verdict === 'warn' ? 'warning' : 'danger'" size="small">
                    {{ row.verdict }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="detail" label="详情" min-width="240" />
            </el-table>
          </div>

          <!-- Agent 评测结果 -->
          <div class="card">
            <h3 class="card-title">评测 Agent 结果</h3>
            <el-table :data="report.results" size="small" border>
              <el-table-column prop="item" label="评测项" min-width="150" />
              <el-table-column label="得分" width="110">
                <template #default="{ row }">
                  <el-progress
                    :percentage="Math.round(row.score * 100)"
                    :status="row.passed ? 'success' : 'exception'"
                    :stroke-width="8"
                  />
                </template>
              </el-table-column>
              <el-table-column label="结论" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.passed ? 'success' : 'danger'" size="small">
                    {{ row.passed ? '通过' : '未通过' }}
                  </el-tag>
                  <el-tag v-if="row.needs_human_review" type="warning" size="small" class="tag-item">需复核</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="理由" min-width="240">
                <template #default="{ row }">
                  <span class="reason-text">{{ row.reason }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="confidence" label="置信度" width="90" />
            </el-table>
          </div>

          <!-- 沙箱执行日志 -->
          <div class="card">
            <h3 class="card-title">沙箱交互日志（{{ report.sandbox_runs.length }} 条）</h3>
            <el-collapse>
              <el-collapse-item v-for="(s, i) in report.sandbox_runs" :key="i" :name="i">
                <template #title>
                  <span class="log-input">{{ s.input }}</span>
                  <el-tag v-if="s.timeout" type="danger" size="small" class="tag-item">超时</el-tag>
                  <span class="log-duration">{{ s.duration_ms }}ms</span>
                </template>
                <pre class="log-output">{{ s.output || '（无文本产出，见产物文件）' }}</pre>
              </el-collapse-item>
            </el-collapse>
          </div>

          <!-- 人工复核 -->
          <div class="card">
            <h3 class="card-title">人工复核台（{{ reviewItems.length }} 条待复核）</h3>
            <el-empty v-if="!reviewItems.length" description="没有待复核的案例" :image-size="60" />

            <div v-for="(item, i) in reviewItems" :key="i" class="review-item">
              <div class="review-head">
                <el-tag size="small" type="warning">{{ item.item }}</el-tag>
                <span class="review-score">得分 {{ item.score }}</span>
                <el-tag v-if="item.review" size="small" :type="item.review.decision === 'approve' ? 'success' : 'danger'">
                  已复核：{{ item.review.decision }}
                </el-tag>
              </div>
              <p class="review-reason">{{ item.reason }}</p>
              <pre v-if="parseEvidence(item.evidence)" class="review-evidence">{{ parseEvidence(item.evidence) }}</pre>
              <el-button size="small" type="primary" plain @click="openReview(item)">人工复核</el-button>
            </div>

            <!-- 复核表单 -->
            <el-form v-if="reviewForm.result_id" label-position="top" class="review-form">
              <el-form-item label="复核结论">
                <el-radio-group v-model="reviewForm.decision">
                  <el-radio-button value="approve">确认通过</el-radio-button>
                  <el-radio-button value="reject">否决</el-radio-button>
                  <el-radio-button value="revise">修正后保留</el-radio-button>
                </el-radio-group>
              </el-form-item>
              <el-form-item label="备注（可选）">
                <el-input v-model="reviewForm.note" type="textarea" :rows="2" placeholder="你的判断依据，将作为标注数据反哺评测 Agent" />
              </el-form-item>
              <div class="review-actions">
                <el-button @click="reviewForm.result_id = null">取消</el-button>
                <el-button type="primary" @click="submitReview">提交复核结果</el-button>
              </div>
            </el-form>
          </div>
        </template>
      </template>
    </main>
  </div>
</template>

<style scoped>
.eval-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}

.eval-main {
  flex: 1;
  width: 100%;
  max-width: 960px;
  margin: 0 auto;
  padding: 32px 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.eval-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
  color: #303133;
}

.page-sub {
  font-size: 13px;
  color: #909399;
  margin-top: 4px;
}

.card {
  background: #fff;
  border-radius: 12px;
  padding: 20px 24px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 14px;
}

.card-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.stage-tag {
  font-size: 15px;
  font-weight: 600;
  color: #409eff;
}

.run-meta {
  font-size: 12px;
  color: #909399;
}

.run-summary {
  margin-top: 10px;
  font-size: 13px;
  color: #606266;
  background: #f5f7fa;
  border-radius: 6px;
  padding: 8px 12px;
  line-height: 1.7;
}

.pipe-steps {
  margin-top: 16px;
}

/* 四问 */
.four-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.four-item {
  text-align: center;
  border-radius: 10px;
  padding: 16px 8px;
  border: 1px solid #ebeef5;
}

.four-item.pass {
  background: #f0f9eb;
  border-color: #b3e19d;
}

.four-item.fail {
  background: #fef0f0;
  border-color: #fbc4c4;
}

.four-score {
  font-size: 30px;
  font-weight: 700;
  color: #303133;
}

.four-item.pass .four-score {
  color: #67c23a;
}

.four-item.fail .four-score {
  color: #f56c6c;
}

.four-name {
  font-size: 13px;
  color: #606266;
  margin-top: 4px;
}

.four-verdict {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.env-image {
  font-size: 12px;
  color: #409eff;
  background: #f0f7ff;
  padding: 2px 6px;
  border-radius: 4px;
}

/* 强验证 */
.verify-config {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 14px;
  background: #fafbfc;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 12px 14px;
}

.verify-col-title {
  display: block;
  font-size: 12px;
  color: #909399;
  margin-bottom: 6px;
}

.verify-none {
  font-size: 12px;
  color: #c0c4cc;
}

.verify-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.verify-badge {
  font-size: 13px;
  color: #606266;
  background: #f5f7fa;
  border-radius: 6px;
  padding: 4px 10px;
}

.verify-badge b {
  font-size: 15px;
}

.verify-badge b.ok {
  color: #67c23a;
}

.verify-badge b.bad {
  color: #f56c6c;
}

.verify-hint {
  font-size: 12px;
  color: #c0c4cc;
}

.verify-table {
  margin-top: 4px;
}

/* 一票否决 */
.veto-card {
  border: 1px solid #f56c6c;
  background: #fff8f8;
}

.veto-title {
  color: #f56c6c;
}

.veto-item {
  margin-bottom: 10px;
}

.veto-item:last-child {
  margin-bottom: 0;
}

/* 标签 / 文本 */
.tag-item {
  margin-right: 4px;
}

.reason-text {
  font-size: 12px;
  color: #606266;
  line-height: 1.6;
  display: block;
  max-height: 60px;
  overflow: hidden;
}

/* 沙箱日志 */
.log-input {
  flex: 1;
  font-size: 13px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-duration {
  font-size: 12px;
  color: #909399;
}

.log-output {
  background: #282c34;
  color: #abb2bf;
  padding: 12px;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.6;
  max-height: 300px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

/* 人工复核 */
.review-item {
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 12px 16px;
  margin-bottom: 12px;
  background: #fafbfc;
}

.review-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.review-score {
  font-size: 13px;
  color: #e6a23c;
  font-weight: 600;
}

.review-reason {
  font-size: 13px;
  color: #606266;
  margin: 8px 0;
  line-height: 1.7;
}

.review-evidence {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 12px;
  color: #909399;
  max-height: 140px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
  margin-bottom: 8px;
}

.review-form {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px dashed #e4e7ed;
}

.review-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

@media (max-width: 640px) {
  .four-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
