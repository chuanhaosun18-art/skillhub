<!--
  我要成长（PRD F1 + F8）：前台卖「下一步」，后台跑 Skill。
  用户说一句人话 → 识别任务 → 给出下一步 → 路由能力，并解释为什么是它、为什么没选另一个。
  伪需求（情绪、人生抉择、名额竞争、实时信息、资源依赖）在这里被拦住，不进任务流。
-->
<template>
  <div class="grow">
    <section class="hero">
      <h1>你现在卡在哪？</h1>
      <p class="hero-sub">
        用你自己的话说。不用先想清楚要什么能力——那是我的事。
      </p>
      <el-input
        v-model="utterance"
        type="textarea"
        :rows="3"
        placeholder="例如：我的选题被导师退了两次，说范围太大"
        @keydown.ctrl.enter="submit"
      />
      <div class="hero-actions">
        <el-button type="primary" size="large" :loading="loading" @click="submit">
          帮我找下一步
        </el-button>
        <span class="tip">Ctrl + Enter</span>
      </div>
      <div class="examples">
        <span
          v-for="(ex, i) in examples"
          :key="i"
          class="ex"
          @click="utterance = ex"
        >{{ ex }}</span>
      </div>
    </section>

    <!-- 编排态：长周期方向性需求的出口。不承诺结果，但给编排 -->
    <section v-if="res && res.mode === 'orchestration'" class="card orch">
      <h3>这件事我不做成一次任务，我给你排接下来几周</h3>
      <p class="orch-body">{{ res.message }}</p>
      <p class="orch-label">方向：{{ res.label }}</p>
      <el-button type="primary" @click="goOrchestration(res.orchestration_intent)">
        看看有没有人走过这条路
      </el-button>
    </section>

    <!-- 拒绝：情绪类与「该不该」。不创建任务、不落 Experience -->
    <section v-else-if="res && res.mode === 'rejected'" class="card reject">
      <h3>这件事我不做成 Skill</h3>
      <p class="reject-body">{{ res.response }}</p>
      <p class="reject-why">{{ res.reason }}</p>
      <div class="reject-actions">
        <el-button
          v-for="(r, i) in res.resources || []"
          :key="i"
          size="small"
          @click="handleResource(r)"
        >{{ r.label }}</el-button>
        <el-button size="small" type="primary" plain @click="goForum">
          去许愿池挂一个
        </el-button>
      </div>

      <!-- 「该不该」型问题：不给建议，只给别人走过的分支与人数 -->
      <div v-if="(res.branches || []).length" class="branches">
        <div class="branches-title">别人走过的路，以及各自的去向</div>
        <div v-for="(b, i) in res.branches" :key="i" class="branch-item">
          <div class="branch-goal">{{ b.goal_label }}（{{ b.walked_count }} 人走过）</div>
          <div v-if="b.branch_summary && b.branch_summary.branches" class="branch-list">
            <span v-for="(x, j) in b.branch_summary.branches" :key="j" class="branch-chip">
              {{ x.count }} 人{{ x.label }}
            </span>
          </div>
          <div v-if="b.provenance_note" class="branch-prov">{{ b.provenance_note }}</div>
        </div>
        <div class="branches-note">这是别人的路，不是建议。选择是你的事。</div>
      </div>
    </section>

    <!-- 四筛未过 -->
    <section v-else-if="res && res.mode === 'not_skillable'" class="card warn">
      <h3>这类问题不适合用 Skill 解决</h3>
      <p>{{ res.message }}</p>
      <div class="sieve">
        <span v-for="(v, k) in res.sieve" :key="k" v-show="typeof v === 'boolean'"
          class="sv" :class="v ? 'ok' : 'no'">
          {{ sieveLabel(k) }}{{ v ? ' 通过' : ' 未通过' }}
        </span>
      </div>
      <div class="reject-actions" style="margin-top: 14px">
        <el-button size="small" type="primary" plain @click="goForum">
          去许愿池挂一个
        </el-button>
      </div>
    </section>

    <!-- 澄清：只问一轮 -->
    <section v-else-if="res && res.mode === 'clarify'" class="card">
      <h3>再确认一件事</h3>
      <p>{{ res.clarify_question }}</p>
      <el-input v-model="utterance" type="textarea" :rows="2" style="margin-top: 10px" />
      <el-button type="primary" size="small" style="margin-top: 10px" @click="submit">
        继续
      </el-button>
    </section>

    <!-- 兜底：手选任务 -->
    <section v-else-if="res && res.mode === 'manual_fallback'" class="card">
      <h3>{{ res.message }}</h3>
      <div class="opts">
        <el-tag
          v-for="o in res.options || []"
          :key="o.task_intent"
          class="opt"
          @click="startTask(o.task_intent)"
        >{{ o.label }}</el-tag>
      </div>
    </section>

    <!-- 任务卡 + 路由结果 -->
    <template v-else-if="res && res.mode === 'task'">
      <section class="card task">
        <div class="task-label">{{ res.task_card.task_label }}</div>
        <h3>你现在在这里</h3>
        <p class="pos">{{ res.task_card.current_position }}</p>
        <h3>还差什么</h3>
        <ul class="gap">
          <li v-for="(g, i) in res.task_card.gap" :key="i">{{ g }}</li>
        </ul>
        <div class="next">
          <div class="next-label">今天的下一步</div>
          <div class="next-text">{{ res.task_card.next_step }}</div>
        </div>
        <el-button type="primary" @click="startTask(res.task_card.task_intent)">
          在工作台做这件事
        </el-button>
      </section>

      <section v-if="routed" class="card">
        <h3>可以用的能力</h3>
        <p class="sub" v-if="routed.filtered_out_count">
          有 {{ routed.filtered_out_count }} 个没进候选集：没通过准入，或与这件事无关。
        </p>

        <div v-if="routed.empty_reason" class="empty-reason">
          <p>{{ routed.empty_reason }}</p>
          <el-button size="small" type="primary" @click="startTask(res.task_card.task_intent)">
            裸做一次
          </el-button>
        </div>

        <div v-for="(r, i) in routed.results || []" :key="r.skill_id" class="rec">
          <div class="rec-head">
            <span class="rec-rank">{{ i + 1 }}</span>
            <router-link :to="'/trust/' + r.skill_id" class="rec-name">{{ r.name }}</router-link>
            <el-tag size="small" type="info">v{{ r.version }}</el-tag>
          </div>
          <div class="rec-why">{{ r.why_this }}</div>
          <!-- choose_if：不给分数，但给一个你自己能判断的条件 -->
          <div v-if="r.choose_if" class="rec-choose">{{ r.choose_if }}</div>
          <div v-if="r.why_not_alternative" class="rec-why-not">
            {{ r.why_not_alternative }}
          </div>
          <div class="rec-meta">
            <span v-if="r.evidence && !r.evidence.sample_sufficient" class="warn-tag">
              线上样本不足
            </span>
            <span v-if="r.evidence">{{ r.evidence.traceable_decisions }} 条判断可溯源</span>
            <span v-if="r.risk && r.risk.irreversible" class="warn-tag">含不可逆操作</span>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { interpretGoal, routeSkills } from '../api/growth'

