<!--
  Skill Creator（PRD F5）：把一次真实执行变成可安装的 Skill。
  设计要点：
  - 用户只做确认，不做撰写。四槽里的候选判断都是从轨迹抽出来的。
  - 每条候选判断都显示「来自第 N 步」。没有来源的候选在后端就被丢掉了，不会出现在这里。
  - 蒸馏度六项实时显示。不达标的出口是「存成经验笔记」，界面不出现失败/不合格字样。
  - 适用边界是硬性项，不接受折中。
-->
<template>
  <div class="creator" v-loading="loading">
    <template v-if="draft">
      <header class="head">
        <div>
          <div class="crumb">从执行轨迹固化 · 来源第 {{ sourceSteps }} 步轨迹</div>
          <el-input v-model="skillName" class="name-input" placeholder="这个方法叫什么" />
        </div>
        <div class="score-box" :class="{ ok: dist.publishable }">
          <div class="score-num">{{ (dist.score * 100).toFixed(0) }}</div>
          <div class="score-label">经验蒸馏度</div>
          <div class="score-line">发布线 {{ (dist.threshold * 100).toFixed(0) }}</div>
        </div>
      </header>

      <!-- 蒸馏度六项 -->
      <section class="card">
        <h3>还缺什么</h3>
        <div class="dims">
          <div
            v-for="(v, k) in dist.detail"
            :key="k"
            class="dim"
            :class="dimClass(v)"
          >
            <span class="dim-icon">{{ v >= 1 ? '✓' : v > 0 ? '△' : '✕' }}</span>
            <span class="dim-name">{{ DIMENSION_LABELS[k] || k }}</span>
          </div>
        </div>
        <ul v-if="dist.still_missing && dist.still_missing.length" class="missing">
          <li v-for="(m, i) in dist.still_missing" :key="i">{{ m }}</li>
        </ul>
        <p v-else class="all-good">六项都达标了，可以生成 Skill 包。</p>
        <p v-if="extractStats" class="extract-note">
          从轨迹抽出 {{ extractStats.kept }} 条判断，丢掉 {{ extractStats.dropped }} 条。{{ extractStats.note }}
        </p>
      </section>

      <!-- 四槽确认 -->
      <section class="card">
        <h3>关键判断（这是别人真正需要的东西）</h3>
        <p class="sub">
          方法里最值钱的不是步骤，是这些岔路口上的判断。每条都标了它来自轨迹第几步，可以核对。
        </p>

        <div v-for="slot in draft.slots" :key="slot.slot" class="slot">
          <div class="slot-head">
            <span class="slot-prompt">{{ slot.prompt }}</span>
            <el-tag :type="slot.filled ? 'success' : 'info'" size="small">
              {{ slot.filled ? slot.decisions.length + ' 条' : '未填' }}
            </el-tag>
          </div>

          <div v-for="d in slot.decisions" :key="d.id" class="dec">
            <div class="dec-body">
              <div class="dec-line">
                当<b>{{ d.trigger_signal }}</b>时，{{ d.judgment }}
              </div>
              <div class="dec-meta">
                适用：{{ d.scope }} · <span class="src">来自第 {{ d.source_step_index }} 步</span>
              </div>
            </div>
            <el-button link type="danger" size="small" @click="removeDecision(d.id)">删除</el-button>
          </div>

          <div v-if="adding === slot.slot" class="add-form">
            <el-input v-model="form.trigger_signal" placeholder="出现什么信号" size="small" />
            <el-input v-model="form.judgment" placeholder="就要怎么做" size="small" style="margin-top: 6px" />
            <el-input v-model="form.scope" placeholder="在什么场景下成立" size="small" style="margin-top: 6px" />
            <el-input
              v-model.number="form.source_step_index"
              placeholder="来自轨迹第几步"
              size="small"
              style="margin-top: 6px"
            />
            <div style="margin-top: 8px">
              <el-button size="small" type="primary" @click="addDecision(slot.slot)">保存</el-button>
              <el-button size="small" text @click="adding = ''">取消</el-button>
            </div>
          </div>
          <el-button v-else link type="primary" size="small" @click="startAdd(slot.slot)">
            + 补一条
          </el-button>
        </div>
      </section>

      <!-- 适用边界：硬性项 -->
      <section class="card boundary-card">
        <h3>适用边界<span class="required">硬性要求</span></h3>
        <p class="sub">
          不写清边界的方法是危险的——它会在不该用的地方被用。这一项不达标不能发布，不接受折中。
        </p>
        <label class="lbl">什么情况下不适用（每行一条）</label>
        <el-input v-model="notApplicable" type="textarea" :rows="3" />
        <label class="lbl">出现什么情况必须交回给人（每行一条）</label>
        <el-input v-model="handoffTrigger" type="textarea" :rows="3" />
        <label class="lbl">降级路径</label>
        <el-input v-model="fallbackPath" placeholder="做不下去时退到哪一步" />
      </section>

      <!-- description：流通的瓶颈 -->
      <section class="card">
        <h3>description（决定它什么时候被找到）</h3>
        <p class="sub">
          路由第一跳只看名字和这段描述。所以这里要用用户真正会说的话，不是书面语。
          下面是站内真实提问，点一条抄进去。
        </p>
        <el-input v-model="description" type="textarea" :rows="3" />
        <div class="corpus">
          <el-tag
            v-for="(u, i) in draft.corpus_candidates || []"
            :key="i"
            class="corpus-tag"
            @click="useCorpus(u)"
          >
            {{ u }}
          </el-tag>
        </div>
      </section>

      <!-- 操作区 -->
      <section class="actions">
        <el-button :loading="saving" @click="save">保存草稿</el-button>
        <el-button
          type="primary"
          :loading="generating"
          :disabled="!dist.publishable"
          @click="generate"
        >
          生成 Skill 包
        </el-button>
        <el-button v-if="!dist.publishable" text @click="downgrade">先存成经验笔记</el-button>
        <span v-if="!dist.publishable" class="disabled-why">
          还差：{{ (dist.still_missing || []).join('；') }}
        </span>
      </section>

      <!-- 生成结果 -->
      <section v-if="generated" class="card generated">
        <h3>Skill 包已生成</h3>
        <p class="sub">
          自安装校验已通过（SKILL.md 唯一、evals 非空）。下一步跑发布前四问。
        </p>
        <ul class="files">
          <li v-for="f in generated.files" :key="f"><code>{{ f }}</code></li>
        </ul>
        <el-button type="primary" @click="goGate">去跑发布前四问</el-button>
      </section>
    </template>

    <el-empty v-else-if="!loading" description="没有找到草稿" />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  DIMENSION_LABELS,
  getDraft,
  updateDraft,
  upsertDecision,
  deleteDecision,
  downgradeToInsight,
  generateFolder,
} from '../api/growth'

