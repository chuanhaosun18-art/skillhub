<!--
  任务工作台（PRD F4）：让「做事」发生在平台内。
  三栏布局：左=任务上下文与材料，中=与 AI 协作执行区，右=实时执行轨迹。
  关键设计：遇到关键判断时系统必须停下来让用户选，不替用户决定——
  这些判断就是后面能被蒸馏成 Skill 的东西。
-->
<template>
  <div class="wb">
    <!-- 左：任务上下文 -->
    <aside class="wb-side">
      <h3 class="side-title">任务</h3>
      <div v-if="exec" class="ctx">
        <div class="ctx-label">{{ exec.task_label || taskLabel }}</div>
        <div class="ctx-title">{{ exec.task_title || '未命名任务' }}</div>
        <el-tag :type="statusType" size="small">{{ statusText }}</el-tag>
      </div>

      <h3 class="side-title">我的材料</h3>
      <el-input
        v-model="material"
        type="textarea"
        :rows="10"
        placeholder="把选题草稿、简历原文贴进来。材料越具体，判断越准。"
        :disabled="!!execId"
      />

      <div v-if="!execId" class="start-box">
        <el-select v-model="taskIntent" placeholder="选择任务类型" style="width: 100%">
          <el-option
            v-for="t in TASK_INTENTS"
            :key="t.value"
            :label="t.label"
            :value="t.value"
          />
        </el-select>
        <el-input v-model="taskTitle" placeholder="给这次任务起个名字" style="margin-top: 8px" />
        <el-input
          v-model="goal"
          type="textarea"
          :rows="2"
          placeholder="你想达成什么？"
          style="margin-top: 8px"
        />
        <el-button
          type="primary"
          style="width: 100%; margin-top: 10px"
          :loading="starting"
          @click="start"
        >
          开始做这件事
        </el-button>
      </div>

      <div v-else class="side-actions">
        <el-button
          v-if="exec && exec.status === 'running'"
          type="primary"
          :loading="advancing"
          style="width: 100%"
          @click="advance"
        >
          {{ steps.length <= 1 ? '开始第一步' : '下一步' }}
        </el-button>
        <el-button
          v-if="exec && exec.status === 'running'"
          style="width: 100%; margin: 8px 0 0"
          @click="finish"
        >
          我做完了
        </el-button>
        <el-button
          v-if="exec && exec.status === 'running'"
          text
          style="width: 100%"
          @click="quit"
        >
          先不做了
        </el-button>

        <div v-if="exec && exec.status === 'completed'" class="done-box">
          <p class="done-title">这次任务已完成</p>
          <p v-if="!canDistill" class="done-hint">{{ distillHint }}</p>
          <el-button
            v-else
            type="success"
            style="width: 100%"
            :loading="distilling"
            @click="distill"
          >
            把这次的方法固化下来
          </el-button>

          <!-- 反向通道：让任务态用户知道编排态存在。没有这一步，用户做完就走了 -->
          <div v-if="orchSuggestion" class="orch-tip">
            <div class="orch-tip-text">{{ orchSuggestion.message }}</div>
            <el-button
              size="small"
              text
              type="primary"
              @click="$router.push({ path: '/orchestration', query: { intent: orchSuggestion.orchestration_intent } })"
            >
              {{ orchSuggestion.cta }}
            </el-button>
          </div>
        </div>
      </div>
    </aside>

    <!-- 中：协作执行区 -->
    <main class="wb-main">
      <div v-if="!execId" class="empty">
        <h2>先做一遍，再固化</h2>
        <p>
          这里不是聊天框。我会带你一步一步把这件事做完，遇到真正需要你判断的地方会停下来问你。
          做完之后，这次的过程会自动变成一个别人也能用的方法。
        </p>
      </div>

      <template v-else>
        <!-- 待决策：关键判断停顿点 -->
        <div v-if="pending" class="card decision">
          <div class="dec-badge">关键判断</div>
          <h3 class="dec-slot">{{ pending.slot_prompt }}</h3>
          <p class="dec-signal">{{ pending.signal }}</p>
          <div class="dec-options">
            <div
              v-for="(opt, i) in pending.options"
              :key="i"
              class="dec-opt"
              :class="{ picked: picked === opt }"
              @click="picked = opt"
            >
              <span class="opt-idx">{{ String.fromCharCode(65 + i) }}</span>
              <span>{{ opt }}</span>
            </div>
          </div>
          <el-input
            v-model="decisionNote"
            type="textarea"
            :rows="2"
            placeholder="为什么这么选？（选填，但写了以后蒸馏出来的方法会准得多）"
            style="margin-top: 10px"
          />
          <el-button
            type="primary"
            :disabled="!picked"
            :loading="deciding"
            style="margin-top: 10px"
            @click="decide"
          >
            就这么定
          </el-button>
          <p class="dec-note">这一步不由 AI 替你决定。你的选择会成为这个方法里的一条判断。</p>
        </div>

        <!-- 最近一步的产出 -->
        <div v-else-if="latest" class="card">
          <div class="card-head">
            <h3>{{ latest.title }}</h3>
            <el-tag v-if="latest.step_type === 'tool_call'" size="small" type="info">
              工具 {{ latest.tool_name }}
            </el-tag>
          </div>
          <el-alert
            v-if="latest.step_type === 'tool_call' && latest.tool_ok === false"
            type="warning"
            :closable="false"
            title="未能验证，以下内容仅为模型判断"
            style="margin-bottom: 10px"
          />
          <el-input v-model="latestOutput" type="textarea" :rows="12" />
          <div class="card-actions">
            <el-button size="small" :loading="saving" @click="saveEdit">保存我的修改</el-button>
            <span v-if="correctionRatio > 0" class="hint">
              人工修正率 {{ (correctionRatio * 100).toFixed(0) }}%
            </span>
          </div>
        </div>

        <el-alert
          v-if="handoff"
          type="warning"
          :closable="false"
          :title="handoff"
          description="这已经超出这条方法的适用范围了，需要人来判断。这不算失败——知道什么时候该停下来，本身就是能力的一部分。"
          style="margin-top: 12px"
        />

        <el-alert
          v-if="degraded"
          type="info"
          :closable="false"
          title="这一步没能自动生成"
          description="你可以直接在上面写下这一步做了什么，然后继续下一步。"
          style="margin-top: 12px"
        />
      </template>
    </main>

    <!-- 右：实时执行轨迹 -->
    <aside class="wb-trace">
      <h3 class="side-title">执行轨迹</h3>
      <p class="trace-note">
        这条轨迹是证据。之后生成的每一条判断都会标注它来自第几步。
      </p>
      <div v-if="!steps.length" class="trace-empty">还没有步骤</div>
      <ol class="trace">
        <li v-for="s in steps" :key="s.step_index" :class="'t-' + s.step_type">
          <div class="t-head">
            <span class="t-idx">{{ s.step_index }}</span>
            <span class="t-type">{{ stepTypeText(s.step_type) }}</span>
            <span v-if="s.latency_ms" class="t-ms">{{ s.latency_ms }}ms</span>
          </div>
          <div class="t-title">{{ s.title || '—' }}</div>
          <div v-if="s.step_type === 'user_decision'" class="t-choice">
            <span v-if="s.user_choice">已选：{{ choiceText(s.user_choice) }}</span>
            <span v-else class="t-waiting">等待你的判断</span>
          </div>
          <div v-else-if="s.tool_name" class="t-tool">
            {{ s.tool_name }} · {{ s.tool_ok === false ? '失败' : '成功' }}
          </div>
        </li>
      </ol>
    </aside>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  TASK_INTENTS,
  createExecution,
  getExecution,
  advanceExecution,
  recordDecision,
  recordEdit,
  completeExecution,
  abandonExecution,
  distillExecution,
} from '../api/growth'

