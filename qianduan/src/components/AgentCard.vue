<!--
  卡片渲染器：agent 返回什么 type，这里就渲染对应的交互。

  关键设计：不是把所有东西塞进对话气泡。我们最值钱的交互都是非文本的——
  关键判断要点选、蒸馏度要看六项、证据要能下钻、编排要按周勾。
  所以卡片保留结构化交互，只是不再是独立页面。
-->
<template>
  <div class="card" :class="'t-' + card.type">
    <!-- agent 说的话 -->
    <div class="say">{{ card.say }}</div>
    <!-- 为什么现在给这张卡：可解释性，也是产品性格的体现 -->
    <div v-if="card.why" class="why">{{ card.why }}</div>

    <!-- ① 空闲：唯一的入口 -->
    <div v-if="card.type === 'idle'" class="body">
      <div class="chips">
        <span
          v-for="(ex, i) in card.data.examples"
          :key="i"
          class="chip"
          @click="$emit('fill', ex)"
        >{{ ex }}</span>
      </div>
    </div>

    <!-- ② 拒绝态：语气要稳，不给方法，不列清单 -->
    <div v-else-if="card.type === 'reject'" class="body">
      <div v-if="card.data.records_created === false" class="privacy">
        这段对话不会留下任何记录。
      </div>
      <div v-if="(card.data.branches || []).length" class="branches">
        <div class="sub-title">别人走过的路，以及各自的去向</div>
        <div v-for="(b, i) in card.data.branches" :key="i" class="branch">
          <b>{{ b.goal_label }}</b>（{{ b.walked_count }} 人走过）
          <span
            v-for="(x, j) in (b.branch_summary && b.branch_summary.branches) || []"
            :key="j"
            class="bchip"
          >{{ x.count }} 人{{ x.label }}</span>
          <div v-if="b.provenance_note" class="prov">{{ b.provenance_note }}</div>
        </div>
        <div class="note">这是别人的路，不是建议。选择是你的事。</div>
      </div>
      <div class="chips">
        <span
          v-for="(r, i) in card.data.resources || []"
          :key="i"
          class="chip plain"
        >{{ r.label }}</span>
      </div>
    </div>

    <!-- ③ 四筛没过 -->
    <div v-else-if="card.type === 'not_skillable'" class="body">
      <div class="sieve">
        <span
          v-for="k in ['amortizable', 'testable', 'transferable', 'short_loop']"
          :key="k"
          class="sv"
          :class="card.data.sieve[k] ? 'ok' : 'no'"
        >{{ sieveLabel(k) }}</span>
      </div>
    </div>

    <!-- ④ 手选兜底 -->
    <div v-else-if="card.type === 'manual_fallback'" class="body">
      <div class="chips">
        <span
          v-for="o in card.data.options"
          :key="o.task_intent"
          class="chip"
          @click="$emit('pick-intent', o)"
        >{{ o.label }}</span>
      </div>
    </div>

    <!-- ⑤ 任务卡 -->
    <div v-else-if="card.type === 'task'" class="body">
      <div class="kv" v-if="card.data.current_position">
        <span class="k">你现在在这儿</span>{{ card.data.current_position }}
      </div>
      <div class="kv" v-if="(card.data.gap || []).length">
        <span class="k">还差什么</span>{{ card.data.gap.join('；') }}
      </div>
      <div class="next">
        <div class="next-k">今天的下一步</div>
        <div class="next-v">{{ card.data.next_step }}</div>
      </div>
      <div v-if="card.data.routed" class="routed">
        有 {{ card.data.routed.count }} 个现成的方法可以用：
        <router-link
          v-for="s in card.data.routed.skills"
          :key="s.skill_id"
          :to="'/trust/' + s.skill_id"
          class="slink"
        >{{ s.name }}</router-link>
        <div class="note">{{ card.data.routed.note }}</div>
      </div>
    </div>

    <!-- ⑥ 进行中：关键判断停顿 —— 这是全产品最核心的交互 -->
    <div v-else-if="card.type === 'continue_task'" class="body">
      <div class="task-line">
        {{ card.data.task_label }}
        <span v-if="card.data.task_title">· {{ card.data.task_title }}</span>
        <span v-if="card.data.step_count" class="dim">已走 {{ card.data.step_count }} 步</span>
      </div>

      <template v-if="card.data.awaiting_decision">
        <div class="dec-slot">{{ card.data.slot_prompt }}</div>
        <div class="dec-signal">{{ card.data.signal }}</div>
        <div class="opts">
          <div
            v-for="(o, i) in card.data.options"
            :key="i"
            class="opt"
            :class="{ picked: picked === o }"
            @click="picked = o"
          >
            <span class="oi">{{ String.fromCharCode(65 + i) }}</span>{{ o }}
          </div>
        </div>
        <el-input
          v-model="note"
          type="textarea"
          :rows="2"
          placeholder="为什么这么选？（选填，写了以后蒸馏出来的方法会准得多）"
          style="margin-top: 8px"
        />
      </template>
    </div>

    <!-- ⑦ 表态：调用的代价 -->
    <div v-else-if="card.type === 'verdict'" class="body">
      <div v-for="d in card.data.decisions" :key="d.decision_id" class="vd">
        <div class="vd-slot">{{ d.slot_prompt }}</div>
        <div class="vd-stmt">{{ d.statement }}</div>
        <div class="vd-scope">适用：{{ d.scope }}</div>
        <el-radio-group v-model="verdicts[d.decision_id]" size="small">
          <el-radio-button value="held">成立</el-radio-button>
          <el-radio-button value="failed">不成立</el-radio-button>
          <el-radio-button value="not_applicable">不适用我</el-radio-button>
        </el-radio-group>
        <el-input
          v-if="verdicts[d.decision_id] === 'failed'"
          v-model="notes[d.decision_id]"
          size="small"
          placeholder="哪里不成立？这句会变成它的边界"
          style="margin-top: 6px"
        />
      </div>
    </div>

    <!-- ⑧ 蒸馏度六项 -->
    <div v-else-if="card.type === 'draft_gate' && card.data.detail" class="body">
      <div class="score-row">
        <span class="score">{{ (card.data.score * 100).toFixed(0) }}</span>
        <span class="dim">/ 发布线 {{ (card.data.threshold * 100).toFixed(0) }}</span>
      </div>
      <div class="dims">
        <span
          v-for="(v, k) in card.data.detail"
          v-show="typeof v === 'number' && k !== 'cap'"
          :key="k"
          class="dm"
          :class="v >= 1 ? 'ok' : v > 0 ? 'half' : 'no'"
        >{{ (card.data.labels && card.data.labels[k]) || k }}</span>
      </div>
      <ul v-if="(card.data.still_missing || []).length" class="missing">
        <li v-for="(m, i) in card.data.still_missing" :key="i">{{ m }}</li>
      </ul>
    </div>

    <!-- ⑨ 门禁：四问未跑 -->
    <div v-else-if="card.type === 'draft_gate'" class="body">
      <div v-if="(card.data.unrun || []).length" class="unrun">
        还没跑：{{ card.data.unrun.join('、') }}
      </div>
    </div>

    <!-- ⑩ 升级 -->
    <div v-else-if="card.type === 'upgrade'" class="body">
      <div class="kv"><span class="k">触发规则</span>{{ card.data.trigger_rule }}</div>
      <el-input
        v-model="changelog"
        type="textarea"
        :rows="2"
        placeholder="这一版改了什么、为什么（必填）"
      />
    </div>

    <!-- ⑪ 其余：编排复核 / 简历 / 固化，用统计信息 + 按钮 -->
    <div v-else class="body">
      <div v-if="card.data.week_index" class="kv">
        <span class="k">第 {{ card.data.week_index }} 周</span>{{ card.data.item_count }} 件事
      </div>
      <div v-if="card.data.published_skills !== undefined" class="stats">
        <span><b>{{ card.data.published_skills }}</b> 个方法在被人用</span>
        <span><b>{{ card.data.verified_decisions }}</b> 条判断被验证过</span>
      </div>
    </div>

    <!-- 动作按钮 -->
    <div v-if="(card.actions || []).length" class="acts">
      <el-button
        v-for="(a, i) in card.actions"
        :key="i"
        :type="a.primary ? 'primary' : 'default'"
        :text="!a.primary && a.method === 'SKIP'"
        size="default"
        :loading="busy === i"
        @click="$emit('act', { action: a, index: i, payload: buildPayload(a) })"
      >{{ a.label }}</el-button>
      <router-link v-if="card.deep_link" :to="card.deep_link" class="deep">
        看完整页面
      </router-link>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'

