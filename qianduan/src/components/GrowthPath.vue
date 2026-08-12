<!--
  成长路径 / 成长身份（PRD F13）
  这不是一张成就墙，是一条真实走过的路：每个节点都对应一次执行，点开能回到那次任务。
  硬性约束：不出现粉丝数、不出现成长分数、不与他人比名次；默认全部不公开，逐项开启。
-->
<template>
  <div class="gp" v-loading="loading">
    <template v-if="p">
      <!-- 当前位置 -->
      <div class="pos">
        <div class="pos-left">
          <div class="pos-label">当前位置</div>
          <div class="pos-main">
            <span v-if="p.current_position.recent_task">
              正在走：{{ p.current_position.recent_task }}
            </span>
            <span v-else class="pos-empty">{{ p.current_position.empty_hint }}</span>
          </div>
          <div class="pos-sub">
            {{ [p.current_position.school, p.current_position.major, p.current_position.grade]
                .filter(Boolean).join(' · ') || '资料未填' }}
          </div>
        </div>
        <el-button
          v-if="!p.current_position.recent_task"
          type="primary"
          @click="$router.push('/grow')"
        >
          走出第一步
        </el-button>
      </div>

      <p class="principle">{{ p.principle }}</p>

      <!-- 成长状态四阶 -->
      <section v-if="p.growth_states" class="block">
        <h4>
          成长状态
          <span class="h-note">学过 → 做过 → 做成过 → 教会过。最高一阶不是分数最高，是方法能帮到别人。</span>
        </h4>
        <div v-if="!p.growth_states.length" class="empty">还没有任何方向的记录</div>
        <div v-else class="states">
          <div v-for="s in sortedStates" :key="s.task_intent" class="st">
            <div class="st-task">{{ s.task_label }}</div>
            <div class="st-track">
              <span
                v-for="(lb, i) in ['学过', '做过', '做成过', '教会过']"
                :key="i"
                class="st-dot"
                :class="{ on: s.rank >= i + 1 }"
              >{{ lb }}</span>
            </div>
            <div class="st-detail" v-if="s.detail">
              做过 {{ s.detail.executions }} 次，完成 {{ s.detail.completed }} 次
              <template v-if="s.detail.helped_others">
                · 帮到 {{ s.detail.helped_others }} 人
              </template>
            </div>
          </div>
        </div>
      </section>
      <div v-else-if="p.states_hidden" class="hidden-tip">成长状态未公开</div>

      <!-- 成长路线时间线 -->
      <section v-if="p.timeline" class="block">
        <h4>
          成长路线
          <span class="h-note">每个节点都是一次真实执行，不是自填的经历</span>
        </h4>
        <div v-if="!p.timeline.length" class="empty">路线还是空的</div>
        <ol v-else class="tl">
          <li v-for="n in p.timeline" :key="n.execution_id" :class="'s-' + n.status">
            <div class="tl-dot"></div>
            <div class="tl-body">
              <div class="tl-head">
                <span class="tl-task">{{ n.task_label }}</span>
                <el-tag size="small" :type="statusType(n.status)">{{ n.status_label }}</el-tag>
                <span v-if="n.exported" class="tl-flag">产物已用出去</span>
              </div>
              <div class="tl-title">{{ n.task_title || '未命名任务' }}</div>
              <div class="tl-meta">
                {{ n.step_count }} 步
                <template v-if="n.decision_count">
                  · <b>{{ n.decision_count }} 个关键判断</b>
                </template>
                · {{ fmtDate(n.started_at) }}
              </div>
              <!-- 节点最有价值的产出：这次执行有没有变成一个别人能用的方法 -->
              <div v-if="n.skill_id" class="tl-skill">
                已固化为
                <router-link :to="'/trust/' + n.skill_id">{{ n.skill_name }}</router-link>
                <el-tag size="small" effect="plain">{{ n.skill_status }}</el-tag>
              </div>
              <div class="tl-actions">
                <el-button link size="small" @click="openExec(n.execution_id)">
                  回到这次执行
                </el-button>
              </div>
            </div>
          </li>
        </ol>
      </section>
      <div v-else-if="p.timeline_hidden" class="hidden-tip">成长路线未公开</div>

      <!-- 能力资产 -->
      <section v-if="p.assets" class="block">
        <h4>
          能力资产
          <span class="h-note">我沉淀出来的方法，以及它们各自有多少条判断可溯源</span>
        </h4>
        <div v-if="!p.assets.length" class="empty">还没有沉淀出可复用的方法</div>
        <div v-else class="assets">
          <div v-for="a in p.assets" :key="a.skill_id" class="asset">
            <div class="asset-top">
              <router-link :to="'/trust/' + a.skill_id" class="asset-name">{{ a.name }}</router-link>
              <el-tag size="small" :type="a.status === 'published' ? 'success' : 'info'">
                {{ a.status_label }}
              </el-tag>
            </div>
            <div class="asset-meta">
              {{ a.task_label }} · v{{ a.version }} ·
              {{ a.traceable_decisions }} 条判断可溯源 ·
              被调用 {{ a.call_count }} 次
            </div>
          </div>
        </div>
      </section>
      <div v-else-if="p.assets_hidden" class="hidden-tip">能力资产未公开</div>

      <!-- 影响力 -->
      <section v-if="p.influence" class="block">
        <h4>
          影响力
          <span class="h-note">{{ p.influence.note }}</span>
        </h4>
        <div class="infl">
          <div class="in">
            <div class="in-num">{{ p.influence.successors }}</div>
            <div class="in-label">后继者</div>
          </div>
          <div class="in">
            <div class="in-num">{{ p.influence.helped_people }}</div>
            <div class="in-label">用过我方法的人</div>
          </div>
          <div class="in">
            <div class="in-num">{{ p.influence.effective_completions }}</div>
            <div class="in-label">有效完成</div>
          </div>
          <div class="in">
            <div class="in-num">{{ p.influence.decisions_contributed }}</div>
            <div class="in-label">贡献的判断</div>
          </div>
          <div class="in">
            <div class="in-num">{{ p.influence.adopted_feedback }}</div>
            <div class="in-label">被采纳的反馈</div>
          </div>
          <div class="in" v-if="p.influence.maintaining_others">
            <div class="in-num">{{ p.influence.maintaining_others }}</div>
            <div class="in-label">接手维护</div>
          </div>
        </div>
        <p class="infl-basis">{{ p.influence.successor_basis }}</p>
      </section>
      <div v-else-if="p.influence_hidden" class="hidden-tip">影响力未公开</div>

      <!-- 失败与复盘：刻意保留，成长身份才可信 -->
      <section v-if="p.setbacks" class="block">
        <h4>
          停下来的地方
          <span class="h-note">{{ p.setbacks.note }}</span>
        </h4>
        <div v-if="!p.setbacks.stopped.length && !p.setbacks.insights.length" class="empty">
          还没有中途停下的记录
        </div>
        <div v-for="s in p.setbacks.stopped" :key="'st' + s.execution_id" class="setback">
          {{ s.task_label }}「{{ s.task_title }}」— 停在第 {{ s.stopped_at_step }} 步
        </div>
        <div v-for="i in p.setbacks.insights" :key="'in' + i.insight_id" class="note-item">
          <div class="note-claim">经验笔记：{{ i.claim }}</div>
          <div class="note-missing">
            还缺：{{ Array.isArray(i.still_missing) ? i.still_missing.join('；') : '' }}
          </div>
        </div>
      </section>
      <div v-else-if="p.setbacks_hidden" class="hidden-tip">复盘记录未公开</div>

      <!-- 可见性开关：默认全部不公开 -->
      <section v-if="p.is_self && p.visibility" class="block vis">
        <h4>
          谁能看到
          <span class="h-note">默认全部不公开。你自己决定公开哪几段。</span>
        </h4>
        <div class="vis-rows">
          <div v-for="k in p.visibility_keys" :key="k" class="vis-row">
            <span>{{ visLabel(k) }}</span>
            <el-switch
              :model-value="p.visibility[k]"
              @change="(v) => toggleVis(k, v)"
            />
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getMyGrowthProfile, getUserGrowthProfile, updateVisibility } from '../api/growth'

