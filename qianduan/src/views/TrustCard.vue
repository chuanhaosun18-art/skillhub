<!--
  Trust Card（PRD F10）：用户托付的是一项工作，所以要看的是证据，不是分数。
  硬约束：全页不出现综合评分、星级、排行位次。
  每个岔路口都能点开看判断级溯源（脱敏，不展示来源者的原始材料）。
-->
<template>
  <div class="tc" v-loading="loading">
    <template v-if="card">
      <header class="head">
        <div class="label">{{ card.task_label }}</div>
        <h1>{{ card.name }}</h1>
        <div class="policy">{{ card.score_policy }}</div>
      </header>

      <!-- 它做什么 -->
      <section v-if="card.what_it_does" class="card">
        <h3>它帮你完成什么</h3>
        <p class="goal">{{ card.what_it_does.goal }}</p>
        <h4>什么算做完</h4>
        <ul>
          <li v-for="(c, i) in card.what_it_does.done_criteria" :key="i">{{ c }}</li>
        </ul>
      </section>

      <!-- 流程 + 判断级溯源 -->
      <section class="card">
        <h3>流程与关键岔路口</h3>
        <ol class="steps">
          <li v-for="s in (card.workflow && card.workflow.steps) || []" :key="s.index">
            <b>{{ s.title }}</b> — {{ s.io }}
          </li>
        </ol>

        <div
          v-for="g in (card.workflow && card.workflow.decision_slots) || []"
          :key="g.slot"
          class="slot"
        >
          <div class="slot-prompt">{{ g.prompt }}</div>
          <div
            v-for="d in g.decisions"
            :key="d.decision_id"
            class="dec"
            @click="openTrace(d.decision_id)"
          >
            <div class="dec-line">
              当<b>{{ d.trigger_signal }}</b>时，{{ d.judgment }}
            </div>
            <div class="dec-meta">
              适用：{{ d.scope }}
              <span class="src">来自第 {{ d.source_step_index }} 步 · 点开看溯源</span>
            </div>
          </div>
        </div>
      </section>

      <!-- 证据 -->
      <section v-if="card.evidence" class="card">
        <h3>证据</h3>
        <div class="evals">
          <div v-for="e in card.evidence.evals || []" :key="e.eval_type" class="ev"
            :class="e.passed ? 'pass' : 'fail'">
            <div class="ev-label">{{ e.label }}</div>
            <div class="ev-rate">{{ ((e.pass_rate || 0) * 100).toFixed(0) }}%</div>
          </div>
        </div>
        <p class="meta">
          {{ card.evidence.traceable_decisions }} 条判断可溯源到真实执行
          <template v-if="card.evidence.source">
            · {{ card.evidence.source.note }}
            <template v-if="card.evidence.source.step_count">
              （{{ card.evidence.source.step_count }} 步轨迹）
            </template>
          </template>
        </p>
      </section>

      <!-- 边界 -->
      <section v-if="card.boundary" class="card">
        <h3>什么情况下不要用它</h3>
        <ul>
          <li v-for="(b, i) in card.boundary.not_applicable" :key="'n' + i">{{ b }}</li>
        </ul>
        <h4>出现这些情况会交回给人</h4>
        <ul>
          <li v-for="(b, i) in card.boundary.handoff_trigger" :key="'h' + i">{{ b }}</li>
        </ul>
        <p v-if="card.boundary.fallback_path" class="meta">
          降级路径：{{ card.boundary.fallback_path }}
        </p>
      </section>

      <!-- 授权与安全 -->
      <section v-if="card.authorization" class="card">
        <h3>它会读什么、做什么</h3>
        <div class="perms">
          <el-tag v-for="(p, i) in card.authorization.permissions" :key="i" size="small">
            {{ p }}
          </el-tag>
        </div>
        <p v-if="card.authorization.needs_confirmation" class="danger">
          含敏感操作，执行时会逐次要求确认：
          {{ (card.authorization.sensitive_ops || []).join('、') }}
        </p>
        <p class="meta">{{ card.authorization.least_privilege_note }}</p>
      </section>

      <!-- 运行 -->
      <section v-if="card.runtime" class="card">
        <h3>真实运行情况</h3>
        <p v-if="card.runtime.note" class="warn-note">{{ card.runtime.note }}</p>
        <div class="stats">
          <div class="stat">
            <div class="stat-num">{{ card.runtime.sample_size }}</div>
            <div class="stat-label">被调用次数</div>
          </div>
          <div v-if="card.runtime.adoption_rate !== undefined" class="stat">
            <div class="stat-num">{{ pct(card.runtime.adoption_rate) }}</div>
            <div class="stat-label">用出去了</div>
          </div>
          <div v-if="card.runtime.abandon_rate !== undefined" class="stat">
            <div class="stat-num">{{ pct(card.runtime.abandon_rate) }}</div>
            <div class="stat-label">中途放弃</div>
          </div>
          <div v-if="card.runtime.correction_rate !== undefined" class="stat">
            <div class="stat-num">{{ pct(card.runtime.correction_rate) }}</div>
            <div class="stat-label">人工改写</div>
          </div>
        </div>
        <p class="meta">
          这里没有「成功率」。成长类结果的周期是数周到数月、混杂变量极多，
          所以我们用行为信号代替：有没有用出去、有没有放弃、有没有大改。
        </p>
      </section>

      <!-- 维护 -->
      <section v-if="card.maintenance" class="card">
        <h3>谁在维护</h3>
        <p>
          创作者
          <router-link
            v-if="card.maintenance.creator_user_id"
            :to="'/growth/' + card.maintenance.creator_user_id"
            class="who"
          >{{ card.maintenance.creator }}</router-link>
          <span v-else>{{ card.maintenance.creator || '—' }}</span>
          <template v-if="card.maintenance.maintainer">
            · 当前维护者
            <router-link
              v-if="card.maintenance.maintainer_user_id"
              :to="'/growth/' + card.maintenance.maintainer_user_id"
              class="who"
            >{{ card.maintenance.maintainer }}</router-link>
            <span v-else>{{ card.maintenance.maintainer }}</span>
          </template>
          <el-tag v-if="card.maintenance.handed_over" size="small" type="warning">已移交</el-tag>
        </p>
        <p class="meta">最近更新：{{ card.maintenance.updated_at }}</p>
        <div v-if="card.versions && card.versions.length" class="versions">
          <div v-for="v in card.versions" :key="v.version" class="ver">
            <b>v{{ v.version }}</b>
            <span>{{ v.changelog || '（无变更说明）' }}</span>
          </div>
        </div>
      </section>
    </template>

    <!-- 判断级溯源浮层 -->
    <el-dialog v-model="traceOpen" title="这条判断从哪来" width="560px">
      <div v-if="trace">
        <div class="tr-slot">{{ trace.decision.slot_prompt }}</div>
        <p class="tr-line">
          当<b>{{ trace.decision.trigger_signal }}</b>时，{{ trace.decision.judgment }}
        </p>
        <p class="tr-meta">适用场景：{{ trace.decision.scope }}</p>
        <p v-if="trace.decision.counter_example" class="tr-meta">
          已知反例：{{ trace.decision.counter_example }}
        </p>
        <div class="tr-src">
          <div class="tr-src-title">
            来源：{{ trace.source.task_label }} 第 {{ trace.source.step_index }} 步
          </div>
          <div class="tr-src-body">{{ trace.source.situation_summary }}</div>
        </div>
        <p class="tr-privacy">{{ trace.privacy_note }}</p>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getTrustCard, getDecisionTrace } from '../api/growth'