const props = defineProps({
  card: { type: Object, required: true },
  busy: { type: Number, default: -1 },
})
defineEmits(['act', 'fill', 'pick-intent'])

const picked = ref('')
const note = ref('')
const changelog = ref('')
const verdicts = reactive({})
const notes = reactive({})

function sieveLabel(k) {
  return { amortizable: '可摊销', testable: '可测试', transferable: '可转移', short_loop: '短链路' }[k] || k
}

// 每种卡片提交时需要的 body 不同，在这里集中装配，
// 免得 Agent.vue 里堆一堆 if
function buildPayload(action) {
  const t = props.card.type
  const d = props.card.data || {}
  if (t === 'continue_task' && d.awaiting_decision && action.path.endsWith('/decide')) {
    return { step_index: d.step_index, choice: picked.value, note: note.value }
  }
  if (t === 'verdict') {
    const list = Object.keys(verdicts).map((id) => ({
      decision_id: Number(id),
      verdict: verdicts[id],
      note: notes[id] || '',
    })).filter((v) => v.verdict)
    return { verdicts: list }
  }
  if (t === 'upgrade') {
    return { changelog: changelog.value }
  }
  if (t === 'task' && action.path === '/api/growth/executions') {
    return {
      task_intent: d.task_intent,
      task_title: d.task_label,
      goal: d.utterance,
      material: '',
    }
  }
  if (t === 'continue_task' && action.path.endsWith('/complete')) {
    return { exported: true, final_artifact: '' }
  }
  if (t === 'orch_entry') {
    return { utterance: d.utterance, orchestration_intent: d.orchestration_intent }
  }
  return {}
}
</script>

