<script setup>
import { ref, reactive, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { login, register } from '../api/auth'
import AppNavbar from '../components/AppNavbar.vue'

const router = useRouter()
const route = useRoute()

const mode = ref('login') // login | register
const submitting = ref(false)

const loginForm = reactive({ account: '', password: '' })

const registerForm = reactive({
  username: '',
  email: '',
  password: '',
  confirm: '',
  school: '',
  grade: '',
  major: '',
  bio: '',
})

// ===== AI 使用经验问卷（5 题，是/否）=====
// 未答为 null，避免被当作"否"提交
const quiz = reactive({
  heard_of_llm: null, // 1 是否听说过 ChatGPT 等大语言模型
  used_llm: null, // 2 是否使用过大语言模型
  used_agent: null, // 3 是否使用过 Codex / Agent 等 AI 编码代理
  has_agent_installed: null, // 4 电脑中是否装有 Codex 等 Agent 工具
  ran_full_project: null, // 5 是否用上述 Agent 跑过完整项目
})

const quizVisible = ref(false)
const quizSubmitted = ref(false) // 问卷已作答并提交

// 5 道题（用户原话方向，措辞整理如下）
const quizItems = [
  { key: 'heard_of_llm', no: 1, text: '你是否听说过 ChatGPT 这类大语言模型（LLM）？' },
  { key: 'used_llm', no: 2, text: '你是否实际使用过 ChatGPT / DeepSeek / Claude 等大语言模型？' },
  { key: 'used_agent', no: 3, text: '你是否使用过 Codex / Trae / Cursor 等 AI 编码助手（Agent）？' },
  { key: 'has_agent_installed', no: 4, text: '你的电脑上是否安装过上述 AI 编码助手工具？' },
  { key: 'ran_full_project', no: 5, text: '你是否用 AI 编码助手完整跑通过一个项目（不止问答）？' },
]

// 与后端 inferAILevel 一致的推导规则
const quizLevel = computed(() => {
  const q = quiz
  if (q.heard_of_llm === null || q.used_llm === null || q.used_agent === null) return ''
  if (!q.heard_of_llm || !q.used_llm) return 'never'
  if (!q.used_agent) return 'beginner'
  if (q.ran_full_project !== null && !q.ran_full_project) return 'intermediate'
  if (q.ran_full_project) return 'advanced'
  return ''
})

const quizLevelMap = {
  never: { label: '从未用过', desc: '我们将用最通俗的方式介绍，全程零术语、带使用步骤' },
  beginner: { label: '初级', desc: '我们会在介绍里补充基础概念和上手路径' },
  intermediate: { label: '中级', desc: '我们会侧重功能亮点与实际使用场景' },
  advanced: { label: '高级', desc: '我们会讲技术构成、扩展点与最佳实践' },
}

const quizAnsweredCount = computed(() =>
  Object.values(quiz).filter((v) => v !== null).length
)

function switchMode(m) {
  mode.value = m
}

// 基本信息校验通过 → 弹出问卷
function openQuiz() {
  const f = registerForm
  if (f.username.trim().length < 2) {
    ElMessage.warning('用户名至少 2 个字符')
    return
  }
  if (f.password.length < 6) {
    ElMessage.warning('密码至少 6 位')
    return
  }
  if (f.password !== f.confirm) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  quizVisible.value = true
}

async function handleLogin() {
  if (!loginForm.account.trim() || !loginForm.password) {
    ElMessage.warning('请输入账号和密码')
    return
  }
  submitting.value = true
  try {
    await login(loginForm.account.trim(), loginForm.password)
    ElMessage.success('登录成功')
    router.push(route.query.redirect || '/')
  } catch (e) {
    ElMessage.error(e.message || '登录失败')
  } finally {
    submitting.value = false
  }
}

async function submitRegister() {
  if (quizAnsweredCount.value < 5) {
    ElMessage.warning('请完成全部 5 道选择题')
    return
  }
  if (!quizLevel.value) {
    ElMessage.warning('请先作答问卷')
    return
  }
  const f = registerForm
  submitting.value = true
  try {
    const user = await register({
      username: f.username.trim(),
      email: f.email.trim(),
      password: f.password,
      school: f.school.trim(),
      grade: f.grade.trim(),
      major: f.major.trim(),
      bio: f.bio.trim(),
      ai_quiz: {
        heard_of_llm: !!quiz.heard_of_llm,
        used_llm: !!quiz.used_llm,
        used_agent: !!quiz.used_agent,
        has_agent_installed: !!quiz.has_agent_installed,
        ran_full_project: !!quiz.ran_full_project,
      },
    })
    quizSubmitted.value = true
    ElMessage.success(`注册成功，已自动登录（AI 水平：${quizLevelMap[user.ai_level]?.label || user.ai_level}）`)
    router.push('/')
  } catch (e) {
    ElMessage.error(e.message || '注册失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <AppNavbar />

    <main class="auth-main">
      <div class="auth-card">
        <!-- 切换 -->
        <div class="tabs">
          <span class="tab" :class="{ active: mode === 'login' }" @click="switchMode('login')">登录</span>
          <span class="tab" :class="{ active: mode === 'register' }" @click="switchMode('register')">注册</span>
        </div>

        <!-- 登录表单 -->
        <el-form v-if="mode === 'login'" label-position="top" @submit.prevent="handleLogin">
          <el-form-item label="账号（用户名或邮箱）">
            <el-input v-model="loginForm.account" size="large" placeholder="请输入用户名或邮箱" clearable>
              <template #prefix><el-icon><User /></el-icon></template>
            </el-input>
          </el-form-item>
          <el-form-item label="密码">
            <el-input
              v-model="loginForm.password"
              size="large"
              type="password"
              placeholder="请输入密码"
              show-password
              @keyup.enter="handleLogin"
            >
              <template #prefix><el-icon><Lock /></el-icon></template>
            </el-input>
          </el-form-item>
          <el-button
            type="primary"
            size="large"
            class="submit-btn"
            :loading="submitting"
            @click="handleLogin"
          >
            登 录
          </el-button>
        </el-form>

        <!-- 注册表单 -->
        <el-form v-else label-position="top" @submit.prevent="openQuiz">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="用户名（必填）">
                <el-input v-model="registerForm.username" size="large" placeholder="至少 2 个字符" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="邮箱">
                <el-input v-model="registerForm.email" size="large" placeholder="用于找回账号" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="密码（必填）">
                <el-input v-model="registerForm.password" size="large" type="password" placeholder="至少 6 位" show-password />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="确认密码">
                <el-input v-model="registerForm.confirm" size="large" type="password" placeholder="再次输入密码" show-password />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="学校">
                <el-input v-model="registerForm.school" size="large" placeholder="如：北京交通大学" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="年级">
                <el-input v-model="registerForm.grade" size="large" placeholder="如：研二、大三" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="专业">
                <el-input v-model="registerForm.major" size="large" placeholder="如：计算机科学与技术" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="个人简介">
                <el-input
                  v-model="registerForm.bio"
                  type="textarea"
                  :rows="3"
                  placeholder="介绍一下你的技能方向（选填）"
                />
              </el-form-item>
            </el-col>
          </el-row>
          <div class="quiz-entry-tip">
            <el-icon style="vertical-align: -2px; margin-right: 4px"><MagicStick /></el-icon>
            注册时我们想知道你对 AI 工具的熟悉程度，这样能为你生成更合适的技能介绍。
          </div>
          <el-button
            type="primary"
            size="large"
            class="submit-btn"
            @click="openQuiz"
          >
            下一步：AI 使用经验问卷
          </el-button>
        </el-form>

        <!-- AI 使用经验问卷弹窗 -->
        <el-dialog
          v-model="quizVisible"
          title="AI 使用经验问卷"
          width="560px"
          :close-on-click-modal="false"
          destroy-on-close
        >
          <div class="quiz-subtitle">
            共 5 道题（是/否），你的答案决定了我们如何向你介绍技能——新手看得懂，老手看到干货。
          </div>

          <div class="quiz-item" v-for="q in quizItems" :key="q.key">
            <div class="quiz-q">
              <span class="quiz-no">{{ q.no }}</span>{{ q.text }}
            </div>
            <el-radio-group v-model="quiz[q.key]" class="quiz-ans">
              <el-radio-button :value="true">是</el-radio-button>
              <el-radio-button :value="false">否</el-radio-button>
            </el-radio-group>
          </div>

          <!-- 实时推导结果 -->
          <div class="quiz-result" v-if="quizLevel">
            <div class="quiz-result-label">
              你的 AI 水平：
              <el-tag type="warning" effect="light">{{ quizLevelMap[quizLevel].label }}</el-tag>
              <span class="quiz-progress">{{ quizAnsweredCount }}/5 已作答</span>
            </div>
            <div class="quiz-result-desc">{{ quizLevelMap[quizLevel].desc }}</div>
          </div>
          <div class="quiz-result" v-else>
            <span class="quiz-progress">{{ quizAnsweredCount }}/5 已作答，完成全部题目后即可提交</span>
          </div>

          <template #footer>
            <el-button @click="quizVisible = false">返回修改</el-button>
            <el-button type="primary" :loading="submitting" @click="submitRegister">
              完成注册
            </el-button>
          </template>
        </el-dialog>

        <div class="auth-tip">
          <template v-if="mode === 'login'">
            还没有账号？<el-link type="primary" @click="switchMode('register')">立即注册</el-link>
          </template>
          <template v-else>
            已有账号？<el-link type="primary" @click="switchMode('login')">直接登录</el-link>
          </template>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, #e8f1ff 0%, #f5f7fa 50%);
}

.auth-main {
  flex: 1;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 48px 24px;
}

.auth-card {
  width: 100%;
  max-width: 560px;
  background: #fff;
  border-radius: 12px;
  padding: 32px 36px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);
}

.tabs {
  display: flex;
  gap: 32px;
  margin-bottom: 28px;
  border-bottom: 1px solid #f0f2f5;
}

.tab {
  font-size: 18px;
  font-weight: 600;
  color: #909399;
  padding-bottom: 10px;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
}

.tab.active {
  color: #409eff;
  border-bottom-color: #409eff;
}

.submit-btn {
  width: 100%;
  margin-top: 8px;
}

.auth-tip {
  margin-top: 20px;
  text-align: center;
  font-size: 14px;
  color: #909399;
}

.level-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.quiz-entry-tip {
  font-size: 13px;
  color: #909399;
  background: #f5f7fa;
  border-radius: 8px;
  padding: 10px 14px;
  margin: 4px 0 16px;
  line-height: 1.6;
}

.quiz-subtitle {
  font-size: 13px;
  color: #909399;
  margin-bottom: 16px;
  line-height: 1.6;
}

.quiz-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid #f0f2f5;
}

.quiz-q {
  font-size: 14px;
  color: #303133;
  line-height: 1.5;
  flex: 1;
}

.quiz-no {
  display: inline-block;
  min-width: 22px;
  height: 22px;
  line-height: 22px;
  text-align: center;
  border-radius: 50%;
  background: #ecf5ff;
  color: #409eff;
  font-size: 12px;
  font-weight: 700;
  margin-right: 8px;
}

.quiz-ans {
  flex-shrink: 0;
}

.quiz-result {
  margin-top: 16px;
  background: #fffbf2;
  border: 1px solid #f3e2bd;
  border-radius: 8px;
  padding: 12px 14px;
}

.quiz-result-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #303133;
}

.quiz-progress {
  margin-left: auto;
  font-size: 12px;
  color: #b0853a;
}

.quiz-result-desc {
  margin-top: 6px;
  font-size: 12px;
  color: #8a6d3b;
  line-height: 1.6;
}
</style>
