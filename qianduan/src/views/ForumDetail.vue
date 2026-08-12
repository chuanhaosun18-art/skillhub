<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AppNavbar from '../components/AppNavbar.vue'
import { getTopic, createReply, likeTopic, likeReply } from '../api/forum'
import { authState } from '../api/auth'

const route = useRoute()
const router = useRouter()

const topic = ref(null)
const replies = ref([])
const loading = ref(false)
const replyText = ref('')
const replying = ref(false)

function fmtTime(t) {
  if (!t) return ''
  const d = new Date(t)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function load() {
  loading.value = true
  try {
    const data = await getTopic(route.params.id)
    topic.value = data
    replies.value = data.replies || []
  } catch (e) {
    ElMessage.error(e.message || '加载失败')
    router.replace('/forum')
  } finally {
    loading.value = false
  }
}

onMounted(load)

// 点赞 / 取消点赞（target: 'topic' | 'reply'）
async function toggleLike(target, id) {
  if (!authState.token) {
    ElMessage.warning('登录后才能点赞')
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  try {
    const res = target === 'topic' ? await likeTopic(id) : await likeReply(id)
    if (target === 'topic' && topic.value) {
      topic.value.like_count = res.like_count
      topic.value.liked = res.liked
    } else {
      const r = replies.value.find((x) => x.id === id)
      if (r) {
        r.like_count = res.like_count
        r.liked = res.liked
      }
    }
  } catch (e) {
    ElMessage.error(e.message || '操作失败')
  }
}

async function submitReply() {
  const content = replyText.value.trim()
  if (!content) {
    ElMessage.warning('回复内容不能为空')
    return
  }
  if (!authState.token) {
    ElMessage.warning('登录后才能回复')
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  replying.value = true
  try {
    await createReply(topic.value.id, content)
    replyText.value = ''
    ElMessage.success('回复成功')
    await load()
  } catch (e) {
    ElMessage.error(e.message || '回复失败')
  } finally {
    replying.value = false
  }
}
</script>

<template>
  <div class="detail">
    <AppNavbar>
      <div class="back-bar">
        <el-button text @click="router.push('/forum')">
          <el-icon><ArrowLeft /></el-icon>返回论坛
        </el-button>
      </div>
    </AppNavbar>

    <main class="content">
      <el-skeleton :rows="8" animated v-if="loading" />

      <template v-else-if="topic">
        <!-- 帖子主体 -->
        <div class="topic-card">
          <div class="topic-head">
            <div class="topic-title">
              {{ topic.title }}
              <el-tag size="small" :type="topic.category === 'looking_for' ? 'warning' : topic.category === 'experience' ? 'success' : 'info'" effect="plain">
                {{ topic.category_label }}
              </el-tag>
            </div>
            <div class="topic-meta">
              <el-avatar :size="22" :src="topic.avatar || undefined">{{ (topic.username || '匿')[0] }}</el-avatar>
              <span class="meta-user">{{ topic.username }}</span>
              <span class="meta-time">{{ fmtTime(topic.created_at) }}</span>
              <span class="meta-view"><el-icon><View /></el-icon>{{ topic.view_count }} 浏览</span>
            </div>
          </div>
          <div class="topic-body">{{ topic.content || '（没有补充内容）' }}</div>
          <div class="topic-actions">
            <button
              class="like-btn"
              :class="{ liked: topic.liked }"
              @click="toggleLike('topic', topic.id)"
            >
              <el-icon><Pointer /></el-icon>
              <span>{{ topic.like_count ? topic.like_count : '' }}</span>
              <span class="like-text">点赞</span>
            </button>
          </div>
        </div>

        <!-- 回复列表 -->
        <div class="reply-section">
          <div class="reply-title">{{ replies.length ? `${replies.length} 条回复` : '还没有回复，你是第一个' }}</div>
          <div v-if="replies.length" class="reply-list">
            <div class="reply-item" v-for="r in replies" :key="r.id">
              <el-avatar :size="30" :src="r.avatar || undefined">{{ (r.username || '匿')[0] }}</el-avatar>
              <div class="reply-right">
                <div class="reply-meta">
                  <span class="reply-user">{{ r.username }}</span>
                  <span class="reply-time">{{ fmtTime(r.created_at) }}</span>
                  <button
                    class="reply-like"
                    :class="{ liked: r.liked }"
                    @click="toggleLike('reply', r.id)"
                  >
                    <el-icon><Pointer /></el-icon>{{ r.like_count ? r.like_count : '' }}
                  </button>
                </div>
                <div class="reply-content">{{ r.content }}</div>
              </div>
            </div>
          </div>

          <!-- 回复输入 -->
          <div class="reply-box">
            <el-input
              v-model="replyText"
              type="textarea"
              :rows="3"
              maxlength="2000"
              show-word-limit
              :placeholder="authState.token ? '写下你的经验、提醒或线索……' : '登录后即可回复'"
            />
            <div class="reply-actions">
              <el-button type="primary" :loading="replying" @click="submitReply">回复</el-button>
            </div>
          </div>
        </div>
      </template>
    </main>

    <footer class="footer">
      <span>SkillHub © 2026 · 技能发现与分享平台</span>
    </footer>
  </div>
</template>

<style scoped>
.detail {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}

.back-bar {
  width: 100%;
  max-width: 560px;
}

.content {
  flex: 1;
  max-width: 860px;
  width: 100%;
  margin: 0 auto;
  padding: 24px;
}

.topic-card {
  background: #fff;
  border-radius: 12px;
  padding: 28px 32px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  margin-bottom: 20px;
}

.topic-title {
  font-size: 22px;
  font-weight: 700;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}

.topic-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: #909399;
  padding-bottom: 16px;
  border-bottom: 1px solid #f0f2f5;
}

.meta-view {
  display: flex;
  align-items: center;
  gap: 4px;
}

.topic-body {
  margin-top: 18px;
  font-size: 15px;
  color: #303133;
  line-height: 1.9;
  white-space: pre-wrap;
  word-break: break-word;
}

.topic-actions {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid #f0f2f5;
  display: flex;
  gap: 12px;
}

.like-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border: 1px solid #e4e7ed;
  background: #fff;
  color: #606266;
  font-size: 13px;
  border-radius: 16px;
  padding: 5px 16px;
  cursor: pointer;
  transition: all 0.2s;
}

.like-btn:hover {
  color: #f56c6c;
  border-color: #f56c6c;
}

.like-btn.liked {
  color: #f56c6c;
  border-color: #f56c6c;
  background: #fef0f0;
}

.like-text {
  margin-left: 2px;
}

.reply-section {
  background: #fff;
  border-radius: 12px;
  padding: 24px 32px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
}

.reply-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 18px;
}

