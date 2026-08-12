<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { authState, fetchMe, updateProfile, fetchMySkills } from '../api/auth'
import { deleteSkill } from '../api/skills'
import AppNavbar from '../components/AppNavbar.vue'
import GrowthPath from '../components/GrowthPath.vue'

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const mySkills = ref([])

const editForm = reactive({
  email: '',
  school: '',
  grade: '',
  major: '',
  bio: '',
  ai_level: '',
})

function fillForm(u) {
  editForm.email = u?.email || ''
  editForm.school = u?.school || ''
  editForm.grade = u?.grade || ''
  editForm.major = u?.major || ''
  editForm.bio = u?.bio || ''
  editForm.ai_level = u?.ai_level || ''
}

onMounted(async () => {
  loading.value = true
  try {
    // 拉取最新用户信息 + 我的技能
    const u = await fetchMe()
    fillForm(u)
    mySkills.value = await fetchMySkills()
  } catch (e) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
})

async function handleSaveProfile() {
  saving.value = true
  try {
    const u = await updateProfile(authState.user.id, { ...editForm })
    fillForm(u)
    ElMessage.success('资料已更新')
  } catch (e) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

function goDetail(id) {
  router.push(`/skill/${id}`)
}

function goPublish() {
  router.push('/publish')
}

async function handleDelete(skill) {
  try {
    await ElMessageBox.confirm(`确定删除技能「${skill.name}」吗？删除后不可恢复。`, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch (e) {
    return
  }
  try {
    await deleteSkill(skill.id)
    mySkills.value = mySkills.value.filter((s) => s.id !== skill.id)
    ElMessage.success('已删除')
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

function formatDate(d) {
  if (!d) return ''
  return new Date(d).toLocaleDateString('zh-CN')
}

const aiLevelMap = {
  never: '从未用过',
  beginner: '初级',
  intermediate: '中级',
  advanced: '高级',
}

function aiLevelLabel(level) {
  return aiLevelMap[level] || level || '未设置'
}
</script>

<template>
  <div class="profile-page">
    <AppNavbar />

    <main class="profile-main">
      <el-skeleton :rows="6" animated v-if="loading" class="skeleton" />

      <template v-else-if="authState.user">
        <!-- 成长路径：个人中心的主体。人是主体，路径是骨架 -->
        <section class="card growth-card">
          <div class="growth-head">
            <h2 class="growth-title">我的成长路径</h2>
            <el-button link type="primary" @click="router.push('/grow')">继续往下走</el-button>
          </div>
          <GrowthPath />
        </section>

        <!-- 用户信息卡片 -->
        <section class="card user-card">
          <div class="user-head">
            <el-avatar :size="64" :src="authState.user.avatar || undefined">
              {{ (authState.user.username || 'U')[0] }}
            </el-avatar>
            <div class="user-info">
              <div class="user-name">{{ authState.user.username }}</div>
              <div class="user-bio">{{ authState.user.bio || '这个人很懒，还没写简介~' }}</div>
            </div>
          </div>
          <div class="profile-tags">
            <el-tag v-if="authState.user.school" type="primary" effect="light">
              <el-icon><School /></el-icon>&nbsp;{{ authState.user.school }}
            </el-tag>
            <el-tag v-if="authState.user.grade" type="success" effect="light">
              <el-icon><Calendar /></el-icon>&nbsp;{{ authState.user.grade }}
            </el-tag>
            <el-tag v-if="authState.user.major" type="warning" effect="light">
              <el-icon><Reading /></el-icon>&nbsp;{{ authState.user.major }}
            </el-tag>
            <el-tag v-if="authState.user.ai_level" type="danger" effect="light">
              <el-icon><MagicStick /></el-icon>&nbsp;AI {{ aiLevelLabel(authState.user.ai_level) }}
            </el-tag>
          </div>
        </section>

        <!-- 编辑资料 -->
        <section class="card">
          <h2 class="section-title">编辑资料</h2>
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="邮箱">
                <el-input v-model="editForm.email" placeholder="邮箱" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="学校">
                <el-input v-model="editForm.school" placeholder="如：北京交通大学" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="年级">
                <el-input v-model="editForm.grade" placeholder="如：研二、大三" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="专业">
                <el-input v-model="editForm.major" placeholder="如：计算机科学与技术" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="AI 使用经验（影响技能介绍方式）">
                <el-radio-group v-model="editForm.ai_level">
                  <el-radio-button value="never">从未用过 AI 工具</el-radio-button>
                  <el-radio-button value="beginner">初级</el-radio-button>
                  <el-radio-button value="intermediate">中级</el-radio-button>
                  <el-radio-button value="advanced">高级</el-radio-button>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="个人简介">
                <el-input v-model="editForm.bio" type="textarea" :rows="3" placeholder="介绍你的技能方向" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-button type="primary" :loading="saving" @click="handleSaveProfile">保存资料</el-button>
        </section>

        <!-- 我的技能 -->
        <section class="card">
          <div class="section-head">
            <h2 class="section-title">我的技能（{{ mySkills.length }}）</h2>
            <el-button type="primary" round size="small" @click="goPublish">
              <el-icon style="margin-right: 4px"><Plus /></el-icon>发布新技能
            </el-button>
          </div>

          <template v-if="mySkills.length">
            <div class="my-skill" v-for="s in mySkills" :key="s.id">
              <div class="skill-info" @click="goDetail(s.id)">
                <div class="skill-name">
                  {{ s.name }}
                  <el-tag v-if="s.version" size="small" effect="plain">v{{ s.version }}</el-tag>
                </div>
                <div class="skill-desc">{{ s.description }}</div>
                <div class="skill-meta">
                  <el-tag size="small" type="success" effect="light">{{ s.category }}</el-tag>
                  <span class="meta-text">下载 {{ s.download_count }} · 浏览 {{ s.view_count }} · {{ formatDate(s.created_at) }}</span>
                </div>
              </div>
              <div class="skill-actions">
                <el-button size="small" @click="goDetail(s.id)">详情</el-button>
                <el-button size="small" type="danger" plain @click="handleDelete(s)">删除</el-button>
              </div>
            </div>
          </template>
          <el-empty v-else description="你还没有发布任何技能">
            <el-button type="primary" @click="goPublish">去发布第一个技能</el-button>
          </el-empty>
        </section>
      </template>
    </main>
  </div>
</template>

<style scoped>
.profile-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}

.profile-main {
  flex: 1;
  width: 100%;
  max-width: 860px;
  margin: 0 auto;
  padding: 24px;
}

.skeleton {
  background: #fff;
  border-radius: 10px;
  padding: 24px;
}

.card {
  background: #fff;
  border-radius: 10px;
  padding: 24px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
  margin-bottom: 16px;
}

/* 成长路径卡放在最上面：个人中心的主体不是资料表单，是走过的路 */
.growth-card {
  border-top: 3px solid #409eff;
}

.growth-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.growth-title {
  margin: 0;
  font-size: 20px;
}

.user-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.user-head {
  display: flex;
  align-items: center;
  gap: 16px;
}

.user-name {
  font-size: 20px;
  font-weight: 700;
  color: #303133;
}

.user-bio {
  font-size: 13px;
  color: #909399;
  margin-top: 4px;
}

.profile-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 16px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-head .section-title {
  margin-bottom: 0;
}

.my-skill {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 0;
  border-bottom: 1px solid #f0f2f5;
}

.my-skill:last-child {
  border-bottom: none;
}

.skill-info {
  flex: 1;
  cursor: pointer;
  min-width: 0;
}

.skill-name {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 8px;
}

.skill-desc {
  font-size: 13px;
  color: #606266;
  margin: 6px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.skill-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.meta-text {
  font-size: 12px;
  color: #909399;
}

.skill-actions {
  flex-shrink: 0;
  display: flex;
  gap: 8px;
}
</style>
