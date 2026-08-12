<!--
  发布前四问（PRD F6）：发布是门禁，不是按钮。
  四项里「边界停机」必须 100%——该停不停是安全问题，不接受折中。
  「可发现性」测的不是能力，是 description 写得对不对，未通过会列出没被召回的原话。
-->
<template>
  <div class="gate" v-loading="loading">
    <header class="head">
      <h2>发布前四问</h2>
      <p class="sub">
        四项全过，加上准入检查和蒸馏度达标，才允许进市场。任何一项没过，发布按钮不可用。
      </p>
      <el-alert
        v-if="gate && gate.origin === 'route_upload'"
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
        title="直接上传的 Skill 也须先过门禁"
        description="上传后先在这里跑四项测试。没过就去 Creator 补齐目标、流程、边界和关键判断，再回来重跑。线上失败、人工修正和用户反馈会进入下一版。"
      />
    </header>

    <div v-if="gate" class="grid">
      <div v-for="e in gate.evals" :key="e.eval_type" class="eval" :class="evalClass(e)">
        <div class="eval-head">
          <span class="eval-label">{{ e.label }}</span>
          <span class="eval-state">{{ stateText(e) }}</span>
        </div>
        <div class="eval-bar">
          <div
            class="eval-fill"
            :style="{ width: ((e.pass_rate || 0) * 100).toFixed(0) + '%' }"
          />
          <div class="eval-line" :style="{ left: ((e.threshold || 0) * 100).toFixed(0) + '%' }" />
        </div>
        <div class="eval-meta">
          <span v-if="e.ran">通过率 {{ ((e.pass_rate || 0) * 100).toFixed(0) }}%</span>
          <span v-else>尚未运行</span>
          <span>阈值 {{ ((e.threshold || 0) * 100).toFixed(0) }}%</span>
        </div>
        <div v-if="e.eval_type === 'boundary_stop'" class="hard">
          硬性 100%，不可下调
        </div>
        <!-- 每条失败用例的判定原因，直接回答"为什么没通过" -->
        <ul v-if="failedCases(e).length" class="case-reasons">
          <li v-for="(cs, i) in failedCases(e)" :key="i" class="case-item">
            <div class="case-input">输入：{{ cs.input }}</div>
            <div class="case-reason">判定：{{ cs.reason || '未通过' }}</div>
          </li>
        </ul>
        <el-button size="small" :loading="running === e.eval_type" @click="run(e.eval_type)">
          {{ e.ran ? '重跑' : '运行' }}
        </el-button>
      </div>
    </div>

    <!-- 可发现性未通过时的具体缺口 -->
    <section v-if="missed.length" class="card warn">
      <h3>这些原话没能召回到它</h3>
      <p class="sub">
        description 写得不像用户说话。把下面这些说法搬进 description，再重跑一次。
      </p>
      <ul class="missed">
        <li v-for="(m, i) in missed" :key="i">{{ m }}</li>
      </ul>
      <el-button size="small" @click="goEdit">回去改 description</el-button>
    </section>

    <!-- 准入检查与蒸馏度 -->
    <section v-if="gate" class="card">
      <h3>准入检查</h3>
      <p v-if="gate.admission && gate.admission.passed" class="ok-line">
        结构、依赖、权限、数据边界、适用范围都通过了
      </p>
      <ul v-else class="missed">
        <li v-for="(f, i) in (gate.admission && gate.admission.failures) || []" :key="i">{{ f }}</li>
      </ul>

      <h3 style="margin-top: 18px">蒸馏度</h3>
      <p v-if="gate.distillation">
        当前 {{ ((gate.distillation.score || 0) * 100).toFixed(0) }} 分，
        <span :class="gate.distillation.publishable ? 'ok-line' : 'bad-line'">
          {{ gate.distillation.publishable ? '已达发布线' : '未达发布线' }}
        </span>
      </p>
      <ul v-if="gate.distillation && gate.distillation.still_missing.length" class="missed">
        <li v-for="(m, i) in gate.distillation.still_missing" :key="i">{{ m }}</li>
      </ul>
    </section>

    <!-- 修复建议：逐条失败原因 → 反向指导如何改，可一键写回草稿 -->
    <section class="card">
      <h3>失败原因与修复建议</h3>
      <p class="sub">
        逐条反馈每一项为什么失败，并给出可直接写回 Creator 草稿的修改建议。
      </p>
      <el-button type="primary" plain :loading="fixLoading" @click="generateFix">
        生成修复建议
      </el-button>

      <template v-if="fix">
        <ul class="diagnosis" v-if="fix.diagnosis && fix.diagnosis.length">
          <li v-for="(d, i) in fix.diagnosis" :key="i" class="diag-item">
            <div class="diag-item-name">{{ d.item }}</div>
            <div class="diag-why">为什么：{{ d.why }}</div>
            <div class="diag-how">怎么改：{{ d.how }}</div>
          </li>
        </ul>
        <div class="diag-actions">
          <el-button
            v-if="hasFixPatch"
            type="success"
            :loading="applying"
            @click="applyFix"
          >
            一键应用建议到草稿
          </el-button>
          <el-button text @click="$router.push('/creator?v=' + (gate && gate.version_id))">
            去 Creator 手动改
          </el-button>
        </div>
      </template>
    </section>

    <section class="actions">
      <el-button type="primary" :loading="publishing" @click="doPublish">发布</el-button>
      <el-button @click="$router.push('/eval/' + skillId)">查看评测报告</el-button>
      <el-button text @click="$router.push('/creator?v=' + (gate && gate.version_id))">
        回到 Creator
      </el-button>
    </section>

    <!-- 被拒绝的原因逐条列出 -->
    <el-alert
      v-if="blocked.length"
      type="error"
      :closable="false"
      title="发布被拒绝"
      style="margin-top: 14px"
    >
      <ul class="missed">
        <li v-for="(b, i) in blocked" :key="i">{{ b }}</li>
      </ul>
    </el-alert>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getGateStatus, runEvals, publishSkill, getGateFixSuggestion, applyGateFix } from '../api/growth'

