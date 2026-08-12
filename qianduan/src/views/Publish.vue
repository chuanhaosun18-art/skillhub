<script setup>
import { ref, reactive, computed, nextTick, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createSkill, guideChat, generateSkillPack } from '../api/skills'
import { authState } from '../api/auth'
import AppNavbar from '../components/AppNavbar.vue'

const router = useRouter()

// ===== 发布路径选择：upload | guide =====
const mode = ref('') // 空 = 未选择

function chooseMode(m) {
  mode.value = m
  clearProofs()
}

// ===== 评估指标证明图片（能力证明，多张） =====
const PROOF_MAX = 6
const proofList = ref([]) // { raw: File, url: objectURL }

function clearProofs() {
  proofList.value.forEach((p) => p.url && URL.revokeObjectURL(p.url))
  proofList.value = []
}

function addProofFiles(fileList) {
  for (const f of fileList) {
    const raw = f.raw || f
    if (!raw || !raw.type || !raw.type.startsWith('image/')) {
      ElMessage.warning('评估指标图片仅支持图片格式')
      continue
    }
    if (raw.size > 10 * 1024 * 1024) {
      ElMessage.warning('单张评估指标图片不能超过 10MB')
      continue
    }
    if (proofList.value.length >= PROOF_MAX) {
      ElMessage.warning(`最多上传 ${PROOF_MAX} 张评估指标图片`)
      break
    }
    proofList.value.push({ raw, url: URL.createObjectURL(raw) })
  }
}

function removeProof(idx) {
  const item = proofList.value[idx]
  if (item && item.url) URL.revokeObjectURL(item.url)
  proofList.value.splice(idx, 1)
}

// ===== 方式一：直接上传 =====
const submitting = ref(false)
const archiveFile = ref(null)

const form = reactive({
  name: '',
  description: '',
  category: '其他',
  tags: [],
  version: '1.0.0',
})

const categories = [
  '论文写作',
  '编程开发',
  '数据科学',
  '设计创作',
  '效率工具',
  '语言学习',
  '其他',
]

function handleFileChange(file) {
  const raw = file.raw || file
  if (raw && raw.name && !/\.zip$/i.test(raw.name)) {
    ElMessage.warning('请上传 .zip 格式的 skill 包（可选）')
    archiveFile.value = null
    return
  }
  archiveFile.value = raw || null
}

function removeFile() {
  archiveFile.value = null
}

async function handlePublish() {
  if (form.name.trim().length < 2) {
    ElMessage.warning('请输入技能名称（至少 2 个字符）')
    return
  }
  if (!form.description.trim()) {
    ElMessage.warning('请输入技能描述')
    return
  }
  if (form.tags.length > 8) {
    ElMessage.warning('标签最多 8 个')
    return
  }

  const fd = new FormData()
  fd.append('name', form.name.trim())
  fd.append('description', form.description.trim())
  fd.append('category', form.category)
  fd.append('tags', JSON.stringify(form.tags))
  fd.append('version', form.version.trim() || '1.0.0')
  if (archiveFile.value) fd.append('archive', archiveFile.value)
  proofList.value.forEach((p) => fd.append('proof_images', p.raw))

  submitting.value = true
  try {
    const skill = await createSkill(fd)
    ElMessage.success('技能发布成功')
    router.push(`/skill/${skill.id}`)
  } catch (e) {
    ElMessage.error(e.message || '发布失败')
  } finally {
    submitting.value = false
  }
}

// ===== 方式二：AI 引导创建 =====
// 对话消息：{ role: 'user'|'assistant', content, kind?: 'text'|'image'|'file', attName?, attMime? }
const messages = ref([])
const inputText = ref('')
const sending = ref(false)
const generating = ref(false)
const chatBox = ref(null)

// 引导进度：LLM 在回复中夹带 【进度】标签
const progress = ref(0)

// 语音输入（Web Speech API）
let recognition = null
const listening = ref(false)
const voiceSupported = ref(
  typeof window !== 'undefined' && !!(window.SpeechRecognition || window.webkitSpeechRecognition)
)

