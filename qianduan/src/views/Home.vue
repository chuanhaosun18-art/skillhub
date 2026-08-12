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
      <h1 class="title">你现在卡在哪？</h1>
      <p class="subtitle">
        前台卖「下一步」，后台跑 Skill。用你自己的话说清现在的处境，
        剩下的交给我——不用先想清楚自己需要什么能力。
      </p>

      <!-- 主入口：我要成长。Skill 是底层基础设施，不该是前台术语 -->
      <div class="grow-band">
        <div class="grow-text">
          <div class="grow-title">我要成长</div>
          <div class="grow-sub">
            说一句你现在的困境 → 拿到今天的下一步 → 在工作台把它做完 →
            这次的方法自动变成别人也能用的 Skill
          </div>
        </div>
        <el-button type="primary" size="large" round @click="$router.push('/grow')">
          帮我找下一步
        </el-button>
      </div>

      <div class="or-line"><span>或者，直接翻别人已经沉淀好的能力</span></div>

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
      <span>WowSkillLand © 2026 · 大学生成长复利系统</span>
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
  margin-bottom: 28px;
  line-height: 1.8;
  max-width: 620px;
  margin-left: auto;
  margin-right: auto;
}

/* 主入口在视觉上必须压过搜索框：产品不是 Skill 目录 */
.grow-band {
  max-width: 720px;
  margin: 0 auto 24px;
  background: linear-gradient(135deg, #ecf5ff 0%, #f7fbff 100%);
  border: 1px solid #c6e2ff;
  border-radius: 14px;
  padding: 22px 26px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  text-align: left;
}
.grow-title {
  font-size: 20px;
  font-weight: 700;
  color: #303133;
  margin-bottom: 6px;
}
.grow-sub {
  font-size: 13px;
  color: #606266;
  line-height: 1.8;
}
.or-line {
  position: relative;
  max-width: 720px;
  margin: 0 auto 20px;
  text-align: center;
}
.or-line::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 0;
  right: 0;
  height: 1px;
  background: #ebeef5;
}
.or-line span {
  position: relative;
  background: #f5f7fa;
  padding: 0 14px;
  font-size: 12px;
  color: #c0c4cc;
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