const route = useRoute()
const loading = ref(true)
const card = ref(null)
const traceOpen = ref(false)
const trace = ref(null)

function pct(v) {
  if (v === undefined || v === null) return '—'
  return (v * 100).toFixed(0) + '%'
}

async function openTrace(id) {
  try {
    trace.value = await getDecisionTrace(id)
    traceOpen.value = true
  } catch (e) {
    ElMessage.error(e.message)
  }
}

onMounted(async () => {
  try {
    card.value = await getTrustCard(route.params.id)
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.tc {
  max-width: 820px;
  margin: 0 auto;
  padding: 30px 20px 60px;
}
.head .label {
  font-size: 12px;
  color: #909399;
}
.head h1 {
  margin: 6px 0 8px;
  font-size: 26px;
}
.policy {
  font-size: 12px;
  color: #909399;
  background: #fafafa;
  border-left: 2px solid #dcdfe6;
  padding: 8px 12px;
  line-height: 1.7;
}
.card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 20px;
  margin-top: 16px;
}
.card h3 {
  margin: 0 0 10px;
  font-size: 16px;
}
.card h4 {
  margin: 14px 0 6px;
  font-size: 13px;
  color: #606266;
}
.card ul,
.steps {
  margin: 0;
  padding-left: 20px;
  line-height: 1.9;
  color: #606266;
  font-size: 14px;
}
.goal {
  margin: 0;
  line-height: 1.8;
}
.slot {
  margin-top: 16px;
}
.slot-prompt {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
}
.dec {
  background: #fafafa;
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 8px;
  cursor: pointer;
  border: 1px solid transparent;
}
.dec:hover {
  border-color: #409eff;
  background: #f7fbff;
}
.dec-line {
  font-size: 14px;
  line-height: 1.7;
}
.dec-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
.src {
  color: #409eff;
  margin-left: 6px;
}
.evals {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}
.ev {
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  padding: 10px 14px;
  min-width: 120px;
}
.ev.pass {
  border-color: #67c23a;
}
.ev.fail {
  border-color: #f56c6c;
}
.ev-label {
  font-size: 12px;
  color: #606266;
  line-height: 1.4;
}
.ev-rate {
  font-size: 20px;
  font-weight: 700;
  margin-top: 4px;
}
.meta {
  font-size: 12px;
  color: #909399;
  line-height: 1.8;
  margin: 12px 0 0;
}
.perms {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.danger {
  color: #f56c6c;
  font-size: 13px;
  margin: 10px 0 0;
}
.warn-note {
  color: #e6a23c;
  font-size: 12px;
  margin: 0 0 10px;
}
.stats {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}
.stat-num {
  font-size: 24px;
  font-weight: 700;
}
.stat-label {
  font-size: 12px;
  color: #909399;
}
/* 创作者可点进成长身份 */
.who {
  color: #409eff;
  text-decoration: none;
}
.who:hover {
  text-decoration: underline;
}
.versions {
  margin-top: 12px;
}
.ver {
  display: flex;
  gap: 10px;
  font-size: 13px;
  padding: 6px 0;
  border-top: 1px solid #f5f5f5;
  color: #606266;
}
.tr-slot {
  font-size: 13px;
  color: #909399;
}
.tr-line {
  font-size: 16px;
  line-height: 1.8;
  margin: 8px 0;
}
.tr-meta {
  font-size: 13px;
  color: #606266;
  margin: 4px 0;
}
.tr-src {
  background: #f7fbff;
  border-radius: 8px;
  padding: 12px;
  margin: 14px 0 10px;
}
.tr-src-title {
  font-size: 12px;
  color: #409eff;
  margin-bottom: 6px;
}
.tr-src-body {
  font-size: 13px;
  line-height: 1.7;
  color: #303133;
}
.tr-privacy {
  font-size: 12px;
  color: #909399;
  margin: 0;
}
</style>