function initRecognition() {
  if (recognition || !voiceSupported.value) return
  const SR = window.SpeechRecognition || window.webkitSpeechRecognition
  recognition = new SR()
  recognition.lang = 'zh-CN'
  recognition.interimResults = true
  recognition.continuous = true
  recognition.onresult = (e) => {
    let finalText = ''
    let interim = ''
    for (let i = e.resultIndex; i < e.results.length; i++) {
      const r = e.results[i]
      if (r.isFinal) finalText += r[0].transcript
      else interim += r[0].transcript
    }
    // 中间结果实时回填到输入框
    if (finalText) {
      inputText.value = (inputText.value + finalText).trim()
    }
    if (interim) {
      // 只展示提示，不覆盖已输入内容
      if (!inputText.value) inputText.value = interim
    }
  }
  recognition.onend = () => {
    listening.value = false
  }
  recognition.onerror = () => {
    listening.value = false
    ElMessage.warning('语音识别出错，请重试或改用文字输入')
  }
}

function toggleVoice() {
  if (!voiceSupported.value) {
    ElMessage.warning('当前浏览器不支持语音输入，请使用 Chrome / Edge')
    return
  }
  initRecognition()
  if (listening.value) {
    recognition.stop()
    listening.value = false
    return
  }
  try {
    recognition.start()
    listening.value = true
  } catch (e) {
    listening.value = false
  }
}

onBeforeUnmount(() => {
  if (recognition && listening.value) {
    try {
      recognition.stop()
    } catch (e) {
      /* ignore */
    }
  }
})

// 文件 / 图片附件
async function handleAttachment(file, type) {
  const raw = file.raw || file
  if (!raw) return
  if (raw.size > 4 * 1024 * 1024) {
    ElMessage.warning('附件大小不能超过 4MB')
    return
  }
  const reader = new FileReader()
  reader.onload = async () => {
    const base64 = String(reader.result).split(',')[1]
    messages.value.push({
      role: 'user',
      kind: type,
      content: type === 'image' ? `[图片附件] ${raw.name}` : `[文件附件] ${raw.name}`,
      attName: raw.name,
      attMime: raw.type || '',
      attData: base64,
    })
    await scrollToBottom()
    await sendMessages()
  }
  reader.readAsDataURL(raw)
}

function onImageChange(file) {
  handleAttachment(file, 'image')
}
function onFileChange(file) {
  handleAttachment(file, 'file')
}

async function scrollToBottom() {
  await nextTick()
  if (chatBox.value) chatBox.value.scrollTop = chatBox.value.scrollHeight
}

// 简单 markdown 渲染（安全转义 + 换行 + 粗体/行内代码/代码块）
function escapeHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function renderMsg(content) {
  if (!content) return ''
  let html = escapeHtml(content)
  // 代码块 ```lang ... ```
  html = html.replace(/```(\w*)\n([\s\S]*?)```/g, (_m, lang, code) => `<pre><code>${code.trim()}</code></pre>`)
  // 行内代码 `code`
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  // 粗体 **text**
  html = html.replace(/\*\*([^*]+)\*\*/g, '<b>$1</b>')
  // 换行 -> <br>
  html = html.replace(/\n/g, '<br/>')
  return html
}

// 提取 LLM 回复中的引导进度标签（如 【进度】40%）
function extractProgress(text) {
  const m = String(text).match(/【进度】(\d{1,3})/)
  if (m) {
    const p = Math.min(100, Math.max(0, parseInt(m[1], 10)))
    progress.value = p
    return true
  }
  return false
}

async function sendMessages() {
  const text = inputText.value.trim()
  if (!text && !(messages.value.length && messages.value[messages.value.length - 1].attData)) {
    return
  }
  if (sending.value) return

  // 组装当前消息：取最后一条 user 消息（可能是附件消息或纯文本）
  let attachment = null
  const last = messages.value[messages.value.length - 1]
  if (last && last.role === 'user' && last.attData) {
    attachment = {
      type: last.kind,
      name: last.attName,
      mime: last.attMime,
      data: last.attData,
    }
  } else if (text) {
    messages.value.push({ role: 'user', kind: 'text', content: text })
    inputText.value = ''
  }

  const history = messages.value
    .filter((m) => m.role === 'user' || m.role === 'assistant')
    .map((m) => ({ role: m.role, content: m.content }))

  sending.value = true
  try {
    const resp = await guideChat(history, attachment)
    const reply = resp.data || ''
    extractProgress(reply)
    // 剥离进度标签，避免在气泡中露出
    const clean = String(reply).replace(/【进度】\s*\d{1,3}\s*%?\s*/g, '').trim()
    messages.value.push({ role: 'assistant', kind: 'text', content: clean })
    await scrollToBottom()
  } catch (e) {
    ElMessage.error(e.message || 'AI 回复失败，请重试')
  } finally {
    sending.value = false
  }
}