.reply-list {
  display: flex;
  flex-direction: column;
  gap: 18px;
  margin-bottom: 24px;
}

.reply-item {
  display: flex;
  gap: 12px;
}

.reply-right {
  flex: 1;
  min-width: 0;
}

.reply-meta {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 4px;
}

.reply-user {
  font-size: 13px;
  font-weight: 600;
  color: #606266;
}

.reply-time {
  font-size: 12px;
  color: #c0c4cc;
}

.reply-like {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: none;
  background: none;
  font-size: 12px;
  color: #909399;
  cursor: pointer;
  padding: 2px 8px;
  border-radius: 12px;
  transition: all 0.2s;
}

.reply-like:hover {
  color: #f56c6c;
  background: #fef0f0;
}

.reply-like.liked {
  color: #f56c6c;
}

.reply-content {
  font-size: 14px;
  color: #303133;
  line-height: 1.8;
  white-space: pre-wrap;
  word-break: break-word;
  background: #fafbfc;
  border-radius: 8px;
  padding: 10px 14px;
}

.reply-box {
  border-top: 1px solid #f0f2f5;
  padding-top: 18px;
}

.reply-actions {
  margin-top: 10px;
  text-align: right;
}

.footer {
  text-align: center;
  padding: 24px;
  color: #c0c4cc;
  font-size: 13px;
}
</style>
