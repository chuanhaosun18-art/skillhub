<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getHotKeywords } from '../api/skills'
import { authState } from '../api/auth'
import AppNavbar from '../components/AppNavbar.vue'

const router = useRouter()
const keyword = ref('')
const hotKeywords = ref([])

onMounted(async () => {
  hotKeywords.value = await getHotKeywords()
})

function handleSearch() {
  const kw = keyword.value.trim()
  if (!kw) {
    ElMessage.warning('请输入要搜索的技能')
    return
  }
  router.push({ path: '/search', query: { q: kw } })
}

function searchKeyword(kw) {
  keyword.value = kw
  router.push({ path: '/search', query: { q: kw } })
}
</script>

<template>
  <div class="home">
    <!-- 顶部导航 -->
    <AppNavbar />

    <!-- 主内容 -->
    <main class="hero">
      <h1 class="title">发现身边最值得学习的技能</h1>
      <p class="subtitle">
        无需登录即可搜索他人技能，登录后可发布自己的技能并获取推荐
      </p>

      <!-- 搜索框 -->
      <div class="search-box">
        <el-input
          v-model="keyword"
          size="large"
          placeholder="搜索你感兴趣的技能，如：Vue、Python、机器学习..."
          clearable
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-button type="primary" size="large" @click="handleSearch">搜索</el-button>
      </div>

      <!-- 热门搜索 -->
      <div class="hot-search" v-if="hotKeywords.length">
        <span class="hot-label">热门搜索：</span>
        <el-link
          v-for="kw in hotKeywords"
          :key="kw"
          class="hot-item"
          type="primary"
          @click="searchKeyword(kw)"
        >
          {{ kw }}
        </el-link>
      </div>

      <!-- 登录态提示 -->
      <div class="guest-tip">
        <el-icon><InfoFilled /></el-icon>
        <template v-if="authState.user">
          <span>欢迎回来，{{ authState.user.username }}！已登录用户可发布技能、管理自己的技能包。</span>
        </template>
        <template v-else>
          <span>当前为游客模式，搜索即用。登录后可发布技能、收藏与获得个性化推荐。</span>
        </template>
      </div>
    </main>

    <!-- 底部 -->
    <footer class="footer">
      <span>SkillHub © 2026 · 技能发现与分享平台</span>
    </footer>
  </div>
</template>

<style scoped>
.home {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, #e8f1ff 0%, #f5f7fa 40%);
}

.hero {
  flex: 1;
  max-width: 900px;
  margin: 0 auto;
  padding: 60px 24px 40px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.title {
  font-size: 42px;
  font-weight: 700;
  color: #303133;
  margin-bottom: 12px;
}

.subtitle {
  font-size: 16px;
  color: #909399;
  margin-bottom: 40px;
}

.search-box {
  width: 100%;
  max-width: 680px;
  display: flex;
  gap: 12px;
}

.search-box :deep(.el-input__wrapper) {
  border-radius: 24px;
  padding: 4px 20px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.06);
}

.search-box .el-button {
  border-radius: 24px;
  padding: 0 32px;
  font-size: 16px;
}

.hot-search {
  margin-top: 24px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 16px;
}

.hot-label {
  color: #606266;
  font-size: 14px;
}

.hot-item {
  font-size: 14px;
}

.guest-tip {
  margin-top: 48px;
  padding: 12px 24px;
  background: rgba(64, 158, 255, 0.08);
  border: 1px solid rgba(64, 158, 255, 0.2);
  border-radius: 8px;
  color: #606266;
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.footer {
  text-align: center;
  padding: 24px;
  color: #c0c4cc;
  font-size: 13px;
}
</style>