function handleSend() {
  sendMessages()
}

function handleKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessages()
  }
}

// ===== 生成 skill 包 =====
const genResult = ref(null) // { name, title, description, category, tags, version, files, zip_base64 }
const publishInfo = reactive({ name: '', description: '', category: '其他', tags: [], version: '1.0.0' })
const publishingGen = ref(false)
const skillMdPreview = computed(() => {
  if (!genResult.value) return ''
  const md = genResult.value.files.find((f) => f.path === 'SKILL.md')
  return md ? md.content : ''
})

async function handleGenerate() {
  if (messages.value.length === 0) {
    ElMessage.warning('还没有任何对话内容，先和 AI 聊聊你的想法吧')
    return
  }
  generating.value = true
  try {
    const resp = await generateSkillPack(
      messages.value
        .filter((m) => m.role === 'user' || m.role === 'assistant')
        .map((m) => ({ role: m.role, content: m.content }))
    )
    const d = resp.data
    genResult.value = d
    publishInfo.name = d.title || d.name
    publishInfo.description = d.description || ''
    publishInfo.category = d.category || '其他'
    publishInfo.tags = Array.isArray(d.tags) ? d.tags : []
    publishInfo.version = d.version || '1.0.0'
    await scrollToBottom()
  } catch (e) {
    ElMessage.error(e.message || '生成失败，请重试')
  } finally {
    generating.value = false
  }
}

async function handlePublishGenerated() {
  if (publishInfo.name.trim().length < 2) {
    ElMessage.warning('请输入技能名称（至少 2 个字符）')
    return
  }
  if (!publishInfo.description.trim()) {
    ElMessage.warning('请输入技能描述')
    return
  }
  publishingGen.value = true
  try {
    // zip base64 -> Blob -> FormData
    const zipBytes = atob(genResult.value.zip_base64)
    const arr = new Uint8Array(zipBytes.length)
    for (let i = 0; i < zipBytes.length; i++) arr[i] = zipBytes.charCodeAt(i)
    const zipBlob = new Blob([arr], { type: 'application/zip' })
    const zipFile = new File([zipBlob], `${genResult.value.name}.zip`, { type: 'application/zip' })

    const fd = new FormData()
    fd.append('name', publishInfo.name.trim())
    fd.append('description', publishInfo.description.trim())
    fd.append('category', publishInfo.category)
    fd.append('tags', JSON.stringify(publishInfo.tags))
    fd.append('version', publishInfo.version.trim() || '1.0.0')
    fd.append('archive', zipFile)
    proofList.value.forEach((p) => fd.append('proof_images', p.raw))

    const skill = await createSkill(fd)
    ElMessage.success('技能发布成功')
    router.push(`/skill/${skill.id}`)
  } catch (e) {
    ElMessage.error(e.message || '发布失败')
  } finally {
    publishingGen.value = false
  }
}

function backToModeSelect() {
  mode.value = ''
  clearProofs()
  messages.value = []
  genResult.value = null
  progress.value = 0
}
</script>

