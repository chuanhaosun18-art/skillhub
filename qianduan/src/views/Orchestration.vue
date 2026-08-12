<!--
  编排态（PRD F17）：长周期方向性需求的出口。
  交付的不是产物，是一份带时间的编排——而且必须来自别人真走过的路。

  三条硬约束在界面上的体现：
  1. 没有来源 Path 就不生成，直接告诉用户"我不会凭空给你排"。
  2. 全页不出现任何成功率/通过率，只给绝对人数与真实分叉（含"停下"那一支）。
  3. 不可控项独立成组，不混在待办里。
-->
<template>
  <div class="page">
    <AppNavbar />
    <main class="main">
      <!-- 阶段 0：选方向 + 前置检查 -->
      <section v-if="stage === 'probe'" class="card">
        <h1>接下来几周该做什么</h1>
        <p class="sub">
          这类事我不承诺结果——名额和别人的表现不由你的准备决定。
          但接下来每周该做什么可以排清楚，用别人真走过的路来排。
        </p>
        <el-select v-model="orchIntent" placeholder="选一个方向" style="width: 260px">
          <el-option v-for="o in ORCHESTRATION_INTENTS" :key="o.value" :label="o.label" :value="o.value" />
        </el-select>
        <el-input
          v-model="utterance"
          type="textarea"
          :rows="2"
          placeholder="用你自己的话说一下现在的情况（选填）"
          style="margin-top: 12px"
        />
        <el-button type="primary" :loading="loading" style="margin-top: 12px" @click="doProbe">
          看看有没有人走过
        </el-button>
      </section>

      <!-- 没人走过：这一幕比生成成功更能说明平台的判断力 -->
      <section v-if="stage === 'unavailable'" class="card empty-card">
        <h2>这条路还没有人在这里走完过</h2>
        <p class="empty-msg">{{ probeResult.message }}</p>
        <div class="empty-actions">
          <el-button v-for="(o, i) in probeResult.options || []" :key="i" @click="handleOption(o)">
            {{ o.label }}
          </el-button>
        </div>
        <p class="empty-note">
          你刚说的话已经记下来了。等有人走完这条路，我们会回来找你。
        </p>
      </section>

      <!-- 阶段 1：上下文访谈 -->
      <section v-if="stage === 'interview'" class="card">
        <div class="src-badge">
          {{ probeResult.walked_total }} 人走过这条路
          <span v-if="probeResult.provenance_note" class="prov">{{ probeResult.provenance_note }}</span>
        </div>
        <h2>先问你几件事</h2>
        <p class="sub">
          编排不适配你的情况就是废纸。这几个问题问完就开始排，最多 5 轮。
        </p>

        <div class="collected">
          <el-tag
            v-for="f in fieldList"
            :key="f.key"
            :type="collected[f.key] ? 'success' : 'info'"
            effect="plain"
          >
            {{ f.label }}{{ collected[f.key] ? ' ✓' : '' }}
          </el-tag>
        </div>

        <div v-for="(q, i) in questions" :key="i" class="q">
          <div class="q-text">{{ q }}</div>
        </div>
        <el-input
          v-model="answer"
          type="textarea"
          :rows="3"
          placeholder="一起回答就行，不用分开写"
        />
        <div class="row">
          <el-button type="primary" :loading="loading" @click="doInterview">继续</el-button>
          <el-button v-if="readyToGenerate" type="success" :loading="loading" @click="doGenerate">
            开始排
          </el-button>
          <span v-if="missing.length" class="miss">还差：{{ missing.map(fieldLabel).join('、') }}</span>
        </div>
      </section>

      <!-- 阶段 2：编排 -->
      <template v-if="stage === 'plan' && orch">
        <section class="card plan-head">
          <div>
            <div class="plan-label">{{ orch.label }}</div>
            <h2>{{ orch.goal_label || '我的编排' }}</h2>
            <div class="plan-meta">
              共 {{ orch.horizon_weeks }} 周 ·
              <el-tag size="small" :type="statusType(orch.status)">{{ statusText(orch.status) }}</el-tag>
            </div>
          </div>
          <el-button
            v-if="orch.status === 'drafting'"
            type="primary"
            :loading="loading"
            @click="doAdopt"
          >
            就按这个来
          </el-button>
        </section>

        <!-- 不承诺结果：这句话必须在最显眼的位置 -->
        <el-alert type="info" :closable="false" style="margin-bottom: 16px">
          {{ orch.promise_note }}
        </el-alert>

        <!-- 真实分叉，含「停下」那一支。只给人数不给比率 -->
        <section v-if="orch.branch_summary" class="card branch">
          <h3>走过这条路的人后来怎么样了</h3>
            <p class="branch-text">{{ orch.branch_summary }}</p>
          <div v-for="s in orch.source_paths || []" :key="s.path_id" class="prov-line">
            <el-tag size="small" :type="s.provenance === 'observed' ? 'success' : 'warning'">
              {{ s.provenance === 'observed' ? '平台内观测' : '回忆整理' }}
            </el-tag>
            <span v-if="s.provenance_note">{{ s.provenance_note }}</span>
          </div>
        </section>

        <!-- 按周的待办 -->
        <section v-for="w in orch.weeks || []" :key="w.week_index" class="card week">
          <div class="week-head">
            <h3>第 {{ w.week_index }} 周</h3>
            <el-button
              v-if="orch.status === 'active'"
              size="small"
              text
              @click="doReview(w.week_index)"
            >
              复核这一周
            </el-button>
          </div>
          <div v-for="it in w.items" :key="it.id" class="item">
            <el-checkbox
              :model-value="it.status === 'done'"
              :disabled="orch.status === 'drafting'"
              @change="(v) => toggleItem(it, v)"
            />
            <div class="item-body">
              <div class="item-title" :class="{ done: it.status === 'done' }">{{ it.title }}</div>
              <div class="item-why">{{ it.why_now }}</div>
              <div class="item-meta">
                <span v-if="it.due_date" :class="{ overdue: it.status === 'expired' }">
                  截止 {{ it.due_date }}
                </span>
                <el-tag v-if="it.status === 'expired'" size="small" type="danger">已过期</el-tag>
                <el-button
                  v-if="it.linked_task_intent"
                  link
                  type="primary"
                  size="small"
                  @click="goTask(it)"
                >
                  在工作台做这件事
                </el-button>
              </div>
            </div>
          </div>
        </section>

        <!-- 不可控项独立成组：绝不混进待办 -->
        <section v-if="(orch.uncontrollable || []).length" class="card uncontrollable">
          <h3>这些不由你决定</h3>
          <p class="sub">
            它们会影响结果，但不由你的准备决定。放在这里是为了让你知道什么时候会发生，
            而不是让你去努力。
          </p>
          <div v-for="u in orch.uncontrollable" :key="u.id" class="unc-item">
            <div class="unc-title">{{ u.title }}</div>
            <div v-if="u.why_now" class="unc-note">{{ u.why_now }}</div>
          </div>
        </section>

        <!-- 复核历史 -->
        <section v-if="(orch.reviews || []).length" class="card">
          <h3>周复核记录</h3>
          <p class="sub">这是唯一能说明这份编排有没有用的证据——你有没有回来。</p>
          <div v-for="r in orch.reviews" :key="r.week_index + '-' + r.reviewed_at" class="rev">
            第 {{ r.week_index }} 周：完成 {{ r.done_count }}/{{ r.total_count }}
            <span v-if="r.expired_count" class="overdue">，过期 {{ r.expired_count }}</span>
          </div>
        </section>
      </template>

      <!-- 生成失败的兜底：给原始路径，不给半成品编排 -->
      <section v-if="stage === 'raw' && rawPaths.length" class="card">
        <h2>这是别人走过的原始顺序</h2>
        <p class="sub">{{ rawMessage }}</p>
        <div v-for="p in rawPaths" :key="p.id" class="raw-path">
          <div class="raw-head">{{ p.goal_label }}（{{ p.walked_count }} 人走过）</div>
          <ol>
            <li v-for="n in p.nodes" :key="n.id">
              {{ n.label }}
              <el-tag v-if="!n.controllable" size="small" type="warning">不可控</el-tag>
            </li>
          </ol>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import AppNavbar from '../components/AppNavbar.vue'
