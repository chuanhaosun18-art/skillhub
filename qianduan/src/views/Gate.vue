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

    <section class="actions">
      <el-button type="primary" :loading="publishing" @click="doPublish">发布</el-button>
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
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getGateStatus, runEvals, publishSkill } from '../api/growth'

const route = useRoute()
const router = useRouter()
const skillId = Number(route.query.skill)

const loading = ref(true)
const gate = ref(null)
const running = ref('')
const publishing = ref(false)
const missed = ref([])
const blocked = ref([])

function evalClass(e) {
  if (!e.ran) return 'idle'
  return e.passed ? 'pass' : 'fail'
}
function stateText(e) {
  if (!e.ran) return '未运行'
  return e.passed ? '通过' : '未通过'
}

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

function goEdit() {
  if (gate.value && gate.value.version_id) {
    router.push({ path: '/creator', query: { v: gate.value.version_id } })
  }
}

onMounted(load)
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
