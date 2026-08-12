<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessageBox, ElMessage } from 'element-plus'
import { authState, clearAuth } from '../api/auth'
import { getUnreadCount, fetchNotifications, markAllRead, markRead } from '../api/notifications'

const router = useRouter()

// ---------- 消息通知：铃铛角标 + 下拉列表 + 轮询未读数 ----------
const unreadCount = ref(0)
const notifList = ref([])
const notifVisible = ref(false)
const notifLoading = ref(false)
let notifTimer = null

const notifTypeMeta = {
  message: { icon: 'ChatDotRound', text: '私信', color: '#409eff' },
  review: { icon: 'Star', text: '评价', color: '#e6a23c' },
  issue: { icon: 'Warning', text: '改进意见', color: '#f56c6c' },
}

async function pollUnread() {
  if (!authState.token) return
  try {
    unreadCount.value = await getUnreadCount()
  } catch (e) {
    /* 网络/后端未就绪时静默 */
  }
}

async function openNotif() {
  if (!authState.token) return
  notifVisible.value = true
  notifLoading.value = true
  try {
    notifList.value = await fetchNotifications()
  } catch (e) {
    /* ignore */
  } finally {
    notifLoading.value = false
  }
}

async function notifGo(n) {
  // 点击单条：标记已读并跳转
  if (!n.is_read) {
    markRead(n.id).catch(() => {})
    n.is_read = 1
    unreadCount.value = Math.max(0, unreadCount.value - 1)
  }
  notifVisible.value = false
  if (n.type === 'message') {
    router.push({ path: `/chat/${n.related_id}`, query: { name: n.actor_name } })
  } else {
    router.push(`/skill/${n.related_id}`)
  }
}

async function readAll() {
  try {
    await markAllRead()
  } catch (e) {
    /* ignore */
  }
  notifList.value.forEach((n) => (n.is_read = 1))
  unreadCount.value = 0
}

function fmtTime(t) {
  if (!t) return ''
  const d = new Date(t)
  if (isNaN(d.getTime())) return ''
  const diff = Date.now() - d.getTime()
  if (diff < 60 * 1000) return '刚刚'
  if (diff < 60 * 60 * 1000) return Math.floor(diff / 60000) + ' 分钟前'
  if (diff < 24 * 60 * 60 * 1000) return Math.floor(diff / 3600000) + ' 小时前'
  const p = (x) => String(x).padStart(2, '0')
  return `${d.getMonth() + 1}-${d.getDate()} ${p(d.getHours())}:${p(d.getMinutes())}`
}

onMounted(() => {
  pollUnread()
  notifTimer = setInterval(pollUnread, 15000)
})

onUnmounted(() => {
  if (notifTimer) clearInterval(notifTimer)
})

function goLogin() {
  router.push('/login')
}

function goPublish() {
  if (!authState.token) {
    ElMessage.warning('请先登录后再发布技能')
    router.push('/login')
    return
  }
  router.push('/publish')
}

function goProfile() {
  router.push('/profile')
}

// 「我要成长」：用户用一句人话说清卡在哪，系统给下一步并路由能力
function goGrow() {
  if (!authState.token) {
    ElMessage.warning('登录后才能记录你的成长轨迹')
    router.push('/login')
    return
  }
  router.push('/grow')
}

function goWorkbench() {
  router.push('/workbench')
}

async function logout() {
  try {
    await ElMessageBox.confirm('确定退出登录吗？', '提示', { type: 'warning' })
  } catch (e) {
    return
  }
  clearAuth()
  ElMessage.success('已退出登录')
  router.push('/')
}
</script>