import {
  ORCHESTRATION_INTENTS,
  probeOrchestration,
  interviewOrchestration,
  createOrchestration,
  getOrchestration,
  adoptOrchestration,
  updateOrchItem,
  reviewOrchestration,
} from '../api/growth'

const route = useRoute()
const router = useRouter()

const stage = ref('probe') // probe | unavailable | interview | plan | raw
const loading = ref(false)
const orchIntent = ref(route.query.intent || 'postgrad_recommend')
const utterance = ref(route.query.goal || '')
const probeResult = ref({})
const orch = ref(null)
const rawPaths = ref([])
const rawMessage = ref('')

const round = ref(0)
const questions = ref([])
const answer = ref('')
const collected = reactive({})
const missing = ref(['target', 'current_progress', 'weekly_hours', 'hard_constraints'])

const fieldList = [
  { key: 'target', label: '具体目标' },
  { key: 'current_progress', label: '当前进度' },
  { key: 'weekly_hours', label: '每周投入' },
  { key: 'hard_constraints', label: '硬约束' },
]

const readyToGenerate = computed(() => missing.value.length === 0)

function fieldLabel(k) {
  return (fieldList.find((f) => f.key === k) || {}).label || k
}

function statusText(s) {
  const m = { drafting: '草稿', active: '进行中', paused: '已暂停', completed: '已完成', abandoned: '已放弃' }
  return m[s] || s
}
function statusType(s) {
  const m = { drafting: 'info', active: 'primary', paused: 'warning', completed: 'success' }
  return m[s] || 'info'
}

