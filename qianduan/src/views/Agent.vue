<!--
  单一入口。用户只面对这一个页面。

  流程刻意做成三段，每段的职责不重叠：
    说一句话        → POST /agent/say   （唯一调模型的地方）
    在卡片上操作    → 直接打原有端点    （门禁与硬约束都在那儿，绕不过去）
    操作完成        → GET  /agent/state （纯规则推导下一张卡）

  旧页面全部保留做深链：Trust Card 要能分享，快照更要能分享。
-->
<template>
  <div class="page">
    <AppNavbar />
    <main class="stream" ref="streamEl">
      <div class="head">
        <h1>WowSkillLand</h1>
        <p class="tag">说一句你现在卡在哪，剩下的我来安排</p>
      </div>

      <!-- 历史消息 -->
      <div v-for="(m, i) in log" :key="i" class="turn">
        <div v-if="m.role === 'user'" class="mine">{{ m.text }}</div>
        <AgentCard
          v-else
          :card="m.card"
          :busy="busyIndex"
          @act="onAct"
          @fill="fill"
          @pick-intent="onPickIntent"
        />
      </div>

      <div v-if="loading" class="thinking">…</div>

      <!-- 输入区常驻底部 -->
      <div class="composer">
        <el-input
          v-model="draft"
          type="textarea"
          :rows="2"
          :disabled="loading"
          placeholder="用你自己的话说，比如：我的选题被导师退了两次，说范围太大"
          @keydown.ctrl.enter="say"
        />
        <div class="composer-row">
          <el-button type="primary" :loading="loading" @click="say">说给它听</el-button>
          <el-button text :disabled="loading" @click="refresh">看看我现在该做什么</el-button>
          <span class="hint">Ctrl + Enter 发送</span>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AppNavbar from '../components/AppNavbar.vue'
import AgentCard from '../components/AgentCard.vue'
import { agentState, agentSay, runAction, createExecution } from '../api/growth'

const router = useRouter()
const log = ref([])
const draft = ref('')
const loading = ref(false)
const busyIndex = ref(-1)
const streamEl = ref(null)

function push(entry) {
  log.value.push(entry)
  nextTick(() => {
    if (streamEl.value) streamEl.value.scrollTop = streamEl.value.scrollHeight
  })
}

function fill(text) {
  draft.value = text
}

async function refresh() {
  loading.value = true
  try {
    const res = await agentState()
    push({ role: 'agent', card: res.card })
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function say() {
  const text = draft.value.trim()
  if (!text) return
  push({ role: 'user', text })
  draft.value = ''
  loading.value = true
  try {
    const res = await agentSay(text)
    push({ role: 'agent', card: res.card })
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function onPickIntent(option) {
  // 模型判不出来时用户手选，直接建执行
  loading.value = true
  try {
    const res = await createExecution({
      task_intent: option.task_intent,
      task_title: option.label,
      goal: '',
      material: '',
    })
    push({
      role: 'agent',
      card: {
        type: 'continue_task',
        say: '开始了。接着往下走吧。',
        data: { execution_id: res.data.id, task_label: option.label, step_count: 1 },
        actions: [
          { label: '下一步', method: 'POST', path: `/api/growth/executions/${res.data.id}/advance`, primary: true },
          { label: '我做完了', method: 'POST', path: `/api/growth/executions/${res.data.id}/complete` },
        ],
        deep_link: `/workbench?id=${res.data.id}`,
      },
    })
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function onAct({ action, index, payload }) {
  // 跳页类动作：有些东西（Creator 的四槽、编排的周视图）在完整页面里做更顺手
  if (action.method === 'GOTO') {
    router.push(action.path)
    return
  }
  if (action.method === 'SKIP') {
    push({ role: 'agent', card: { type: 'idle', say: '好，那先放着。有需要随时说。', data: { examples: [] } } })
    return
  }

  busyIndex.value = index
  loading.value = true
  try {
    const res = await runAction(action, payload)

    // 有些动作的返回本身就该被看见，不能默默吞掉
    if (res && res.blocked) {
      ElMessage.warning('被门禁拦住了：' + res.blocked.join('；'))
    }
    if (res && res.version_candidate_created) {
      ElMessage.success('同类问题已重复出现，生成了版本候选')
    }
    if (res && res.mode === 'decision') {
      // 推进后正好停在关键判断上，直接渲染，不用等下一轮
      push({
        role: 'agent',
        card: {
          type: 'continue_task',
          say: '这里需要你自己判断一下，我不替你选。',
          why: '你这一选会成为这个方法里的一条判断。',
          data: {
            awaiting_decision: true,
            step_index: res.step_index,
            slot_prompt: res.slot_prompt,
            signal: res.signal,
            options: res.options,
            execution_id: currentExecId(action),
          },
          actions: [
            { label: '就这么定', method: 'POST', path: action.path.replace('/advance', '/decide'), primary: true },
          ],
        },
      })
      return
    }

    // 其余一律回到规则推导，让 agent 决定下一张卡
    await refresh()
  } catch (e) {
    // 门禁类错误（409）要把原因逐条摊出来，不能只说失败
    if (e.payload && (e.payload.blocked || e.payload.still_missing)) {
      const list = e.payload.blocked || e.payload.still_missing
      push({
        role: 'agent',
        card: {
          type: 'draft_gate',
          say: '这一步过不去，原因在下面。',
          why: '发布是门禁不是按钮——列出来的每一条都得补掉。',
          data: { still_missing: list },
        },
      })
    } else {
      ElMessage.error(e.message)
    }
  } finally {
    busyIndex.value = -1
    loading.value = false
  }
}

function currentExecId(action) {
  const m = String(action.path).match(/executions\/(\d+)/)
  return m ? Number(m[1]) : null
}

onMounted(refresh)
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f7fa;
}
.stream {
  max-width: 760px;
  margin: 0 auto;
  padding: 26px 20px 40px;
}
.head {
  margin-bottom: 20px;
}
.head h1 {
  margin: 0;
  font-size: 26px;
}
.tag {
  color: #909399;
  font-size: 14px;
  margin: 6px 0 0;
}
.turn {
  margin-bottom: 4px;
}
/* 用户自己说的话靠右，视觉上和 agent 的卡片区分开 */
.mine {
  max-width: 78%;
  margin: 0 0 14px auto;
  background: #409eff;
  color: #fff;
  border-radius: 12px 12px 4px 12px;
  padding: 11px 15px;
  font-size: 15px;
  line-height: 1.7;
}
.thinking {
  color: #c0c4cc;
  font-size: 22px;
  letter-spacing: 3px;
  padding: 4px 0 12px;
}
.composer {
  position: sticky;
  bottom: 0;
  background: #f5f7fa;
  padding: 12px 0 4px;
  border-top: 1px solid #ebeef5;
  margin-top: 10px;
}
.composer-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 10px;
}
.hint {
  font-size: 12px;
  color: #c0c4cc;
  margin-left: auto;
}
</style>