<template>
  <header class="navbar">
    <div class="navbar-inner">
      <div class="logo" @click="router.push('/')">
        <el-icon :size="24" color="#409eff"><Connection /></el-icon>
        <span class="logo-text">SkillHub</span>
      </div>

      <!-- 中间区域：调用方通过 slot 传入搜索框等 -->
      <div class="navbar-center">
        <slot />
      </div>

      <div class="nav-actions">
        <!-- 主入口是「我要成长」而不是「发布技能」：前台卖下一步，后台跑 Skill -->
        <el-button text class="grow-entry" @click="goGrow">我要成长</el-button>
        <el-button text class="grow-entry" @click="router.push('/forum')">论坛</el-button>
        <el-button v-if="authState.user" text @click="goWorkbench">我的任务</el-button>
        <template v-if="authState.user">
          <!-- 消息通知铃铛：角标显示未读数 -->
          <el-popover
            v-model:visible="notifVisible"
            placement="bottom-end"
            :width="340"
            trigger="click"
            popper-class="notif-popper"
            @show="openNotif"
          >
            <template #reference>
              <el-badge :value="unreadCount" :hidden="unreadCount === 0" :max="99">
                <el-button text class="notif-bell">
                  <el-icon :size="20"><Bell /></el-icon>
                </el-button>
              </el-badge>
            </template>

            <div class="notif-panel">
              <div class="notif-head">
                <span class="notif-title">消息通知</span>
                <el-button link type="primary" size="small" @click="readAll">全部已读</el-button>
              </div>
              <div v-if="notifLoading" class="notif-empty">加载中...</div>
              <div v-else-if="notifList.length === 0" class="notif-empty">暂无通知</div>
              <div v-else class="notif-list">
                <div
                  v-for="n in notifList"
                  :key="n.id"
                  class="notif-item"
                  :class="{ 'notif-item-unread': !n.is_read }"
                  @click="notifGo(n)"
                >
                  <el-icon :size="16" :color="(notifTypeMeta[n.type] || {}).color">
                    <component :is="(notifTypeMeta[n.type] || { icon: 'Bell' }).icon" />
                  </el-icon>
                  <div class="notif-body">
                    <div class="notif-text">
                      <span class="notif-actor">{{ n.actor_name || '用户' }}</span>
                      <span class="notif-meta">{{ (notifTypeMeta[n.type] || {}).text || n.type }}</span>
                      <span v-if="n.skill_name" class="notif-skill">《{{ n.skill_name }}》</span>
                    </div>
                    <div class="notif-content">{{ n.content }}</div>
                    <div class="notif-time">{{ fmtTime(n.created_at) }}</div>
                  </div>
                </div>
              </div>
            </div>
          </el-popover>

          <el-button round @click="goPublish">
            <el-icon style="margin-right: 4px"><Plus /></el-icon>上传已有 Skill
          </el-button>
          <el-dropdown trigger="click" @command="(cmd) => (cmd === 'profile' ? goProfile() : logout())">
            <span class="user-entry">
              <el-avatar :size="28" :src="authState.user.avatar || undefined">
                {{ (authState.user.username || 'U')[0] }}
              </el-avatar>
              <span class="username">{{ authState.user.username }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">
                  <el-icon><User /></el-icon>个人中心
                </el-dropdown-item>
                <el-dropdown-item command="logout" divided>
                  <el-icon><SwitchButton /></el-icon>退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
        <template v-else>
          <el-button text @click="goLogin">登录 / 注册</el-button>
          <el-button type="primary" round @click="goGrow">我要成长</el-button>
        </template>
      </div>
    </div>
  </header>
</template>

<style scoped>
.navbar {
  background: #fff;
  box-shadow: 0 1px 6px rgba(0, 0, 0, 0.06);
  position: sticky;
  top: 0;
  z-index: 100;
}

.navbar-inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 12px 24px;
  display: flex;
  align-items: center;
  gap: 24px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  flex-shrink: 0;
}

.logo-text {
  font-size: 20px;
  font-weight: 700;
  color: #303133;
}

.navbar-center {
  flex: 1;
  display: flex;
  justify-content: center;
}

.nav-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.user-entry {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 8px;
  outline: none;
  transition: background 0.2s;
}

.user-entry:hover {
  background: #f5f7fa;
}

.username {
  font-size: 14px;
  color: #303133;
}

.notif-bell {
  padding: 6px;
  margin: 0 4px;
}

.notif-panel {
  max-height: 420px;
  display: flex;
  flex-direction: column;
}

.notif-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px 10px;
  border-bottom: 1px solid #f0f2f5;
}

.notif-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.notif-empty {
  text-align: center;
  color: #909399;
  font-size: 13px;
  padding: 28px 0;
}

.notif-list {
  overflow-y: auto;
  flex: 1;
}

.notif-item {
  display: flex;
  gap: 10px;
  padding: 10px 8px;
  border-bottom: 1px solid #f5f7fa;
  cursor: pointer;
  transition: background 0.15s;
}

.notif-item:hover {
  background: #f5f7fa;
}

.notif-item-unread .notif-body {
  background: transparent;
}

.notif-item-unread::before {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #f56c6c;
  flex-shrink: 0;
  margin-top: 6px;
}

.notif-item-unread .notif-text,
.notif-item-unread .notif-content {
  font-weight: 600;
}

.notif-body {
  flex: 1;
  min-width: 0;
}

.notif-text {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 13px;
}

.notif-actor {
  color: #303133;
  font-weight: 600;
}

.notif-meta {
  color: #909399;
  font-size: 12px;
  background: #f0f2f5;
  border-radius: 4px;
  padding: 0 6px;
}

.notif-skill {
  color: #409eff;
  font-size: 12px;
}

.notif-content {
  color: #606266;
  font-size: 13px;
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.notif-time {
  color: #c0c4cc;
  font-size: 12px;
  margin-top: 2px;
}
</style>