async function doProbe() {
  loading.value = true
  try {
    const res = await probeOrchestration(utterance.value, orchIntent.value)
    probeResult.value = res
    if (!res.available) {
      stage.value = 'unavailable'
      return
    }
    stage.value = 'interview'
    await doInterview(true)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function doInterview(first = false) {
  loading.value = true
  try {
    const res = await interviewOrchestration({
      orchestration_intent: orchIntent.value,
      round: round.value,
      answer: first ? utterance.value : answer.value,
      collected: { ...collected },
    })
    questions.value = res.questions || []
    missing.value = res.missing || []
    Object.keys(res.collected || {}).forEach((k) => {
      collected[k] = res.collected[k]
    })
    round.value = res.round || round.value + 1
    answer.value = ''
    if (res.degraded) ElMessage.info('自动追问不可用，用固定问题继续')
    if (res.round_limit_hit) ElMessage.info(res.message)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function doGenerate() {
  loading.value = true
  try {
    const res = await createOrchestration({
      orchestration_intent: orchIntent.value,
      goal_label: collected.target || utterance.value,
      context: { ...collected },
    })
    if (res.mode === 'raw_path') {
      rawPaths.value = res.paths || []
      rawMessage.value = res.message
      stage.value = 'raw'
      return
    }
    orch.value = res
    stage.value = 'plan'
    if (res.extract_stats) ElMessage.info(res.extract_stats.note)
  } catch (e) {
    if (e.payload && e.payload.missing) {
      missing.value = e.payload.missing
      ElMessage.warning('还差一点上下文')
    } else {
      ElMessage.error(e.message)
    }
  } finally {
    loading.value = false
  }
}

async function doAdopt() {
  loading.value = true
  try {
    orch.value = await adoptOrchestration(orch.value.id)
    ElMessage.success('已采纳。每周回来勾一次就行。')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function toggleItem(item, checked) {
  try {
    orch.value = await updateOrchItem(orch.value.id, item.id, {
      status: checked ? 'done' : 'todo',
    })
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function doReview(weekIndex) {
  let note = ''
  try {
    const r = await ElMessageBox.prompt('这周有什么想记下来的？（选填）', `复核第 ${weekIndex} 周`, {
      confirmButtonText: '提交复核',
      cancelButtonText: '取消',
      inputPlaceholder: '比如：绩点排名还没出，第 5 周之后要重排',
    })
    note = r.value || ''
  } catch (e) {
    return
  }
  try {
    const res = await reviewOrchestration(orch.value.id, { week_index: weekIndex, note })
    ElMessage.success(res.encourage)
    orch.value = await getOrchestration(orch.value.id)
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function goTask(item) {
  router.push({
    path: '/workbench',
    query: { intent: item.linked_task_intent, goal: item.title },
  })
}

function handleOption(o) {
  if (o.action === 'create_execution') router.push('/grow')
  else if (o.action === 'browse_paths') router.push('/')
}

onMounted(async () => {
  if (route.query.id) {
    loading.value = true
    try {
      orch.value = await getOrchestration(route.query.id)
      stage.value = 'plan'
    } catch (e) {
      ElMessage.error(e.message)
    } finally {
      loading.value = false
    }
  }
})
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f7fa;
}
.main {
  max-width: 820px;
  margin: 0 auto;
  padding: 28px 20px 60px;
}
.card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 22px;
  margin-bottom: 16px;
}
h1 {
  margin: 0 0 8px;
  font-size: 26px;
}
h2 {
  margin: 0 0 8px;
  font-size: 20px;
}
h3 {
  margin: 0 0 8px;
  font-size: 16px;
}
.sub {
  color: #909399;
  font-size: 13px;
  line-height: 1.8;
  margin: 0 0 16px;
}
/* 没人走过：这一幕要显得笃定，不像是出错 */
.empty-card {
  border-color: #e6a23c;
}
.empty-msg {
  font-size: 15px;
  line-height: 1.9;
  color: #303133;
}
.empty-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin: 16px 0 12px;
}
.empty-note {
  font-size: 12px;
  color: #909399;
  margin: 0;
}
.src-badge {
  font-size: 12px;
  color: #67c23a;
  margin-bottom: 10px;
}
.prov {
  color: #e6a23c;
  margin-left: 8px;
}
.collected {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.q {
  margin-bottom: 10px;
}
.q-text {
  font-size: 15px;
  font-weight: 600;
  line-height: 1.7;
}
.row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 12px;
  flex-wrap: wrap;
}
.miss {
  font-size: 12px;
  color: #e6a23c;
}
.plan-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}
.plan-label {
  font-size: 12px;
  color: #909399;
}
.plan-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.branch-text {
  font-size: 14px;
  line-height: 1.9;
  color: #303133;
  margin: 0 0 10px;
}
.prov-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #909399;
  margin-top: 6px;
}
.week-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.item {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 10px 0;
  border-top: 1px solid #f5f5f5;
}
.item-title {
  font-size: 14px;
  line-height: 1.6;
}
.item-title.done {
  text-decoration: line-through;
  color: #c0c4cc;
}
.item-why {
  font-size: 12px;
  color: #909399;
  line-height: 1.7;
  margin-top: 3px;
}
.item-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
.overdue {
  color: #f56c6c;
}
/* 不可控项：视觉上明确不是待办，不给勾选框 */
.uncontrollable {
  background: #fafafa;
  border-style: dashed;
}
.unc-item {
  padding: 10px 0;
  border-top: 1px solid #f0f0f0;
}
.unc-title {
  font-size: 14px;
  color: #606266;
}
.unc-note {
  font-size: 12px;
  color: #909399;
  margin-top: 3px;
  line-height: 1.7;
}
.rev {
  font-size: 13px;
  color: #606266;
  padding: 6px 0;
}
.raw-path ol {
  padding-left: 20px;
  line-height: 2;
  font-size: 14px;
  color: #606266;
}
.raw-head {
  font-size: 14px;
  font-weight: 600;
  margin-top: 10px;
}
</style>