const route = useRoute()
const router = useRouter()

const execId = ref(route.query.id ? Number(route.query.id) : null)
const exec = ref(null)
const steps = ref([])
const canDistill = ref(false)
const distillHint = ref('')

const taskIntent = ref(route.query.intent || 'thesis_topic')
const taskTitle = ref('')
const goal = ref(route.query.goal || '')
const material = ref('')

const starting = ref(false)
const advancing = ref(false)
const deciding = ref(false)
const saving = ref(false)
const distilling = ref(false)

const pending = ref(null)
const orchSuggestion = ref(null)
const picked = ref('')
const decisionNote = ref('')
const handoff = ref('')
const degraded = ref(false)
const latestOutput = ref('')
const correctionRatio = ref(0)

const taskLabel = computed(
  () => (TASK_INTENTS.find((t) => t.value === taskIntent.value) || {}).label || ''
)

const latest = computed(() => {
  const list = steps.value.filter((s) => s.step_type !== 'user_decision')
  return list.length ? list[list.length - 1] : null
})

const statusText = computed(() => {
  const m = {
    running: '进行中',
    completed: '已完成',
    abandoned: '已放弃',
    handed_off: '已交回给人',
    failed: '出错',
  }
  return m[exec.value?.status] || exec.value?.status || ''
})
const statusType = computed(() => {
  const m = { running: 'primary', completed: 'success', abandoned: 'info', handed_off: 'warning' }
  return m[exec.value?.status] || 'info'
})

