# WowSkillsLand 前端美化交接说明

## 项目概况

- 技术栈：React 18、TypeScript、Vite、React Router、Tailwind CSS、Lucide React
- 数据：`localStorage` 演示数据，键名为 `wowskillsland-demo-v1`
- 当前定位：中文校园 Skill 社区多页面交互原型
- 当前视觉：白色与钴蓝为主，冰川流体背景，Geist 字体

## 启动方式

```powershell
npm install
npm run dev -- --host 127.0.0.1 --port 4173
```

生产检查：

```powershell
npm run build
npm run preview -- --host 127.0.0.1 --port 4173
```

## 页面路由

- `/`：任务匹配首页
- `/explore`：Skill、经验动态、赛博人格探索页
- `/creator`：Bonjour 式个人创作者主页、成长时间线、资产工坊和赛博人格管理
- `/trust`：可信评测浏览与模拟提交
- `/u/:handle`：公开个人主页与赛博人格入口
- `/share/:type/:id`：仅链接内容

## 主要文件

- `src/components/AppShell.tsx`：共享导航、账号、通知和登录弹窗
- `src/components/IceFlowBackground.tsx`：动态背景
- `src/pages/*.tsx`：各路由页面
- `src/store/DemoStore.tsx`：跨页面状态和持久化
- `src/data/seed.ts`：演示数据
- `src/index.css`：当前全部视觉样式和响应式规则
- `assets/skillx-glass-data-flow.png`：当前动态背景基础纹理

## 美化时请保留

- 现有路由和中文导航名称
- 成长记录的公开、仅链接、私密三级可见性
- 记录对资产生成和赛博人格的独立授权
- 赛博人格确认、测试对话和公开入口状态
- 可信评测维度和社交操作
- `390px`、`768px`、`1440px` 三档响应式可用性

## 当前视觉方向

- 首页按原始 Wandor 提示词的单屏 Hero 比例实现
- 探索页采用轻量经验房间卡片网格
- 创作者中心采用左侧个人档案卡、右侧成长时间线的 Bonjour 式布局
- 不使用棕色背景，不直接使用带水印的参考图

## 交接包说明

压缩包已排除 `node_modules`、`dist`、`.screenshots`、旧静态原型、构建缓存和临时检查图片。执行 `npm install` 后即可运行。
