<!--
  看别人的成长身份。只显示对方公开的部分——未公开的段落会明确写「未公开」，
  而不是假装那里什么都没有。这是「前路关系」的落点：我连接你，不是因为兴趣相同，
  而是因为你走过我正在走的路。
-->
<template>
  <div class="page">
    <AppNavbar />
    <main class="main">
      <div class="head">
        <h1>成长身份</h1>
        <p class="sub">
          这里没有粉丝数，也没有排名。你看到的是这个人真实走过的路，
          以及他沉淀下来的、你可以直接用的方法。
        </p>
      </div>
      <section class="card">
        <GrowthPath :user-id="userId" />
      </section>

      <!-- 联系 TA：在线聊天 或 与"虚拟自己"聊天（TA 走过同样路，可先与虚拟的 TA 聊聊） -->
      <section class="card contact-card">
        <h2 class="contact-title">
          联系 {{ username || 'TA' }}
          <span class="contact-note">TA 能帮你，也能被你帮。选择一种方式开始。</span>
        </h2>
        <div class="contact-actions">
          <el-button size="large" type="primary" plain round :loading="starting" @click="startDirectChat">
            <el-icon style="margin-right: 6px"><ChatDotRound /></el-icon>在线聊天
          </el-button>
          <el-button
            v-if="persona.chat_enabled"
            size="large" type="success" plain round :loading="starting" @click="startPersonaChat"
          >
            <el-icon style="margin-right: 6px"><MagicStick /></el-icon>与虚拟的 TA 聊天
          </el-button>
          <div v-else class="persona-off-tip">TA 还没开启虚拟自己，先在线聊聊吧</div>
        </div>
        <div v-if="persona.chat_enabled && persona.summary" class="persona-summary">
          <div class="persona-summary-label">TA 的虚拟自己是什么样：</div>
          <div class="persona-summary-text">{{ persona.summary }}</div>
          <div class="persona-summary-meta">已被 {{ persona.chat_count }} 位访客聊过</div>
        </div>
      </section>

      <!-- 是否允许对方查看聊天记录：主动聊天的人决定 -->
      <el-dialog
        v-model="personaDialog"
        title="与虚拟的 TA 聊天"
        width="420px"
      >
        <p class="dialog-desc">
          这个"虚拟自己"由 TA 的真实对话蒸馏而成。开始前请选择：
        </p>
        <el-checkbox v-model="allowOwnerView">
          允许 TA 查看本次聊天记录
        </el-checkbox>
        <p class="dialog-hint">不勾选则只有你能看到这段对话，对方完全不知道。</p>
        <template #footer>
          <el-button @click="personaDialog = false">取消</el-button>
          <el-button type="primary" :loading="starting" @click="confirmPersonaChat">开始聊天</el-button>
        </template>
      </el-dialog>
    </main>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AppNavbar from '../components/AppNavbar.vue'
import GrowthPath from '../components/GrowthPath.vue'
import { isLoggedIn } from '../api/auth'
import { getUserGrowthProfile } from '../api/growth'
import { getPublicPersona, createPersonaChat } from '../api/persona'
import { createDirectChat } from '../api/chat'

const route = useRoute()
const router = useRouter()
const userId = computed(() => route.params.id)

const username = ref('')
const persona = ref({ chat_enabled: 0 })
const starting = ref(false)
const personaDialog = ref(false)
const allowOwnerView = ref(false)

function requireLogin() {
  if (isLoggedIn()) return true
  router.push({ path: '/login', query: { redirect: route.fullPath } })
  return false
}

async function startDirectChat() {
  if (!requireLogin()) return
  starting.value = true
  try {
    const res = await createDirectChat(Number(userId.value))
    router.push(`/chat/${res.chat_id}`)
  } catch (e) {
    ElMessage.error(e.message || '发起聊天失败')
    starting.value = false
  }
}

async function startPersonaChat() {
  if (!requireLogin()) return
  personaDialog.value = true
}

async function confirmPersonaChat() {
  starting.value = true
  try {
    const res = await createPersonaChat(userId.value, allowOwnerView.value)
    personaDialog.value = false
    router.push({ path: `/persona-chat/${res.chat_id}`, query: { name: username.value } })
  } catch (e) {
    ElMessage.error(e.message || '开始聊天失败')
    starting.value = false
  }
}

onMounted(async () => {
  try {
    const g = await getUserGrowthProfile(userId.value)
    username.value = g.username || ''
  } catch (e) {
    /* 成长资料缺失不阻塞聊天入口 */
  }
  try {
    persona.value = await getPublicPersona(userId.value)
  } catch (e) {
    /* ignore */
  }
})
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f7fa;
}
.main {
  max-width: 860px;
  margin: 0 auto;
  padding: 28px 20px 60px;
}
.head h1 {
  margin: 0 0 6px;
  font-size: 24px;
}
.sub {
  color: #909399;
  font-size: 13px;
  line-height: 1.8;
  margin: 0 0 18px;
  max-width: 620px;
}
.card {
  background: #fff;
  border-radius: 10px;
  padding: 24px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  margin-bottom: 16px;
}
.contact-card {
  border-top: 3px solid #67c23a;
}
.contact-title {
  margin: 0 0 16px;
  font-size: 20px;
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}
.contact-note {
  font-size: 12px;
  font-weight: 400;
  color: #909399;
}
.contact-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.persona-off-tip {
  font-size: 13px;
  color: #c0c4cc;
}
.persona-summary {
  margin-top: 16px;
  background: #f6fdf8;
  border: 1px solid #e6f6ec;
  border-radius: 8px;
  padding: 12px 14px;
}
.persona-summary-label {
  font-size: 12px;
  color: #67c23a;
  font-weight: 600;
}
.persona-summary-text {
  font-size: 13px;
  color: #606266;
  line-height: 1.7;
  margin: 6px 0;
}
.persona-summary-meta {
  font-size: 12px;
  color: #c0c4cc;
}
.dialog-desc {
  margin: 0 0 12px;
  font-size: 13px;
  color: #606266;
  line-height: 1.7;
}
.dialog-hint {
  margin: 8px 0 0;
  font-size: 12px;
  color: #c0c4cc;
}
</style>
