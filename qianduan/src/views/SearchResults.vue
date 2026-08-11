<script setup>
import { ref, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { searchSkills } from '../api/skills'
import AppNavbar from '../components/AppNavbar.vue'

const route = useRoute()
const router = useRouter()

const keyword = ref('')
const loading = ref(false)
const results = ref([])
const searched = ref(false)

// 分类筛选
const activeCategory = ref('全部')

// 排序
const sortOrder = ref('default')

const filteredResults = computed(() => {
  let list = [...results.value]
  if (activeCategory.value !== '全部') {
    list = list.filter((s) => s.category === activeCategory.value)
  }
  if (sortOrder.value === 'rating') {
    list.sort((a, b) => b.rating - a.rating)
  } else if (sortOrder.value === 'likes') {
    list.sort((a, b) => b.likes - a.likes)
  }
  return list
})

const categoriesInResults = computed(() => {
  const set = new Set(results.value.map((s) => s.category))
  return ['全部', ...set]
})

async function doSearch(kw) {
  loading.value = true
  searched.value = true
  try {
    results.value = await searchSkills(kw)
  } finally {
    loading.value = false
  }
}

// 监听路由参数变化触发搜索
watch(
  () => route.query.q,
  (q) => {
    keyword.value = q || ''
    doSearch(q || '')
  },
  { immediate: true }
)

function handleSearch() {
  const kw = keyword.value.trim()
  if (!kw) return
  router.push({ path: '/search', query: { q: kw } })
}

function selectCategory(cat) {
  activeCategory.value = cat
}

function goBack() {
  router.push('/')
}

function goDetail(id) {
  router.push({ path: `/skill/${id}` })
}
</script>

<template>
  <div class="search-page">
    <!-- 顶部导航（搜索框放入中间 slot） -->
    <AppNavbar>
      <div class="search-bar">
        <el-input
          v-model="keyword"
          placeholder="搜索技能、标签或用户..."
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

    <!-- 内容区 -->
    <main class="content">
      <!-- 搜索信息 -->
      <div class="result-header">
        <div class="result-meta">
          <template v-if="searched">
            <span v-if="keyword">“{{ keyword }}” 的搜索结果</span>
            <span v-else>全部技能</span>
            <span class="count">共 {{ results.length }} 条</span>
          </template>
          <span v-else>技能列表</span>
        </div>
      </div>

      <!-- 筛选栏 -->
      <div class="filter-bar" v-if="searched && results.length">
        <el-radio-group v-model="sortOrder" size="small">
          <el-radio-button value="default">默认</el-radio-button>
          <el-radio-button value="rating">评分最高</el-radio-button>
          <el-radio-button value="likes">最受欢迎</el-radio-button>
        </el-radio-group>
        <div class="categories">
          <span
            v-for="cat in categoriesInResults"
            :key="cat"
            class="category-tag"
            :class="{ active: activeCategory === cat }"
            @click="selectCategory(cat)"
          >
            {{ cat }}
          </span>
        </div>
      </div>

      <!-- 加载中 -->
      <el-skeleton :rows="6" animated v-if="loading" class="skeleton" />

      <!-- 结果列表 -->
      <template v-else>
        <div class="skill-list" v-if="filteredResults.length">
          <div class="skill-card" v-for="skill in filteredResults" :key="skill.id" @click="goDetail(skill.id)">
            <div class="card-top">
              <div class="skill-name">
                {{ skill.name }}
                <el-tag v-if="skill.version" size="small" effect="plain">v{{ skill.version }}</el-tag>
              </div>
              <el-rate :model-value="skill.rating" disabled :show-score="false" text-color="#ff9900" />
            </div>
            <div class="skill-desc">{{ skill.description }}</div>
            <div class="skill-tags">
              <el-tag v-for="tag in skill.tags" :key="tag" size="small" type="info" effect="plain">
                {{ tag }}
              </el-tag>
            </div>
            <div class="card-bottom">
              <div class="owner">
                <el-avatar :size="24" :src="skill.avatar || undefined">{{ skill.owner[0] }}</el-avatar>
                <span class="owner-name">{{ skill.owner }}</span>
                <el-tag size="small" type="success" effect="plain">{{ skill.category }}</el-tag>
              </div>
              <div class="stats">
                <span class="stat"><el-icon><StarFilled /></el-icon> {{ skill.rating }}</span>
                <span class="stat"><el-icon><ThumbsUp /></el-icon> {{ skill.likes }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- 空状态 -->
        <el-empty v-else-if="searched" description="没有找到匹配的技能，换个关键词试试吧">
          <el-button type="primary" @click="goBack">返回首页</el-button>
        </el-empty>
      </template>
    </main>

    <footer class="footer">
      <span>SkillHub © 2026 · 技能发现与分享平台</span>
    </footer>
  </div>
</template>

<style scoped>
.search-page {
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
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
  padding: 24px;
}

.result-header {
  margin-bottom: 16px;
}

.result-meta {
  font-size: 18px;
  font-weight: 600;
  color: #303133;
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.count {
  font-size: 13px;
  font-weight: 400;
  color: #909399;
}

.filter-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 20px;
  padding: 12px 16px;
  background: #fff;
  border-radius: 8px;
}

.categories {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.category-tag {
  padding: 4px 12px;
  border-radius: 16px;
  font-size: 13px;
  color: #606266;
  cursor: pointer;
  background: #f5f7fa;
  transition: all 0.2s;
}

.category-tag:hover {
  color: #409eff;
}

.category-tag.active {
  background: #409eff;
  color: #fff;
}

.skeleton {
  padding: 16px;
  background: #fff;
  border-radius: 8px;
}

.skill-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}

.skill-card {
  background: #fff;
  border-radius: 10px;
  padding: 20px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  transition: transform 0.2s, box-shadow 0.2s;
  cursor: pointer;
}

.skill-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.08);
}

.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.skill-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 8px;
}

.skill-desc {
  font-size: 13px;
  color: #606266;
  line-height: 1.6;
  margin-bottom: 12px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.skill-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}

.card-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid #f0f2f5;
  padding-top: 12px;
}

.owner {
  display: flex;
  align-items: center;
  gap: 8px;
}

.owner-name {
  font-size: 13px;
  color: #606266;
}

.stats {
  display: flex;
  gap: 16px;
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