<style scoped>
.card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 12px;
  padding: 18px 20px;
  margin-bottom: 14px;
}
/* 需要用户判断的卡片要比普通卡片重，因为那是核心动作 */
.t-continue_task,
.t-verdict {
  border-color: #409eff;
  border-width: 2px;
  background: #f7fbff;
}
.t-reject {
  background: #fafafa;
  border-color: #dcdfe6;
}
.t-upgrade {
  border-color: #67c23a;
}
.say {
  font-size: 16px;
  line-height: 1.8;
  color: #303133;
}
.why {
  font-size: 12px;
  color: #909399;
  line-height: 1.7;
  margin-top: 8px;
  padding-left: 10px;
  border-left: 2px solid #ebeef5;
}
.body {
  margin-top: 14px;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.chip {
  font-size: 13px;
  color: #409eff;
  background: #ecf5ff;
  border-radius: 14px;
  padding: 6px 12px;
  cursor: pointer;
}
.chip.plain {
  color: #606266;
  background: #f4f4f5;
  cursor: default;
}
.privacy {
  font-size: 12px;
  color: #909399;
  margin-bottom: 10px;
}
.sub-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 8px;
}
.branch {
  font-size: 13px;
  color: #606266;
  margin-bottom: 8px;
  line-height: 1.8;
}
.bchip {
  font-size: 12px;
  background: #f4f4f5;
  border-radius: 10px;
  padding: 2px 8px;
  margin-left: 6px;
}
.prov {
  font-size: 11px;
  color: #e6a23c;
}
.note {
  font-size: 12px;
  color: #909399;
  margin-top: 8px;
}
.sieve,
.dims {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.sv,
.dm {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 10px;
  border: 1px solid #dcdfe6;
  color: #909399;
}
.sv.ok,
.dm.ok {
  border-color: #67c23a;
  color: #67c23a;
  background: #f0f9eb;
}
.sv.no,
.dm.no {
  border-color: #f56c6c;
  color: #f56c6c;
  background: #fef0f0;
}
.dm.half {
  border-color: #e6a23c;
  color: #e6a23c;
  background: #fdf6ec;
}
.kv {
  font-size: 14px;
  line-height: 1.8;
  color: #606266;
}
.k {
  display: inline-block;
  min-width: 88px;
  color: #909399;
  font-size: 12px;
}
.next {
  background: #ecf5ff;
  border-radius: 8px;
  padding: 12px 14px;
  margin: 12px 0;
}
.next-k {
  font-size: 12px;
  color: #409eff;
}
.next-v {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.6;
  margin-top: 4px;
}
.routed {
  font-size: 13px;
  color: #606266;
  margin-top: 10px;
}
.slink {
  color: #409eff;
  text-decoration: none;
  margin-left: 8px;
}
.task-line {
  font-size: 14px;
  color: #303133;
}
.dim {
  font-size: 12px;
  color: #c0c4cc;
  margin-left: 8px;
}
.dec-slot {
  font-size: 16px;
  font-weight: 600;
  margin: 12px 0 6px;
}
.dec-signal {
  font-size: 14px;
  color: #606266;
  line-height: 1.8;
  white-space: pre-wrap;
}
.opts {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 10px;
}
.opt {
  display: flex;
  gap: 10px;
  padding: 11px 13px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #fff;
  cursor: pointer;
  font-size: 14px;
  line-height: 1.6;
}
.opt:hover {
  border-color: #409eff;
}
.opt.picked {
  border-color: #409eff;
  background: #ecf5ff;
}
.oi {
  font-weight: 700;
  color: #409eff;
}
.vd {
  border-top: 1px solid #f0f0f0;
  padding: 12px 0;
}
.vd-slot {
  font-size: 12px;
  color: #909399;
}
.vd-stmt {
  font-size: 14px;
  line-height: 1.7;
  margin: 4px 0;
}
.vd-scope {
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
}
.score-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 10px;
}
.score {
  font-size: 30px;
  font-weight: 700;
}
.missing {
  margin: 10px 0 0;
  padding-left: 20px;
  font-size: 13px;
  color: #e6a23c;
  line-height: 1.9;
}
.unrun {
  font-size: 13px;
  color: #e6a23c;
}
.stats {
  display: flex;
  gap: 20px;
  font-size: 14px;
  color: #606266;
}
.stats b {
  font-size: 20px;
  color: #303133;
}
.acts {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 16px;
}
.deep {
  font-size: 12px;
  color: #909399;
  text-decoration: none;
  margin-left: auto;
}
.deep:hover {
  color: #409eff;
}
</style>