const route = useRoute()
const router = useRouter()
const versionId = Number(route.query.v)

const loading = ref(true)
const saving = ref(false)
const generating = ref(false)
const draft = ref(null)
const generated = ref(null)
const extractStats = ref(null)

const skillName = ref('')
const description = ref('')
const notApplicable = ref('')
const handoffTrigger = ref('')
const fallbackPath = ref('')

const adding = ref('')
const form = reactive({ trigger_signal: '', judgment: '', scope: '', source_step_index: 0 })

const dist = computed(() => draft.value?.distillation || { score: 0, detail: {}, threshold: 0.75 })
const sourceSteps = computed(() => draft.value?.source_execution?.step_count || 0)

function dimClass(v) {
  if (v >= 1) return 'ok'
  if (v > 0) return 'partial'
  return 'bad'
}

async function load() {
  loading.value = true
  try {
    const res = await getDraft(versionId)
    draft.value = res
    if (res.extract_stats) extractStats.value = res.extract_stats
    skillName.value = res.skill_name || ''
    description.value = res.version?.description || ''
    const b = safeParse(res.version?.boundary, {})
    notApplicable.value = (b.not_applicable || []).join('\n')
    handoffTrigger.value = (b.handoff_trigger || []).join('\n')
    fallbackPath.value = b.fallback_path || ''
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function safeParse(s, def) {
  if (!s) return def
  if (typeof s === 'object') return s
  try {
    return JSON.parse(s)
  } catch (e) {
    return def
  }
}

function toLines(s) {
  return String(s || '')
    .split('\n')
    .map((x) => x.trim())
    .filter(Boolean)
}

async function save() {
  saving.value = true
  try {
    const res = await updateDraft(versionId, {
      name: skillName.value,
      description: description.value,
      boundary: {
        not_applicable: toLines(notApplicable.value),
        handoff_trigger: toLines(handoffTrigger.value),
        fallback_path: fallbackPath.value,
      },
    })
    draft.value = res
    ElMessage.success('已保存')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

function startAdd(slot) {
  adding.value = slot
  form.trigger_signal = ''
  form.judgment = ''
  form.scope = ''
  form.source_step_index = 0
}

async function addDecision(slot) {
  try {
    const res = await upsertDecision(versionId, { ...form, slot })
    draft.value = res
    adding.value = ''
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function removeDecision(id) {
  try {
    const res = await deleteDecision(id)
    draft.value = res
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function useCorpus(u) {
  description.value = description.value ? description.value + '；' + u : u
}

async function generate() {
  generating.value = true
  try {
    await save()
    const res = await generateFolder(versionId)
    generated.value = res
    ElMessage.success('Skill 包已生成')
  } catch (e) {
    if (e.payload && e.payload.still_missing) {
      ElMessage.warning(e.payload.still_missing.join('；'))
    } else {
      ElMessage.error(e.message)
    }
  } finally {
    generating.value = false
  }
}

async function downgrade() {
  try {
    await ElMessageBox.confirm(
      '这次先存成经验笔记。缺的几项补上之后随时可以再来一次，笔记不会丢。',
      '存成经验笔记',
      { confirmButtonText: '存起来', cancelButtonText: '再改改' }
    )
  } catch (e) {
    return
  }
  try {
    const res = await downgradeToInsight(versionId)
    ElMessage.success(res.message)
    router.push('/profile')
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function goGate() {
  router.push({ path: '/gate', query: { skill: generated.value.skill_id } })
}

onMounted(load)
</script>

<style scoped>
.creator {
  max-width: 900px;
  margin: 0 auto;
  padding: 24px 20px 60px;
}
.head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
  margin-bottom: 18px;
}
.crumb {
  font-size: 12px;
  color: #909399;
  margin-bottom: 6px;
}
.name-input {
  width: 460px;
}
.name-input :deep(input) {
  font-size: 20px;
  font-weight: 600;
  border: none;
  padding-left: 0;
}
.score-box {
  text-align: center;
  border: 2px solid #e6a23c;
  border-radius: 10px;
  padding: 10px 18px;
  min-width: 110px;
}
.score-box.ok {
  border-color: #67c23a;
}
.score-num {
  font-size: 30px;
  font-weight: 700;
  line-height: 1;
}
.score-label {
  font-size: 12px;
  color: #606266;
  margin-top: 4px;
}
.score-line {
  font-size: 11px;
  color: #909399;
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
  font-size: 16px;
}
.sub {
  color: #909399;
  font-size: 13px;
  line-height: 1.7;
  margin: 0 0 14px;
}
.dims {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.dim {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 16px;
  font-size: 13px;
  border: 1px solid #dcdfe6;
}
.dim.ok {
  border-color: #67c23a;
  color: #67c23a;
  background: #f0f9eb;
}
.dim.partial {
  border-color: #e6a23c;
  color: #e6a23c;
  background: #fdf6ec;
}
.dim.bad {
  border-color: #f56c6c;
  color: #f56c6c;
  background: #fef0f0;
}
.missing {
  margin: 14px 0 0;
  padding-left: 20px;
  color: #e6a23c;
  font-size: 13px;
  line-height: 1.9;
}
.all-good {
  color: #67c23a;
  font-size: 13px;
  margin: 12px 0 0;
}
.extract-note {
  font-size: 12px;
  color: #909399;
  margin: 10px 0 0;
}
.slot {
  border-top: 1px solid #f0f0f0;
  padding: 14px 0;
}
.slot-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.slot-prompt {
  font-size: 14px;
  font-weight: 600;
}
.dec {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  background: #fafafa;
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 8px;
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
}
.add-form {
  background: #f7fbff;
  border-radius: 8px;
  padding: 12px;
  margin: 8px 0;
}
.boundary-card {
  border-color: #f56c6c;
}
.required {
  font-size: 11px;
  color: #f56c6c;
  border: 1px solid #f56c6c;
  border-radius: 4px;
  padding: 1px 6px;
  margin-left: 8px;
  vertical-align: middle;
}
.lbl {
  display: block;
  font-size: 13px;
  color: #606266;
  margin: 12px 0 6px;
}
.corpus {
  margin-top: 10px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.corpus-tag {
  cursor: pointer;
}
.actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.disabled-why {
  font-size: 12px;
  color: #e6a23c;
}
.generated {
  border-color: #67c23a;
}
.files {
  margin: 0 0 14px;
  padding-left: 20px;
  font-size: 13px;
  line-height: 1.9;
}
</style>