<template>
  <div class="publish-page">
    <AppNavbar />

    <main class="publish-main">
      <!-- 路径选择 -->
      <div v-if="!mode" class="publish-card">
        <h1 class="page-title">发布技能</h1>
        <p class="page-sub">选择一种方式，把你的经验、流程或技能包分享给更多人</p>

        <div class="mode-grid">
          <button class="mode-card" @click="chooseMode('upload')">
            <el-icon class="mode-icon"><UploadFilled /></el-icon>
            <div class="mode-title">直接上传 Skill 包</div>
            <div class="mode-desc">我已经有完整的 skill 文件（zip 格式），直接填写信息发布，适合熟悉 AI 工具的同学。</div>
          </button>
          <button class="mode-card" @click="chooseMode('guide')">
            <el-icon class="mode-icon"><MagicStick /></el-icon>
            <div class="mode-title">AI 引导创建</div>
            <div class="mode-desc">还不清楚怎么构建 skill？和 AI 对话（支持文字、语音、文件、图片），由 AI 帮你把经验整理成完整的 skill 包。</div>
          </button>
        </div>
      </div>

      <!-- 方式一：直接上传 -->
      <div v-else-if="mode === 'upload'" class="publish-card">
        <div class="back-bar">
          <el-button link @click="mode = ''"><el-icon><Back /></el-icon> 返回选择</el-button>
        </div>
        <h1 class="page-title">发布技能</h1>
        <p class="page-sub">分享你的技能包，帮助更多人学习和使用</p>

        <el-form label-position="top">
          <el-form-item label="技能名称（必填）">
            <el-input v-model="form.name" size="large" placeholder="如：BJTU 论文自动写作 Skill" maxlength="60" show-word-limit />
          </el-form-item>

          <el-form-item label="技能描述（必填）">
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="4"
              placeholder="介绍一下这个技能的用途、适用场景、如何使用..."
            />
          </el-form-item>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="分类">
                <el-select v-model="form.category" size="large" style="width: 100%" allow-create filterable>
                  <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="版本号">
                <el-input v-model="form.version" size="large" placeholder="如：1.0.0" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="标签（最多 8 个，回车添加）">
            <el-select
              v-model="form.tags"
              multiple
              filterable
              allow-create
              default-first-option
              placeholder="输入后回车添加标签"
              style="width: 100%"
            >
              <el-option v-for="t in form.tags" :key="t" :label="t" :value="t" />
            </el-select>
          </el-form-item>

          <el-form-item label="Skill 包（可选，zip 格式）">
            <div class="upload-area">
              <el-upload
                drag
                :auto-upload="false"
                :show-file-list="false"
                :limit="1"
                accept=".zip"
                :on-change="handleFileChange"
              >
                <el-icon class="upload-icon"><UploadFilled /></el-icon>
                <div class="el-upload__text">拖拽 zip 到此处，或 <em>点击选择</em></div>
                <template #tip>
                  <div class="el-upload__tip">skill 包为完整文件集合（如插件、脚本、配置模板等），不填则仅发布文字介绍</div>
                </template>
              </el-upload>
              <div v-if="archiveFile" class="file-picked">
                <el-icon color="#409eff"><Document /></el-icon>
                <span class="file-name">{{ archiveFile.name }}</span>
                <el-button link type="danger" @click="removeFile">移除</el-button>
              </div>
            </div>
          </el-form-item>

          <el-form-item label="评估指标图片（能力证明，可选，最多 6 张）">
            <div class="proof-uploader">
              <el-upload
                list-type="picture-card"
                :auto-upload="false"
                :show-file-list="false"
                accept="image/*"
                multiple
                :disabled="proofList.length >= PROOF_MAX"
                :on-change="(f) => addProofFiles([f])"
              >
                <el-icon><Plus /></el-icon>
              </el-upload>
              <div v-for="(p, i) in proofList" :key="i" class="proof-item">
                <img :src="p.url" class="proof-img" alt="评估指标图片" />
                <span class="proof-remove" @click="removeProof(i)">×</span>
              </div>
            </div>
            <div class="el-upload__tip">上传完成任务的成果截图、执行记录、结果对比等，用证据证明这个技能真的可用（将展示在搜索结果中）</div>
          </el-form-item>

          <el-button
            type="primary"
            size="large"
            class="publish-btn"
            :loading="submitting"
            @click="handlePublish"
          >
            发布技能
          </el-button>
        </el-form>
      </div>

      <!-- 方式二：AI 引导创建 -->
      <div v-else-if="mode === 'guide'" class="publish-card guide-card">
        <div class="back-bar">
          <el-button link @click="backToModeSelect"><el-icon><Back /></el-icon> 返回选择</el-button>
        </div>
        <h1 class="page-title">AI 引导创建 Skill</h1>
        <p class="page-sub">
          和 AI 聊一聊你想封装的经验或流程，支持 <b>打字、语音、文件、图片</b> 与 AI 交流。信息足够后 AI 会提示你生成 Skill 包。
        </p>

        <template v-if="!genResult">
          <div ref="chatBox" class="chat-box">
            <div v-if="messages.length === 0" class="chat-empty">
              <p>💡 试试这样说：</p>
              <p>「我想做一个帮同学整理<b>保研经验</b>的 skill，包括材料准备、导师联系、复试技巧这些方面」</p>
              <p>「我想把写<b>实验报告</b>的流程做成 skill，输入原始数据，自动生成规范报告」</p>
            </div>

            <div v-for="(m, i) in messages" :key="i" class="msg" :class="m.role">
              <div class="msg-bubble">
                <div v-if="m.kind === 'image'" class="att-tag">
                  <el-icon><Picture /></el-icon> 图片附件：{{ m.attName }}
                </div>
                <div v-else-if="m.kind === 'file'" class="att-tag">
                  <el-icon><Document /></el-icon> 文件附件：{{ m.attName }}
                </div>
                <div class="msg-text" v-html="renderMsg(m.content)"></div>
              </div>
            </div>

            <div v-if="sending" class="msg assistant">
              <div class="msg-bubble typing"><span class="dot"></span><span class="dot"></span><span class="dot"></span></div>
            </div>
          </div>

          <!-- 引导进度 -->
          <div class="guide-progress" v-if="progress > 0">
            <el-progress :percentage="progress" :stroke-width="8" :show-text="false" />
            <span class="progress-text">信息完整度 {{ progress }}%</span>
          </div>

          <div class="input-area">
            <div class="attach-btns">
              <el-upload :auto-upload="false" :show-file-list="false" accept="image/*" :on-change="onImageChange">
                <el-tooltip content="发送图片" placement="top">
                  <el-button :icon="Picture" circle />
                </el-tooltip>
              </el-upload>
              <el-upload :auto-upload="false" :show-file-list="false" :on-change="onFileChange">
                <el-tooltip content="发送文件" placement="top">
                  <el-button :icon="Document" circle />
                </el-tooltip>
              </el-upload>
              <el-tooltip :content="listening ? '结束录音' : '语音输入'" placement="top">
                <el-button :icon="Microphone" circle :class="{ 'listening': listening }" @click="toggleVoice" />
              </el-tooltip>
              <span v-if="listening" class="voice-tip">正在聆听…</span>
              <span v-else-if="!voiceSupported" class="voice-tip">浏览器不支持语音，请用 Chrome/Edge</span>
            </div>
            <div class="input-row">
              <el-input
                v-model="inputText"
                type="textarea"
                :rows="2"
                resize="none"
                placeholder="和 AI 描述你想创建的 skill，Enter 发送（Shift+Enter 换行）"
                @keydown="handleKeydown"
              />
              <el-button
                type="primary"
                class="send-btn"
                :loading="sending"
                @click="handleSend"
              >
                发送
              </el-button>
            </div>
            <el-button
              type="warning"
              class="generate-btn"
              :loading="generating"
              @click="handleGenerate"
            >
              ✨ 生成 Skill 包
            </el-button>
          </div>
        </template>

        <!-- 生成结果预览 -->
        <template v-else>
          <el-alert
            type="success"
            :closable="false"
            show-icon
            class="gen-alert"
            title="Skill 包生成完成！"
            :description="`共 ${genResult.files.length} 个文件，请在下方确认信息后发布`"
          />

          <el-form label-position="top" class="gen-form">
            <el-form-item label="技能名称（必填）">
              <el-input v-model="publishInfo.name" size="large" maxlength="60" show-word-limit />
            </el-form-item>
            <el-form-item label="技能描述（必填）">
              <el-input v-model="publishInfo.description" type="textarea" :rows="3" />
            </el-form-item>
            <el-row :gutter="16">
              <el-col :span="12">
                <el-form-item label="分类">
                  <el-select v-model="publishInfo.category" style="width: 100%" allow-create filterable>
                    <el-option v-for="c in categories" :key="c" :label="c" :value="c" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="版本号">
                  <el-input v-model="publishInfo.version" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item label="标签（最多 8 个）">
              <el-select v-model="publishInfo.tags" multiple filterable allow-create default-first-option style="width: 100%">
                <el-option v-for="t in publishInfo.tags" :key="t" :label="t" :value="t" />
              </el-select>
            </el-form-item>
            <el-form-item label="评估指标图片（能力证明，可选，最多 6 张）">
              <div class="proof-uploader">
                <el-upload
                  list-type="picture-card"
                  :auto-upload="false"
                  :show-file-list="false"
                  accept="image/*"
                  multiple
                  :disabled="proofList.length >= PROOF_MAX"
                  :on-change="(f) => addProofFiles([f])"
                >
                  <el-icon><Plus /></el-icon>
                </el-upload>
                <div v-for="(p, i) in proofList" :key="i" class="proof-item">
                  <img :src="p.url" class="proof-img" alt="评估指标图片" />
                  <span class="proof-remove" @click="removeProof(i)">×</span>
                </div>
              </div>
              <div class="el-upload__tip">上传完成任务的成果截图、执行记录、结果对比等，用证据证明这个技能真的可用（将展示在搜索结果中）</div>
            </el-form-item>
          </el-form>

          <div class="gen-files">
            <h3 class="gen-files-title">生成的文件（{{ genResult.files.length }}）</h3>
            <div v-for="(f, i) in genResult.files" :key="i" class="gen-file-item">
              <el-icon color="#67c23a"><FolderOpened /></el-icon>
              <span class="gen-file-path">{{ f.path }}</span>
              <span class="gen-file-size">{{ (new Blob([f.content]).size / 1024).toFixed(1) }} KB</span>
            </div>
          </div>

          <div class="gen-preview">
            <h3 class="gen-files-title">SKILL.md 预览</h3>
            <pre class="gen-preview-code">{{ skillMdPreview }}</pre>
          </div>

          <div class="gen-actions">
            <el-button @click="genResult = null">返回继续对话</el-button>
            <el-button type="primary" :loading="publishingGen" @click="handlePublishGenerated">确认发布</el-button>
          </div>
        </template>
      </div>
    </main>
  </div>