function stepTypeText(t) {
  const m = {
    ai_action: 'AI 产出',
    tool_call: '工具调用',
    user_decision: '关键判断',
    human_handoff: '交回给人',
  }
  return m[t] || t
}

function choiceText(raw) {
  try {
    const o = typeof raw === 'string' ? JSON.parse(raw) : raw
    return o.choice || ''
  } catch (e) {
    return ''
  }
}

async function load() {
  if (!execId.value) return
  try {
    const res = await getExecution(execId.value)
    exec.value = res.data
    steps.value = res.data.steps || []
    canDistill.value = !!res.can_distill
    correctionRatio.value = res.data.correction_ratio || 0
    // 找出尚未做出选择的关键判断
    const waiting = steps.value.find(
      (s) => s.step_type === 'user_decision' && !s.user_choice
    )
    if (waiting) {
      pending.value = {
        step_index: waiting.step_index,
        slot_prompt: waiting.decision_slot,
        signal: waiting.input,
        options: safeOptions(waiting.output),
      }
    }
    if (latest.value) latestOutput.value = latest.value.output || ''
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function safeOptions(raw) {
  try {
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? arr : []
  } catch (e) {
    return []
  }
}

async function start() {
  if (!taskIntent.value) return ElMessage.warning('先选一个任务类型')
  starting.value = true
  try {
    const res = await createExecution({
      task_intent: taskIntent.value,
      task_title: taskTitle.value || taskLabel.value,
      goal: goal.value,
      material: material.value,
    })
    execId.value = res.data.id
    router.replace({ path: '/workbench', query: { id: res.data.id } })
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    starting.value = false
  }
}

async function advance() {
  advancing.value = true
  handoff.value = ''
  degraded.value = false
  try {
    const res = await advanceExecution(execId.value)
    if (res.mode === 'decision') {
      pending.value = res
      picked.value = ''
      decisionNote.value = ''
    } else if (res.mode === 'handoff') {
      handoff.value = res.title
    } else if (res.mode === 'degraded') {
      degraded.value = true
    } else if (res.mode === 'action' && res.done) {
      ElMessage.success('完成标准已满足：' + (res.done_reason || ''))
    }
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    advancing.value = false
  }
}

async function decide() {
  deciding.value = true
  try {
    await recordDecision(execId.value, pending.value.step_index, picked.value, decisionNote.value)
    pending.value = null
    picked.value = ''
    await load()
    ElMessage.success('这条判断已记进轨迹')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    deciding.value = false
  }
}

async function saveEdit() {
  if (!latest.value) return
  saving.value = true
  try {
    const res = await recordEdit(execId.value, latest.value.step_index, latestOutput.value)
    correctionRatio.value = res.correction_ratio
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

async function finish() {
  try {
    await ElMessageBox.confirm(
      '把当前产物作为最终结果提交？导出这个动作会被记录——它是判断这个方法是否真的有用的依据。',
      '完成任务',
      { confirmButtonText: '提交并完成', cancelButtonText: '再改改' }
    )
  } catch (e) {
    return
  }
  try {
    const res = await completeExecution(execId.value, {
      exported: true,
      finalArtifact: latestOutput.value,
    })
    canDistill.value = !!res.can_distill
    distillHint.value = res.distill_hint || ''
    orchSuggestion.value = res.orchestration_suggestion || null
    await load()
    ElMessage.success('已完成')
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function quit() {
  try {
    await abandonExecution(execId.value)
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function distill() {
  distilling.value = true
  try {
    const res = await distillExecution(execId.value)
    router.push({ path: '/creator', query: { v: res.version_id } })
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    distilling.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.wb {
  display: grid;
  grid-template-columns: 320px 1fr 300px;
  gap: 16px;
  padding: 20px;
  max-width: 1600px;
  margin: 0 auto;
  align-items: start;
}
.wb-side,
.wb-trace {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 16px;
  position: sticky;
  top: 16px;
}
.wb-main {
  min-height: 60vh;
}
.side-title {
  font-size: 13px;
  color: #909399;
  margin: 0 0 8px;
  font-weight: 600;
}
.side-title:not(:first-child) {
  margin-top: 18px;
}
.ctx-label {
  font-size: 12px;
  color: #909399;
}
.ctx-title {
  font-size: 16px;
  font-weight: 600;
  margin: 4px 0 8px;
}
.start-box,
.side-actions {
  margin-top: 14px;
}
.done-box {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid #f0f0f0;
}
.done-title {
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 6px;
}
.done-hint {
  font-size: 12px;
  color: #e6a23c;
  line-height: 1.6;
  margin: 0 0 8px;
}
/* 反向通道提示：卡片式、不弹窗、不打断固化流程 */
.orch-tip {
  margin-top: 12px;
  padding: 10px 12px;
  background: #f7fbff;
  border: 1px solid #c6e2ff;
  border-radius: 8px;
}
.orch-tip-text {
  font-size: 12px;
  color: #606266;
  line-height: 1.7;
  margin-bottom: 6px;
}
.empty {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 48px 40px;
  text-align: center;
}
.empty h2 {
  margin: 0 0 12px;
}
.empty p {
  color: #606266;
  line-height: 1.8;
  max-width: 520px;
  margin: 0 auto;
}
.card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 18px;
}
.card-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.card-head h3 {
  margin: 0;
  font-size: 16px;
}
.card-actions {
  margin-top: 10px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.hint {
  font-size: 12px;
  color: #909399;
}
/* 关键判断卡：视觉上必须比普通步骤重，因为它是这个产品的核心动作 */
.decision {
  border: 2px solid #409eff;
  background: #f7fbff;
}
.dec-badge {
  display: inline-block;
  background: #409eff;
  color: #fff;
  font-size: 12px;
  padding: 2px 10px;
  border-radius: 10px;
}
.dec-slot {
  margin: 10px 0 6px;
  font-size: 17px;
}
.dec-signal {
  color: #606266;
  line-height: 1.7;
  margin: 0 0 12px;
  white-space: pre-wrap;
}
.dec-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.dec-opt {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 12px 14px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  cursor: pointer;
  background: #fff;
  line-height: 1.6;
}
.dec-opt:hover {
  border-color: #409eff;
}
.dec-opt.picked {
  border-color: #409eff;
  background: #ecf5ff;
}
.opt-idx {
  font-weight: 700;
  color: #409eff;
}
.dec-note {
  font-size: 12px;
  color: #909399;
  margin: 10px 0 0;
}
.trace-note {
  font-size: 12px;
  color: #909399;
  line-height: 1.6;
  margin: 0 0 10px;
}
.trace-empty {
  color: #c0c4cc;
  font-size: 13px;
}
.trace {
  list-style: none;
  padding: 0;
  margin: 0;
  max-height: 60vh;
  overflow-y: auto;
}
.trace li {
  border-left: 2px solid #ebeef5;
  padding: 8px 0 8px 12px;
  margin-left: 4px;
}
.trace li.t-user_decision {
  border-left-color: #409eff;
}
.trace li.t-tool_call {
  border-left-color: #909399;
}
.trace li.t-human_handoff {
  border-left-color: #e6a23c;
}
.t-head {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #909399;
}
.t-idx {
  background: #f4f4f5;
  border-radius: 4px;
  padding: 0 5px;
}
.t-title {
  font-size: 13px;
  margin-top: 2px;
}
.t-choice,
.t-tool {
  font-size: 12px;
  color: #606266;
  margin-top: 2px;
}
.t-waiting {
  color: #409eff;
}
@media (max-width: 1200px) {
  .wb {
    grid-template-columns: 1fr;
  }
  .wb-side,
  .wb-trace {
    position: static;
  }
}
</style>
