<!--
  与"虚拟自己"的聊天页。
  访客：可与蒸馏出来的 TA 聊天，发送即由 LLM 扮演 TA 回复。
  主人（?mode=owner）：只能查看对方允许看到的聊天记录，只读。
  聊天记录是否对主人可见，由发起聊天的访客决定（createPersonaChat 时选择）。
-->
<template>
  <div class="page">
    <AppNavbar />
    <main class="main">
      <div class="head">
        <el-button link type="primary" @click="goBack">
          <el-icon><ArrowLeft /></el-icon>&nbsp;返回
        </el-button>
        <h1>{{ title }}</h1>
        <p v-if="isOwner" class="sub">对方允许你查看这段聊天，只读。</p>
        <p v-else class="sub">这个"虚拟的 {{ peerName || 'TA' }}"由 TA 的真实对话蒸馏而成。问问 TA 怎么走过来的吧。</p>
      </div>

      <div class="chat-box" ref="chatBox" v-loading="loading">
        <template v-if="messages.length">
          <div
            v-for="m in messages"
            :key="m.id"
            class="msg"
            :class="m.role === 'user' ? 'mine' : 'theirs'"
          >
            <div class="bubble">{{ m.content }}</div>
            <div class="time">{{ fmtTime(m.created_at) }}</div>
          </div>
        </template>
        <el-empty v-else-if="!loading" description="还没有消息，说点什么吧" />
      </div>

      <div v-if="!isOwner" class="input-bar">
        <el-input
          v-model="inputText"
          type="textarea"
          :rows="2"
          resize="none"
          placeholder="和虚拟的 TA 聊聊…（Enter 发送，Shift+Enter 换行）"
          :disabled="sending"
          @keydown.enter.exact.prevent="send"
        />
        <el-button type="primary" :loading="sending" @click="send">发送</el-button>
      </div>
    </main>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AppNavbar from '../components/AppNavbar.vue'
import { getPersonaMessages, sendPersonaMessage } from '../api/persona'

const route = useRoute()
const router = useRouter()

const chatId = computed(() => route.params.chatId)
const peerName = ref(route.query.name || '')
const isOwner = ref(route.query.mode === 'owner')

const loading = ref(true)
const sending = ref(false)
const messages = ref([])
const inputText = ref('')
const chatBox = ref(null)
let timer = null
let lastPollId = 0

const title = computed(() =>
  isOwner.value ? `与虚拟我的聊天（${peerName.value || '访客'}）` : `与虚拟的 ${peerName.value || 'TA'} 聊天`,
)

function goBack() {
  router.back()
}

function fmtTime(s) {
  if (!s) return ''
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return ''
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

async function scrollBottom() {
  await nextTick()
  if (chatBox.value) chatBox.value.scrollTop = chatBox.value.scrollHeight
}

async function loadAll() {
  try {
    const res = await getPersonaMessages(chatId.value)
    messages.value = res.messages
    await scrollBottom()
  } catch (e) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

// 轮询拉新消息：只把新出现的追加进来，不整屏刷新
async function poll() {
  try {
    const res = await getPersonaMessages(chatId.value)
    const list = res.messages || []
    if (list.length > lastPollId) {
      const fresh = list.filter((m) => m.id > lastPollId)
      if (fresh.length) {
        messages.value = list
        lastPollId = list[list.length - 1].id
        await scrollBottom()
      }
    }
  } catch (e) {
    /* 轮询失败静默，下次再试 */
  }
}

async function send() {
  const text = inputText.value.trim()
  if (!text || sending.value) return
  sending.value = true
  try {
    await sendPersonaMessage(chatId.value, text)
    inputText.value = ''
    await loadAll()
  } catch (e) {
    ElMessage.error(e.message || '发送失败')
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  await loadAll()
  if (messages.value.length) {
    lastPollId = messages.value[messages.value.length - 1].id
  }
  // 访客视角才轮询等 TA 回复；主人只看历史
  if (!isOwner.value) {
    timer = setInterval(poll, 3000)
  }
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f7fa;
}
.main {
  max-width: 760px;
  margin: 0 auto;
  padding: 24px 20px 40px;
}
.head h1 {
  margin: 8px 0 6px;
  font-size: 22px;
}
.sub {
  color: #909399;
  font-size: 13px;
  line-height: 1.7;
  margin: 0 0 16px;
}
.chat-box {
  height: 480px;
  overflow-y: auto;
  background: #fff;
  border-radius: 10px;
  padding: 18px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.msg {
  display: flex;
  flex-direction: column;
  max-width: 78%;
}
.msg.mine {
  align-self: flex-end;
  align-items: flex-end;
}
.msg.theirs {
  align-self: flex-start;
  align-items: flex-start;
}
.bubble {
  font-size: 14px;
  line-height: 1.7;
  padding: 10px 14px;
  border-radius: 10px;
  white-space: pre-wrap;
  word-break: break-word;
}
.mine .bubble {
  background: #409eff;
  color: #fff;
  border-top-right-radius: 2px;
}
.theirs .bubble {
  background: #f0f2f5;
  color: #303133;
  border-top-left-radius: 2px;
}
.time {
  font-size: 11px;
  color: #c0c4cc;
  margin-top: 4px;
}
.input-bar {
  display: flex;
  gap: 12px;
  margin-top: 14px;
  align-items: flex-end;
}
.input-bar .el-button {
  flex-shrink: 0;
}
</style>