</template>

<style scoped>
.publish-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fa;
}

.publish-main {
  flex: 1;
  width: 100%;
  max-width: 760px;
  margin: 0 auto;
  padding: 32px 24px;
}

.publish-card {
  background: #fff;
  border-radius: 12px;
  padding: 32px 36px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
}

.page-title {
  font-size: 22px;
  font-weight: 700;
  color: #303133;
}

.page-sub {
  font-size: 13px;
  color: #909399;
  margin: 6px 0 24px;
}

.back-bar {
  margin-bottom: 8px;
}

/* 路径选择 */
.mode-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.mode-card {
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  padding: 28px 24px;
  text-align: left;
  background: #fff;
  cursor: pointer;
  transition: all 0.2s;
  font-family: inherit;
}

.mode-card:hover {
  border-color: #409eff;
  box-shadow: 0 6px 20px rgba(64, 158, 255, 0.12);
  transform: translateY(-2px);
}

.mode-icon {
  font-size: 32px;
  color: #409eff;
  margin-bottom: 12px;
}

.mode-card:nth-child(2) .mode-icon {
  color: #e6a23c;
}

.mode-title {
  font-size: 17px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}

.mode-desc {
  font-size: 13px;
  color: #909399;
  line-height: 1.7;
}

.upload-icon {
  font-size: 48px;
  color: #c0c4cc;
  margin-bottom: 8px;
}