const props = defineProps({
  userId: { type: [Number, String], default: null },
})

const router = useRouter()
const loading = ref(true)
const p = ref(null)

// 按成长阶段从高到低排，最有分量的方向排在前面
const sortedStates = computed(() => {
  if (!p.value || !p.value.growth_states) return []
  return [...p.value.growth_states].sort((a, b) => b.rank - a.rank)
})

function statusType(s) {
  const m = {
    completed: 'success',
    running: 'primary',
    abandoned: 'info',
    handed_off: 'warning',
    failed: 'danger',
  }
  return m[s] || 'info'
}

function fmtDate(s) {
  if (!s) return ''
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function visLabel(k) {
  const m = {
    timeline: '成长路线',
    states: '成长状态',
    assets: '能力资产',
    influence: '影响力',
    failures: '停下来的地方',
  }
  return m[k] || k
}

function openExec(id) {
  router.push({ path: '/workbench', query: { id } })
}

async function toggleVis(key, val) {
  try {
    const res = await updateVisibility({ [key]: val })
    p.value.visibility = res.visibility
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(async () => {
  try {
    p.value = props.userId
      ? await getUserGrowthProfile(props.userId)
      : await getMyGrowthProfile()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.gp {
  min-height: 80px;
}
.pos {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid #f0f0f0;
}
.pos-label {
  font-size: 12px;
  color: #909399;
}
.pos-main {
  font-size: 18px;
  font-weight: 600;
  margin: 4px 0;
}
.pos-empty {
  font-size: 14px;
  font-weight: 400;
  color: #606266;
}
.pos-sub {
  font-size: 12px;
  color: #909399;
}
.principle {
  font-size: 12px;
  color: #909399;
  line-height: 1.7;
  background: #fafafa;
  border-left: 2px solid #dcdfe6;
  padding: 8px 12px;
  margin: 14px 0 0;
}
.block {
  margin-top: 22px;
}
.block h4 {
  margin: 0 0 12px;
  font-size: 15px;
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}
.h-note {
  font-size: 12px;
  font-weight: 400;
  color: #909399;
  line-height: 1.6;
}
.empty,
.hidden-tip {
  font-size: 13px;
  color: #c0c4cc;
  padding: 10px 0;
}
/* 四阶状态：用轨道而不是进度条，避免读成"完成度百分比" */
.states {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.st-task {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 6px;
}
.st-track {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.st-dot {
  font-size: 12px;
  padding: 3px 12px;
  border-radius: 12px;
  border: 1px solid #e4e7ed;
  color: #c0c4cc;
  background: #fff;
}
.st-dot.on {
  border-color: #409eff;
  color: #fff;
  background: #409eff;
}
.st-detail {
  font-size: 12px;
  color: #909399;
  margin-top: 6px;
}
/* 时间线 */
.tl {
  list-style: none;
  margin: 0;
  padding: 0;
}
.tl li {
  position: relative;
  padding: 0 0 18px 22px;
  border-left: 2px solid #ebeef5;
}
.tl li:last-child {
  border-left-color: transparent;
}
.tl-dot {
  position: absolute;
  left: -6px;
  top: 4px;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #dcdfe6;
  border: 2px solid #fff;
}
.tl li.s-completed .tl-dot {
  background: #67c23a;
}
.tl li.s-running .tl-dot {
  background: #409eff;
}
.tl li.s-abandoned .tl-dot {
  background: #c0c4cc;
}
.tl-head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.tl-task {
  font-size: 14px;
  font-weight: 600;
}
.tl-flag {
  font-size: 11px;
  color: #67c23a;
}
.tl-title {
  font-size: 13px;
  color: #606266;
  margin-top: 2px;
}
.tl-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
.tl-skill {
  font-size: 12px;
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.tl-skill a {
  color: #409eff;
  text-decoration: none;
}
.tl-actions {
  margin-top: 2px;
}
/* 能力资产 */
.assets {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.asset {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 12px 14px;
}
.asset-top {
  display: flex;
  align-items: center;
  gap: 8px;
}
.asset-name {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  text-decoration: none;
}
.asset-name:hover {
  color: #409eff;
}
.asset-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 6px;
}
/* 影响力 */
.infl {
  display: flex;
  gap: 26px;
  flex-wrap: wrap;
}
.in-num {
  font-size: 24px;
  font-weight: 700;
}
.in-label {
  font-size: 12px;
  color: #909399;
}
.infl-basis {
  font-size: 11px;
  color: #c0c4cc;
  margin: 12px 0 0;
}
/* 失败与复盘 */
.setback,
.note-item {
  font-size: 13px;
  color: #606266;
  padding: 8px 12px;
  background: #fafafa;
  border-radius: 6px;
  margin-bottom: 8px;
  line-height: 1.7;
}
.note-missing {
  font-size: 12px;
  color: #e6a23c;
  margin-top: 4px;
}
/* 可见性 */
.vis {
  border-top: 1px solid #f0f0f0;
  padding-top: 16px;
}
.vis-rows {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-width: 320px;
}
.vis-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
}
</style>