const router = useRouter()
const utterance = ref('')
const loading = ref(false)
const res = ref(null)
const routed = ref(null)

const examples = [
  '我的选题被导师退了两次，说范围太大',
  '科研经历怎么写进产品岗的简历',
  '我最近很焦虑，不知道该干什么',
  '我该不该考研',
]

function sieveLabel(k) {
  const m = {
    amortizable: '可摊销',
    testable: '可测试',
    transferable: '可转移',
    short_loop: '短链路',
  }
  return m[k] || k
}

async function submit() {
  if (!utterance.value.trim()) return ElMessage.warning('先说一句你现在的情况')
  loading.value = true
  routed.value = null
  try {
    const r = await interpretGoal(utterance.value)
    res.value = r
    if (r.mode === 'task') {
      // 识别成功后立刻路由能力，并要求解释
      try {
        routed.value = await routeSkills(utterance.value, r.task_card.task_intent)
      } catch (e) {
        routed.value = null
      }
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function startTask(intent) {
  router.push({ path: '/workbench', query: { intent, goal: utterance.value } })
}

function handleResource(r) {
  if (r.action === 'goto_home') router.push('/')
  else if (r.action === 'goto_graph') router.push('/orchestration')
}

function goOrchestration(intent) {
  router.push({ path: '/orchestration', query: { intent, goal: utterance.value } })
}

// 「没有 Skill 能解」→ 论坛：带原话过去，直接发一帖问有没有人遇到过
function goForum() {
  const kw = utterance.value.trim()
  router.push({ path: '/forum', query: kw ? { q: kw, ask: '1' } : { ask: '1' } })
}
</script>

<style scoped>
.grow {
  max-width: 780px;
  margin: 0 auto;
  padding: 40px 20px 60px;
}
.hero h1 {
  font-size: 30px;
  margin: 0 0 8px;
}
.hero-sub {
  color: #606266;
  margin: 0 0 18px;
}
.hero-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 12px;
}
.tip {
  font-size: 12px;
  color: #c0c4cc;
}
.examples {
  margin-top: 14px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.ex {
  font-size: 12px;
  color: #409eff;
  background: #ecf5ff;
  border-radius: 12px;
  padding: 4px 10px;
  cursor: pointer;
}
.card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 20px;
  margin-top: 18px;
}
.card h3 {
  margin: 0 0 8px;
  font-size: 15px;
}
.card h3:not(:first-child) {
  margin-top: 16px;
}
.sub {
  font-size: 12px;
  color: #909399;
  margin: 0 0 12px;
}
/* 编排态卡：语气要笃定——不承诺结果不等于帮不上 */
.orch {
  border-color: #409eff;
  background: #f7fbff;
}
.orch-body {
  font-size: 15px;
  line-height: 1.9;
  color: #303133;
  margin: 0 0 8px;
}
.orch-label {
  font-size: 12px;
  color: #909399;
  margin: 0 0 14px;
}
/* 「该不该」型问题的分支展示：只给人数，不给比率，不给建议 */
.branches {
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid #ebeef5;
}
.branches-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 10px;
}
.branch-item {
  margin-bottom: 12px;
}
.branch-goal {
  font-size: 13px;
  color: #303133;
}
.branch-list {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 6px;
}
.branch-chip {
  font-size: 12px;
  background: #f4f4f5;
  border-radius: 10px;
  padding: 2px 10px;
  color: #606266;
}
.branch-prov {
  font-size: 11px;
  color: #e6a23c;
  margin-top: 4px;
}
.branches-note {
  font-size: 12px;
  color: #909399;
  margin-top: 10px;
}
.rec-choose {
  font-size: 13px;
  color: #409eff;
  margin-top: 6px;
  line-height: 1.7;
}
/* 拒绝卡：语气要稳，不用列表，不给方法 */
.reject {
  border-color: #909399;
  background: #fafafa;
}
.reject-body {
  line-height: 1.9;
  color: #303133;
  margin: 0 0 10px;
}
.reject-why {
  font-size: 12px;
  color: #909399;
  line-height: 1.7;
}
.reject-actions {
  margin-top: 12px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.warn {
  border-color: #e6a23c;
}
.sieve {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 10px;
}
.sv {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 10px;
  border: 1px solid #dcdfe6;
}
.sv.ok {
  color: #67c23a;
  border-color: #67c23a;
}
.sv.no {
  color: #f56c6c;
  border-color: #f56c6c;
}
.opts,
.rec-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.opt {
  cursor: pointer;
}
.task-label {
  font-size: 12px;
  color: #909399;
  margin-bottom: 10px;
}
.pos {
  margin: 0;
  line-height: 1.8;
}
.gap {
  margin: 0;
  padding-left: 20px;
  line-height: 1.9;
  color: #606266;
}
.next {
  background: #ecf5ff;
  border-radius: 8px;
  padding: 14px;
  margin: 16px 0;
}
.next-label {
  font-size: 12px;
  color: #409eff;
  margin-bottom: 4px;
}
.next-text {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.6;
}
.rec {
  border-top: 1px solid #f0f0f0;
  padding: 14px 0;
}
.rec-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.rec-rank {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #409eff;
  color: #fff;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.rec-name {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  text-decoration: none;
}
.rec-name:hover {
  color: #409eff;
}
.rec-why {
  font-size: 13px;
  color: #606266;
  margin-top: 6px;
  line-height: 1.7;
}
/* 「为什么没选另一个」是平台辨别力的可见形式，视觉上要留住 */
.rec-why-not {
  font-size: 12px;
  color: #909399;
  margin-top: 6px;
  padding: 8px 10px;
  background: #fafafa;
  border-left: 2px solid #dcdfe6;
  line-height: 1.7;
}
.rec-meta {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
}
.warn-tag {
  color: #e6a23c;
}
.empty-reason {
  background: #fdf6ec;
  border-radius: 8px;
  padding: 14px;
  line-height: 1.8;
}
</style>