.file-picked {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding: 10px 16px;
  background: #f5f7fa;
  border-radius: 8px;
}

.file-name {
  flex: 1;
  font-size: 13px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.publish-btn {
  width: 100%;
  margin-top: 8px;
}

/* 评估指标证明图片 */
.proof-uploader {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  width: 100%;
}

.proof-uploader :deep(.el-upload--picture-card) {
  width: 100px;
  height: 100px;
}

.proof-item {
  position: relative;
  width: 100px;
  height: 100px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #e4e7ed;
  background: #f5f7fa;
}

.proof-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.proof-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 14px;
  line-height: 18px;
  text-align: center;
  cursor: pointer;
  user-select: none;
}

.proof-remove:hover {
  background: #f56c6c;
}

/* AI 引导 */
.guide-card {
  max-width: 820px;
}

.chat-box {
  height: 380px;
  overflow-y: auto;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 16px;
  background: #fafbfc;
  margin-bottom: 12px;
}

.chat-empty {
  text-align: center;
  color: #909399;
  font-size: 13px;
  line-height: 2.2;
  padding: 48px 16px;
}

.msg {
  display: flex;
  margin-bottom: 12px;
}

.msg.user {
  justify-content: flex-end;
}

.msg.assistant {
  justify-content: flex-start;
}