const route = useRoute()
const router = useRouter()
const skillId = Number(route.query.skill)

const loading = ref(true)
const gate = ref(null)
const running = ref('')
const publishing = ref(false)
const missed = ref([])
const blocked = ref([])
const fix = ref(null)
const fixLoading = ref(false)
const applying = ref(false)

function evalClass(e) {
  if (!e.ran) return 'idle'
  return e.passed ? 'pass' : 'fail'
}
function stateText(e) {
  if (!e.ran) return '未运行'
  return e.passed ? '通过' : '未通过'
}

/** 该类型测试中判定未通过的用例（detail 已由后端逐条带出判定原因） */
function failedCases(e) {
  if (!Array.isArray(e.detail)) return []
  return e.detail.filter((cs) => cs && cs.passed === false)
}

/** 建议里是否有可直接写回草稿的内容 */
const hasFixPatch = computed(() => {
  const d = fix.value && fix.value.draft
  if (!d) return false
  return !!(d.goal || (d.done_criteria && d.done_criteria.length) ||
    (d.gotchas && d.gotchas.length) || (d.boundary && (d.boundary.not_applicable || d.boundary.handoff_trigger)) ||
    (d.judgments && d.judgments.length))
})

async function load() {
  loading.value = true
  try {
    gate.value = await getGateStatus(skillId)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

/** 自动依次重跑：尚未评测、或已评测但没达到满分的项。
 *  用户修改完 skill 回来，没满分的会自动重跑，不需要手动逐个点。 */
async function autoRunPending() {
  const types = ['discoverability', 'completion', 'stability', 'boundary_stop']
  for (const t of types) {
    const item = (gate.value && gate.value.evals || []).find((e) => e.eval_type === t)
    if (item && (!item.ran || !item.passed)) {
      await run(t)
    }
  }
}

async function run(type) {
  running.value = type
  try {
    const res = await runEvals(skillId, type)
    const item = (res.data || [])[0]
    if (item && item.missed) missed.value = item.missed
    else if (type === 'discoverability') missed.value = []
    if (item && item.error) ElMessage.warning(item.error)
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    running.value = ''
  }
}

async function doPublish() {
  publishing.value = true
  blocked.value = []
  try {
    await publishSkill(skillId)
    ElMessage.success('已发布')
    router.push('/skill/' + skillId)
  } catch (e) {
    if (e.payload && e.payload.blocked) blocked.value = e.payload.blocked
    else ElMessage.error(e.message)
  } finally {
    publishing.value = false
  }
}

async function generateFix() {
  fixLoading.value = true
  try {
    const res = await getGateFixSuggestion(skillId)
    fix.value = res.suggestion
    if (!fix.value || !fix.value.diagnosis || !fix.value.diagnosis.length) {
      ElMessage.info('当前没有需要修复的失败项')
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    fixLoading.value = false
  }
}

async function applyFix() {
  if (!fix.value || !fix.value.draft) return
  applying.value = true
  try {
    await applyGateFix(skillId, fix.value.draft)
    ElMessage.success('建议已写入草稿，测试用例已重播')
    fix.value = null
    await load()
    // 改完立刻自动重跑没满分的项，直接看到修改有没有生效
    await autoRunPending()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    applying.value = false
  }
}

function goEdit() {
  if (gate.value && gate.value.version_id) {
    router.push({ path: '/creator', query: { v: gate.value.version_id } })
  }
}

onMounted(async () => {
  await load()
  await autoRunPending()
})
</script>

<style scoped>
.gate {
  max-width: 900px;
  margin: 0 auto;
  padding: 24px 20px 60px;
}
.head h2 {
  margin: 0 0 6px;
}
.sub {
  color: #909399;
  font-size: 13px;
  line-height: 1.7;
  margin: 0 0 18px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}
.eval {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 14px;
}
.eval.pass {
  border-color: #67c23a;
}
.eval.fail {
  border-color: #f56c6c;
}
.eval-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 10px;
}
.eval-label {
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
}
.eval-state {
  font-size: 12px;
  color: #909399;
  white-space: nowrap;
}
.eval.pass .eval-state {
  color: #67c23a;
}
.eval.fail .eval-state {
  color: #f56c6c;
}
.eval-bar {
  position: relative;
  height: 6px;
  background: #f0f0f0;
  border-radius: 3px;
  overflow: visible;
}
.eval-fill {
  height: 6px;
  background: #409eff;
  border-radius: 3px;
}
.eval.pass .eval-fill {
  background: #67c23a;
}
.eval.fail .eval-fill {
  background: #f56c6c;
}
.eval-line {
  position: absolute;
  top: -3px;
  width: 2px;
  height: 12px;
  background: #303133;
}
.eval-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #909399;
  margin: 8px 0 10px;
}
.hard {
  font-size: 11px;
  color: #f56c6c;
  margin-bottom: 8px;
}
.case-reasons {
  margin: 0 0 10px;
  padding: 0;
  list-style: none;
}
.case-item {
  background: #fef0f0;
  border: 1px solid #fde2e2;
  border-radius: 6px;
  padding: 8px 10px;
  margin-bottom: 6px;
  font-size: 12px;
  line-height: 1.6;
}
.case-input {
  color: #606266;
  font-weight: 600;
}
.case-reason {
  color: #f56c6c;
}
.diagnosis {
  margin: 14px 0;
  padding: 0;
  list-style: none;
}
.diag-item {
  background: #f8f9fb;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 8px;
  font-size: 13px;
  line-height: 1.7;
}
.diag-item-name {
  font-weight: 600;
  margin-bottom: 4px;
}
.diag-why {
  color: #f56c6c;
}
.diag-how {
  color: #409eff;
}
.diag-actions {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-top: 4px;
}
.card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 18px;
  margin-bottom: 16px;
}
.card h3 {
  margin: 0 0 6px;
  font-size: 15px;
}
.card.warn {
  border-color: #e6a23c;
}
.missed {
  margin: 8px 0 14px;
  padding-left: 20px;
  font-size: 13px;
  line-height: 1.9;
  color: #606266;
}
.ok-line {
  color: #67c23a;
}
.bad-line {
  color: #e6a23c;
}
.actions {
  display: flex;
  gap: 10px;
  align-items: center;
}
</style>
