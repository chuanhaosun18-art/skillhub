<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import AppNavbar from '../components/AppNavbar.vue'
import { listTopics, createTopic, FORUM_CATEGORIES } from '../api/forum'
import { authState } from '../api/auth'

const route = useRoute()
const router = useRouter()

const topics = ref([])
const loading = ref(false)
const keyword = ref('')
const activeCategory = ref('全部')

// 发帖对话框
const dialogVisible = ref(false)
const posting = ref(false)
const form = ref({ title: '', category: 'help', content: '' })

function fmtTime(t) {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  const pad = (n) => String(n).padStart(2, '0')
  if (sameDay) return `${pad(d.getHours())}:${pad(d.getMinutes())}`
  if (d.getFullYear() === now.getFullYear()) return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

async function load() {
  loading.value = true
  try {
    const kw = keyword.value.trim()
    topics.value = await listTopics({ keyword: kw, category: activeCategory.value })
  } catch (e) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  const kw = keyword.value.trim()
  router.replace({ path: '/forum', query: kw ? { q: kw } : {} })
  load()
}

function selectCategory(cat) {
  activeCategory.value = cat
  load()
}

function goDetail(id) {
  router.push(`/forum/${id}`)
}

function openCreate() {
  if (!authState.token) {
    ElMessage.warning('登录后才能发帖')
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  form.value.title = keyword.value.trim()
  form.value.category = 'help'
  form.value.content = ''
  dialogVisible.value = true
}

async function submitTopic() {
  const title = form.value.title.trim()
  if (title.length < 3) {
    ElMessage.warning('标题太短了，多说几个字让大家知道你想聊什么')
    return
  }
  posting.value = true
  try {
    const { id } = await createTopic(form.value)
    ElMessage.success('发布成功')
    dialogVisible.value = false
    router.push(`/forum/${id}`)
  } catch (e) {
    ElMessage.error(e.message || '发布失败')
  } finally {
    posting.value = false
  }
}

// 从搜索结果页跳来：q 预填搜索框；ask=1 时直接打开发帖框
onMounted(() => {
  if (route.query.q) keyword.value = route.query.q
  load()
  if (route.query.ask === '1' && authState.token) {
    openCreate()
  }
})

watch(
  () => route.query.q,
  (q) => {
    if (q !== undefined && q !== keyword.value) keyword.value = q || ''
  }
)
</script>

<template>
  <div class="forum">
    <AppNavbar>
      <div class="search-bar">
        <el-input
          v-model="keyword"
          placeholder="在论坛里找一找，或直接发帖问"
          clearable
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-button type="primary" @click="handleSearch">搜索</el-button>
      </div>
    </AppNavbar>

    <main class="content">
      <!-- 论坛定位说明 -->
      <div class="intro">
        <h1 class="intro-title">论坛 · 没做成 Skill 的东西，在这里聊</h1>
        <p class="intro-sub">
          有些经验不值得做成 Skill，但值得被听见：一句话的提醒、一次踩坑、一个还没人做成能力的需求。
          这里不卖方法，只连接人。
        </p>
      </div>

      <!-- 工具栏 -->
      <div class="toolbar">
        <div class="cats">
          <span
            v-for="cat in ['全部', ...FORUM_CATEGORIES.map((c) => c.label)]"
            :key="cat"
            class="cat"
            :class="{ active: activeCategory === cat }"
            @click="selectCategory(cat)"
          >{{ cat }}</span>
        </div>
        <el-button type="primary" round @click="openCreate">
          <el-icon style="margin-right: 4px"><EditPen /></el-icon>发帖
        </el-button>
      </div>

      <!-- 列表 -->
      <el-skeleton :rows="5" animated v-if="loading" />

      <template v-else>
        <div v-if="topics.length" class="topic-list">
          <div class="topic-item" v-for="t in topics" :key="t.id" @click="goDetail(t.id)">
            <div class="topic-main">
              <div class="topic-title">
                {{ t.title }}
                <el-tag size="small" :type="t.category === 'looking_for' ? 'warning' : t.category === 'experience' ? 'success' : 'info'" effect="plain">
                  {{ t.category_label }}
                </el-tag>
              </div>
              <div class="topic-meta">
                <el-avatar :size="20" :src="t.avatar || undefined">{{ (t.username || '匿')[0] }}</el-avatar>
                <span class="meta-user">{{ t.username }}</span>
                <span class="meta-time">{{ fmtTime(t.created_at) }}</span>
              </div>
            </div>
            <div class="topic-stats">
              <span class="stat"><el-icon><ChatDotRound /></el-icon>{{ t.reply_count }}</span>
              <span class="stat"><el-icon><Pointer /></el-icon>{{ t.like_count }}</span>
              <span class="stat"><el-icon><View /></el-icon>{{ t.view_count }}</span>
            </div>
          </div>
        </div>

        <el-empty v-else description="还没有相关帖子。它可能是个新问题——发一帖，看看有没有人遇到过">
          <el-button type="primary" @click="openCreate">我来发一帖</el-button>
        </el-empty>
      </template>
    </main>

    <!-- 发帖对话框 -->
    <el-dialog v-model="dialogVisible" title="发一帖" width="560px" :close-on-click-modal="false">
      <el-form label-position="top">
        <el-form-item label="分类">
          <el-radio-group v-model="form.category">
            <el-radio-button
              v-for="c in FORUM_CATEGORIES"
              :key="c.value"
              :value="c.value"
            >{{ c.label }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="80" show-word-limit placeholder="一句话说清你想聊什么，例如：实习摸鱼时怎么保持手感" />
        </el-form-item>
        <el-form-item label="内容">
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="6"
            maxlength="5000"
            show-word-limit
            placeholder="想说的细节、背景、尝试过什么。不需要写成方法——聊得开就好"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="posting" @click="submitTopic">发布</el-button>
      </template>
    </el-dialog>

    <footer class="footer">
      <span>SkillHub © 2026 · 技能发现与分享平台</span>
    </footer>
  </div>
</template>

<style scoped>
.forum {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}

.search-bar {
  display: flex;
  gap: 8px;
  width: 100%;
  max-width: 560px;
}

.search-bar :deep(.el-input__wrapper) {
  border-radius: 20px;
}

.content {
  flex: 1;
  max-width: 900px;
  width: 100%;
  margin: 0 auto;
  padding: 32px 24px;
}

.intro {
  margin-bottom: 20px;
}

.intro-title {
  font-size: 24px;
  font-weight: 700;
  color: #303133;
  margin-bottom: 8px;
}

.intro-sub {
  font-size: 14px;
  color: #909399;
  line-height: 1.8;
  max-width: 640px;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 20px;
  padding: 12px 16px;
  background: #fff;
  border-radius: 10px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
}

.cats {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.cat {
  padding: 5px 14px;
  border-radius: 16px;
  font-size: 13px;
  color: #606266;
  cursor: pointer;
  background: #f5f7fa;
  transition: all 0.2s;
}

.cat:hover {
  color: #409eff;
}

.cat.active {
  background: #409eff;
  color: #fff;
}

.topic-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.topic-item {
  background: #fff;
  border-radius: 10px;
  padding: 18px 22px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.topic-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.08);
}

.topic-main {
  min-width: 0;
  flex: 1;
}

.topic-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topic-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #909399;
}

.topic-stats {
  display: flex;
  gap: 16px;
  flex-shrink: 0;
}

.stat {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #909399;
}

.footer {
  text-align: center;
  padding: 24px;
  color: #c0c4cc;
  font-size: 13px;
}
</style>