.msg-bubble {
  max-width: 78%;
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.7;
  word-break: break-word;
}

.msg.user .msg-bubble {
  background: #409eff;
  color: #fff;
  border-top-right-radius: 4px;
}

.msg.assistant .msg-bubble {
  background: #fff;
  border: 1px solid #e4e7ed;
  border-top-left-radius: 4px;
  color: #303133;
}

.msg-text :deep(p) {
  margin: 4px 0;
}

.msg-text :deep(ul),
.msg-text :deep(ol) {
  margin: 4px 0;
  padding-left: 20px;
}

.msg-text :deep(pre) {
  background: #282c34;
  color: #abb2bf;
  padding: 10px;
  border-radius: 8px;
  overflow-x: auto;
  font-size: 12px;
  margin: 6px 0;
}

.att-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 255, 255, 0.2);
  border: 1px dashed rgba(255, 255, 255, 0.5);
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 13px;
}

.msg.assistant .att-tag {
  background: #f5f7fa;
  border-color: #dcdfe6;
  color: #606266;
}

.typing {
  display: flex;
  gap: 4px;
  align-items: center;
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #c0c4cc;
  animation: blink 1.2s infinite;
}

.dot:nth-child(2) {
  animation-delay: 0.2s;
}

.dot:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes blink {
  0%, 80%, 100% {
    opacity: 0.3;
  }
  40% {
    opacity: 1;
  }
}

.guide-progress {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.guide-progress .el-progress {
  flex: 1;
}

.progress-text {
  font-size: 12px;
  color: #909399;
  white-space: nowrap;
}

.input-area {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.attach-btns {
  display: flex;
  align-items: center;
  gap: 8px;
}

.voice-tip {
  font-size: 12px;
  color: #e6a23c;
  margin-left: 4px;
}

.listening {
  background: #f56c6c !important;
  border-color: #f56c6c !important;
  color: #fff !important;
  animation: pulse 1.2s infinite;
}

@keyframes pulse {
  0%, 100% {
    box-shadow: 0 0 0 0 rgba(245, 108, 108, 0.5);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(245, 108, 108, 0);
  }
}

.input-row {
  display: flex;
  gap: 8px;
  align-items: flex-end;
}

.input-row .el-textarea {
  flex: 1;
}

.send-btn {
  height: 48px;
}

.generate-btn {
  width: 100%;
  height: 44px;
  font-size: 15px;
}

/* 生成结果 */
.gen-alert {
  margin-bottom: 16px;
}

.gen-files {
  margin-top: 16px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 12px 16px;
}

.gen-files-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 10px;
}

.gen-file-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  font-size: 13px;
}

.gen-file-path {
  flex: 1;
  color: #303133;
  font-family: Consolas, monospace;
}

.gen-file-size {
  color: #909399;
  font-size: 12px;
}

.gen-preview {
  margin-top: 16px;
}

.gen-preview-code {
  background: #282c34;
  color: #abb2bf;
  padding: 14px;
  border-radius: 8px;
  font-size: 12px;
  line-height: 1.6;
  max-height: 320px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.gen-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 20px;
}

@media (max-width: 640px) {
  .mode-grid {
    grid-template-columns: 1fr;
  }
}
</style>
