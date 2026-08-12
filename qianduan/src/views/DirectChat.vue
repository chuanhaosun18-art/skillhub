<!--
  在线聊天：一对一实时聊天（轮询实现）。
  /chat        —— 会话列表
  /chat/:id    —— 某个会话的对话界面
-->
<template>
  <div class="page">
    <AppNavbar />
    <main class="main">
      <template v-if="!route.params.id">
        <div class="head">
          <h1>消息</h1>
          <p class="sub">和你走过同一条路的人，聊一聊。</p>
        </div>
        <section class="card">
          <div v-if="!conversations.length" class="empty-tip">
            还没有会话。去别人的「成长身份」页面点「在线聊天」发起。
          </div>
          <div
            v-for="c in conversations"
            :key="c.chat_id"
            class="conv-row"
            @click="router.push(`/chat/${c.chat_id}`)"
          >
            <div class="conv-name">{{ c.peer }}</div>
            <div class="conv-last">{{ c.last_msg || '（还没有消息）' }}</div>
            <div class="conv-time">{{ fmtTime(c.last_at) }}</div>
          </div>
        </section>
      </template>

      <template v-else>
        <div class="head">
          <el-button link type="primary" @click="router.push('/chat')">
            <el-icon><ArrowLeft /></el-icon>&nbsp;返回
          </el-button>
          <h1>与 {{ peerName }} 的对话</h1>
        </div>
        <div class="chat-box" ref="chatBox" v-loading="loading">
          <div v-for="m in messages" :key="m.id" class="msg" :class="m.sender_id === myId ? 'mine' : 'theirs'">
            <div class="bubble">{{ m.content }}</div>
            <div class="time">{{ fmtTime(m.created_at) }}</div>
          </div>
        </div>
        <div class="input-bar">
          <el-input
            v-model="inputText"
            type="textarea"
            :rows="2"
            resize="none"
            placeholder="说点什么…（Enter 发送，Shift+Enter 换行）"
            :disabled="sending"
            @keydown.enter.exact.prevent="send"
          />
          <el-button type="primary" :loading="sending" @click="send">发送</el-button>
        </div>
      </template>
    </main>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AppNavbar from '../components/AppNavbar.vue'
import { authState } from '../api/auth'
import { listDirectChats, getDirectMessages, sendDirectMessage } from '../api/chat'

const route = useRoute()
const router = useRouter()

const conversations = ref([])
const messages = ref([])
const loading = ref(true)
const sending = ref(false)
const inputText = ref('')
const chatBox = ref(null)
let timer = null

const chatId = computed(() => route.params.id)
const myId = computed(() => authState.user?.id)
const peerName = computed(() => route.query.name || '对方')

function fmtTime(s) {
  if (!s) return ''
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n) => String(n).padStart(2, '0')
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function scrollBottom() {
  await nextTick()
  if (chatBox.value) chatBox.value.scrollTop = chatBox.value.scrollHeight
}

async function loadList() {
  try {
    conversations.value = await listDirectChats()
  } catch (e) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

let lastMsgId = 0

async function loadConversation() {
  try {
    const list = await getDirectMessages(chatId.value, 0)
    messages.value = list
    if (list.length) lastMsgId = list[list.length - 1].id
    await scrollBottom()
  } catch (e) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

// 轮询拉新消息
async function poll() {
  try {
    const list = await getDirectMessages(chatId.value, lastMsgId)
    if (list.length) {
      messages.value = messages.value.concat(list)
      lastMsgId = list[list.length - 1].id
      await scrollBottom()
    }
  } catch (e) {
    /* 轮询失败静默 */
  }
}

async function send() {
  const text = inputText.value.trim()
  if (!text || sending.value) return
  sending.value = true
  try {
    await sendDirectMessage(chatId.value, text)
    inputText.value = ''
    await poll()
  } catch (e) {
    ElMessage.error(e.message || '发送失败')
  } finally {
    sending.value = false
  }
}

onMounted(async () => {
  if (!chatId.value) {
    await loadList()
  } else {
    await loadConversation()
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
  margin: 0 0 16px;
}
.card {
  background: #fff;
  border-radius: 10px;
  padding: 8px 18px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
}
.empty-tip {
  font-size: 13px;
  color: #909399;
  padding: 24px 0;
  text-align: center;
}
.conv-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 0;
  border-bottom: 1px solid #f0f2f5;
  cursor: pointer;
}
.conv-row:last-child {
  border-bottom: none;
}
.conv-row:hover .conv-name {
  color: #409eff;
}
.conv-name {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  flex-shrink: 0;
  min-width: 80px;
}
.conv-last {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: #909399;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.conv-time {
  font-size: 12px;
  color: #c0c4cc;
  flex-shrink: 0;
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
