<script setup>
import { useRouter } from 'vue-router'
import { ElMessageBox, ElMessage } from 'element-plus'
import { authState, clearAuth } from '../api/auth'

const router = useRouter()

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
        <el-button v-if="authState.user" text @click="goWorkbench">我的任务</el-button>
        <template v-if="authState.user">
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
</style>
