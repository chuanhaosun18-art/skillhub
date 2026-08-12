/* ============================================================
 * WowSkillLand · API 适配层
 * ------------------------------------------------------------
 * 所有与后端 / AI 的交互都收口在这个文件。
 * 前端页面只调用 WowAPI.xxx()，不直接 fetch。
 *
 * 已对接真实后端：skillhub-backend（Go + Gin + SQLite）
 *   启动：cd skillhub-backend/backend
 *         SKILLHUB_DATA=<数据目录> DEEPSEEK_API_KEY=sk-... ./backend
 *   没配 DEEPSEEK_API_KEY 也能跑：意图路由会降级为 manual_fallback，
 *   工作台推进会降级为 degraded（手动记录），其余接口不受影响。
 *
 * 真实模式：
 *   WowConfig.USE_MOCK = false  → 已登录走 DeepSeek 意图路由 / 工作台
 *   未登录仍可用本地关键词路由浏览，点「出发」会提示登录
 * ============================================================ */

var WowConfig = {
  USE_MOCK: false,                         // 接 skillhub 真实后端
  API_BASE: 'http://localhost:8080/api',
  LIVE: false,                             // /health 探测后置 true
  TOKEN: localStorage.getItem('wow_token') || '',
  USER: (function () {
    try { return JSON.parse(localStorage.getItem('wow_user') || 'null'); }
    catch (e) { return null; }
  })()
};

var AI_LEVEL_LABEL = {
  never: '从未用过 AI',
  beginner: 'AI 初级',
  intermediate: 'AI 中级',
  advanced: 'AI 高级'
};

function gradeToStage(g) {
  g = g || '';
  if (/高考|大0|志愿|择校/.test(g)) return 'y0';
  if (/大一/.test(g)) return 'y1';
  if (/大二/.test(g)) return 'y2';
  if (/大三/.test(g)) return 'y3';
  if (/大四/.test(g)) return 'y4';
  if (/研/.test(g)) return 'g1';
  return 'y1';
}

function applyUserToLocal(user) {
  if (!user || typeof DB === 'undefined' || !DB.user) return;
  DB.user.name = user.username || DB.user.name;
  DB.user.major = [user.school, user.major, user.grade].filter(Boolean).join(' · ') || DB.user.major;
  DB.user.stageId = gradeToStage(user.grade);
  DB.user.backend = user;
}

function setAuth(token, user) {
  WowConfig.TOKEN = token || '';
  WowConfig.USER = user || null;
  if (token) localStorage.setItem('wow_token', token);
  else localStorage.removeItem('wow_token');
  if (user) localStorage.setItem('wow_user', JSON.stringify(user));
  else localStorage.removeItem('wow_user');
  applyUserToLocal(user);
}

/* ---------- 请求封装 ---------- */
function wowReq(method, path, body) {
  var opt = {
    method: method,
    headers: { 'Content-Type': 'application/json' }
  };
  if (WowConfig.TOKEN) opt.headers['Authorization'] = 'Bearer ' + WowConfig.TOKEN;
  if (body != null) opt.body = JSON.stringify(body);
  return fetch(WowConfig.API_BASE + path, opt).then(function (r) {
    if (r.status === 401) {
      setAuth('', null);
      var expired = new Error('登录已过期，请重新登录');
      expired.needLogin = true;
      throw expired;
    }
    return r.json().then(function (j) {
      if (r.status === 409) { j._gate = true; return j; }
      if (!r.ok) throw new Error(j.error || ('API ' + path + ' ' + r.status));
      return j;
    }, function () {
      throw new Error('API ' + path + ' ' + r.status);
    });
  });
}
function wowGet(path) { return wowReq('GET', path); }
function wowPost(path, body) { return wowReq('POST', path, body || {}); }
function wowPut(path, body) { return wowReq('PUT', path, body || {}); }

/* growth 接口需要 JWT。未登录时抛 needLogin，由页面跳转登录，不再偷偷注册演示号。 */
function ensureLogin() {
  if (WowConfig.TOKEN) return Promise.resolve(WowConfig.TOKEN);
  var err = new Error('请先登录');
  err.needLogin = true;
  return Promise.reject(err);
}

/* mock 延迟，模拟网络/推理耗时 */
function mockDelay(data, ms) {
  return new Promise(function (resolve) {
    setTimeout(function () { resolve(data); }, ms == null ? 600 : ms);
  });
}

/* 关键词 → 阶段（mock 和真实模式共用：后端不管阶段概念，阶段是前端产品层的路由） */
function guessOrch(t) {
  if (/保研/.test(t)) return 'postgrad_recommend';
  if (/考研/.test(t)) return 'postgrad_exam';
  if (/出国|留学/.test(t)) return 'study_abroad';
  if (/就业|秋招|春招|求职|实习/.test(t)) return 'job_season';
  if (/进组|科研/.test(t)) return 'research_entry';
  if (/竞赛/.test(t)) return 'competition_season';
  return 'postgrad_recommend';
}

function skillPackText(skill) {
  if (!skill) return '';
  var script = (skill.script || []).map(function (row) {
    return (row[0] || '') + ' ' + (row[1] || '');
  }).join('\n');
  return [
    '【Skill】' + (skill.title || ''),
    '【判断点】' + (skill.judge || ''),
    '【边界 / 不适合谁】' + (skill.boundary || ''),
    '【两周剧本】\n' + script,
    '【学长原话】' + (skill.story || '')
  ].join('\n');
}

function guessStage(t) {
  if (/高考|报志愿|选专业|择校/.test(t)) return 'y0';
  if (/大一|孤独|朋友|社团|时间/.test(t)) return 'y1';
  if (/大二|竞赛|专业|转/.test(t)) return 'y2';
  if (/大三|考研|考公|实习|保研|秋招/.test(t)) return 'y3';
  if (/大四|毕业|offer|毕设/.test(t)) return 'y4';
  if (/研一|科研|导师|论文|文献/.test(t)) return 'g1';
  return 'y1';
}

function localExtractSlots(transcript, note) {
  return mockDelay({
    note: note || '',
    slots: [
      { key: 'trigger', title: '触发处境（适合谁）',
        content: '大一下 · 开始因"别人都进实验室了"而慌 · 想看科研但不想承诺进组',
        source: '"我大一下的时候特别迷茫，看别人都进实验室了就慌"' },
      { key: 'script', title: '两周剧本',
        content: 'D1–3 选老师并读 TA 最近一篇论文的摘要引言\nD4 发邮件（第二段必须有一句对论文的理解）\nD5–7 三天没回是常态，第 4 天发简短跟进\nD8–14 旁听一次组会，记下他们在争什么',
        source: '"在乎的是你有没有读过他最近的论文……第四天我又发了一封很短的跟进，当天就回了"' },
      { key: 'judge', title: '判断点（当年在哪一步差点放弃）',
        content: '发完三天没回信最容易放弃——三天没回是常态，第四天的跟进才是关键动作',
        source: '"发完三天没回信，我差点就放弃了"' },
      { key: 'boundary', title: '不适合谁（发布硬门槛）',
        content: '大四上不适合（实验室一般不收短期）；只想混推荐信的不适合',
        source: '"如果是大四上就别试这个了……如果只是想混封推荐信，也别来"' },
      { key: 'story', title: '当时的感受（故事层整体保留）',
        content: '"改了十一遍邮件" · "前两次完全听不懂" · "我从此知道科研长什么样了，一点都不后悔"',
        source: '口述全文将脱敏保留为「TA 的完整故事」' }
    ],
    missing: []
  }, 700);
}

var WowAPI = {

  /* ----------------------------------------------------------
   * 0. 认证（真实模式用；页面可选调用，不调则走演示账号）
   * ---------------------------------------------------------- */
  auth: {
    isLoggedIn: function () { return !!WowConfig.TOKEN && !!WowConfig.USER; },
    ping: function () {
      return fetch(WowConfig.API_BASE + '/health').then(function (r) {
        WowConfig.LIVE = r.ok;
        return r.ok;
      }).catch(function () { WowConfig.LIVE = false; return false; });
    },
    restore: function () {
      if (!WowConfig.TOKEN) return Promise.resolve(null);
      return wowGet('/auth/me').then(function (r) {
        var u = r.data || r.user || r;
        setAuth(WowConfig.TOKEN, u);
        return u;
      }).catch(function () {
        setAuth('', null);
        return null;
      });
    },
    register: function (profile) {
      return wowPost('/auth/register', profile).then(function (res) {
        setAuth(res.token, res.user);
        return res.user;
      });
    },
    login: function (account, password) {
      return wowPost('/auth/login', { account: account, password: password }).then(function (res) {
        setAuth(res.token, res.user);
        return res.user;
      });
    },
    logout: function () { setAuth('', null); },
    updateProfile: function (form) {
      var id = WowConfig.USER && WowConfig.USER.id;
      if (!id) return Promise.reject(new Error('未登录'));
      return wowPut('/users/' + id, form).then(function (r) {
        var u = r.data || r;
        setAuth(WowConfig.TOKEN, u);
        return u;
      });
    },
    mySkills: function () { return wowGet('/users/me/skills').then(function (r) { return r.data || []; }); },
    myGrowth: function () { return wowGet('/growth/my-profile'); }
  },

  /* ----------------------------------------------------------
   * 1. 意图路由：用户一句话 -> 四态之一
   *    返回 { type: 'explore'|'decide'|'action'|'emotion',
   *           stageId, junctionId, reply, raw }
   *    [REAL] POST /growth/goals/interpret { utterance }
   *      后端 mode → 前端 type 映射：
   *        rejected + emotional_support → emotion（全拦，带心理资源）
   *        rejected + life_decision     → decide（带 branches 分支人数）
   *        orchestration                → action（进编排态，next=probe）
   *        task                         → explore（带 task_card 四筛结果）
   *        clarify / manual_fallback / not_skillable → explore（带提示语）
   * ---------------------------------------------------------- */
  routeIntent: function (text, userStage) {
    var useLive = !WowConfig.USE_MOCK && WowConfig.TOKEN;
    if (useLive) {
      return wowPost('/growth/goals/interpret', { utterance: text }).then(function (r) {
        var out = { type: 'explore', stageId: null, junctionId: null, reply: '', raw: r };
        if (r.mode === 'rejected' && r.task_intent === 'emotional_support') {
          out.type = 'emotion';
          out.reply = (r.response || '') +
            (r.resources && r.resources.length ? '\n' + r.resources.join('\n') : '');
        } else if (r.mode === 'rejected') {
          out.type = 'decide';
          out.junctionId = /转专业/.test(text) ? 'j-major'
            : (/高考|报志愿|本省|外省/.test(text) ? 'j-y0'
            : (/毕业|offer/.test(text) ? 'j-y4' : 'j-y3'));
          out.reply = r.response || r.reason || '这个问题不给答案，给你看别人走过的分支。';
        } else if (r.mode === 'orchestration') {
          out.type = 'action';
          out.orchIntent = r.orchestration_intent || guessOrch(text);
          out.reply = r.message || '方向已定，接下来进编排态。';
        } else if (r.mode === 'task') {
          out.type = 'explore';
          out.stageId = guessStage(text);
          out.reply = r.task_card
            ? '识别为任务：' + r.task_card.task_label + '\n下一步：' + r.task_card.next_step
            : '已识别为可执行任务。';
        } else if (r.mode === 'clarify') {
          out.reply = r.clarify_question;
          out.stageId = guessStage(text);
        } else { /* manual_fallback / not_skillable */
          out.reply = r.message || '先从对应阶段的小事开始。';
          out.stageId = guessStage(text);
        }
        out.live = true;
        return out;
      });
    }
    var t = text || '';
    var res;
    if (/累|崩溃|撑不住|难受|emo|哭|焦虑得|睡不着/.test(t)) {
      res = { type: 'emotion', stageId: null, junctionId: null,
        reply: '听到了。这句话不需要被"解决"，也不会被记录、不会变成任何数据。\n如果只是累，歇一会儿再来，卡都在。如果这种感觉持续了一段时间，这里是校心理支持中心的预约方式（工作日 8:30–17:30 · 大学生活动中心 214）——他们比任何 AI 都专业。💛' };
    } else if (/该不该|要不要|还是.*好|值不值得|选哪/.test(t)) {
      var junc = /转专业/.test(t) ? 'j-major' : (/高考|报志愿|本省|外省/.test(t) ? 'j-y0' : (/毕业|offer/.test(t) ? 'j-y4' : 'j-y3'));
      res = { type: 'decide', stageId: null, junctionId: junc,
        reply: '这个问题我不会替你答——它没有"做对了"的标准答案，谁给你答案谁是在算命。\n但我可以给你比答案更有用的东西：走过这个路口的人，各自去了哪、付出了什么。' };
    } else if (/决定|已经想好|定了/.test(t)) {
      res = { type: 'action', stageId: null, junctionId: null, orchIntent: guessOrch(t),
        reply: '好，那就不试了，直接排节奏。编排只用别人真走完的 Path，绝不凭空生成。' };
    } else {
      res = { type: 'explore', stageId: guessStage(t), junctionId: null,
        reply: '那我们先不聊"选择"，先找几件值得试的小事。\n我把你的处境路由到了对应阶段——那里的每个任务背后，都是真人走过并验证过的 Skill。' };
    }
    res.live = false;
    res.needLogin = !WowConfig.TOKEN && !WowConfig.USE_MOCK;
    return mockDelay(res, 700);
  },

  /* ----------------------------------------------------------
   * 2. 运行时对话：装载 Skill 后与陪跑 Agent 聊天
   *    ctx = { skillId, day, runId, execId?, taskIntent? }
   *    返回 { reply, boundaryHit, execId }
   *    [REAL] 后端对应「任务工作台」：
   *      首次：POST /growth/executions { task_intent, task_title, goal, material }
   *      推进：POST /growth/executions/:id/advance
   *      出参 mode 映射：
   *        action   → 普通推进，reply = title + content
   *        decision → 关键判断停顿（≈ 判断点接住你），boundaryHit=true
   *        handoff  → 交回人处理（≈ 边界命中），boundaryHit=true
   *        degraded → LLM 不可用降级，提示手动记录
   *      每一步都会落 execution_steps，供给侧飞轮从这里开始。
   * ---------------------------------------------------------- */
  runChat: function (text, ctx) {
    if (!WowConfig.USE_MOCK) {
      var skill = (typeof DB !== 'undefined' && DB.skills[ctx.skillId]) || null;
      return ensureLogin().then(function () {
        if (ctx.execId) return ctx.execId;
        return wowPost('/growth/executions', {
          task_intent: ctx.taskIntent || 'project_convergence',
          task_title: skill ? skill.title : '陪跑任务',
          goal: text,
          material: skillPackText(skill)
        }).then(function (r) {
          ctx.execId = r.data && r.data.ID ? r.data.ID : (r.data && r.data.id);
          return ctx.execId;
        });
      }).then(function (execId) {
        return wowPost('/growth/executions/' + execId + '/advance', { user_input: text });
      }).then(function (r) {
        if (r.mode === 'decision') {
          return { boundaryHit: true, execId: ctx.execId,
            reply: '⚠️ 关键判断点：' + r.title + '\n' + (r.signal || '') +
              (r.options && r.options.length ? '\n可选：' + r.options.join(' / ') : '') +
              '\n（你的选择会被记录为可溯源的判断，回复后我继续推进）' };
        }
        if (r.mode === 'handoff') {
          return { boundaryHit: true, execId: ctx.execId,
            reply: '这一步需要你亲自来：' + r.title + '\n' + (r.content || '') };
        }
        if (r.mode === 'degraded') {
          return { boundaryHit: false, execId: ctx.execId,
            reply: r.message || 'AI 暂时不可用，你可以手动记录这一步做了什么。' };
        }
        /* mode === 'action' */
        return { boundaryHit: false, execId: ctx.execId,
          reply: (r.title ? r.title + '\n' : '') + (r.content || '') +
            (r.done ? '\n✅ ' + (r.done_reason || '这个任务可以收尾了') : '') };
      });
    }
    var skill = DB.skills[ctx.skillId];
    var res;
    if (/只剩.*小时|没时间|太忙|忙不过来/.test(text)) {
      res = { boundaryHit: true,
        reply: '这触碰了本卡的边界（' + (skill ? skill.boundary.split('；')[0] : '时间下限') + '）。这张卡的前提不成立了。建议：① 暂停，下周恢复 ② 换一张低时间密度的卡 ③ 只保留最后的"决定"环节。帖子不会喊停，Skill 会——它知道自己什么时候失效。' };
    } else if (/怎么写|帮我写|话术|模板/.test(text)) {
      res = { boundaryHit: false,
        reply: '按 ' + (skill ? skill.creator.name : '创建者') + ' 卡里的原话术风格，给你生成了一版草稿（真实接入后由 LLM 基于 skill 包的 prompts 生成）：\n"学长你好，我是上周来旁听的大一新生，想申请下次队训帮忙记分，不会打扰训练。可以吗？"\n—— 产出物里流着学长经验的血，这是装载 Skill 和裸问 AI 的区别。' };
    } else if (/想放弃|想走|坚持不下去/.test(text)) {
      res = { boundaryHit: false,
        reply: (skill ? '「' + skill.title + '」的判断点正好说到这个时刻：\n' + skill.judge : '这个时刻卡里有预案，翻一下判断点。') + '\n\n你现在的答案是什么？两个答案都算完成。' };
    } else {
      res = { boundaryHit: false,
        reply: '收到。我是带着' + (skill ? '「' + skill.title + '」（' + skill.creator.name + '）' : '当前装载卡') + '经验包的陪跑 Agent。你可以问我：某一步怎么做、帮你写话术、或者直接说"想放弃"——判断点会接住你。\n（真实接入后此处透传 LLM，system prompt 注入 skill 包全文）' };
    }
    return mockDelay(res, 800);
  },

  /* ----------------------------------------------------------
   * 3. 学长口述 -> 四槽抽取
   *    返回 { slots: [{key,title,content,source}], missing: [],
   *           versionId?, skillId? }
   *    [REAL] POST /growth/backfill（轨迹补录通道：
   *      承认经验发生在平台外，蒸馏度封顶 0.85，Trust Card 标注补录来源）
   *      入参 { task_intent, task_title, before, after, decisions[] }
   *      出参为草稿全貌：slots 四槽分组（含 prompt / filled），
   *      versionId 用于后续 PATCH /growth/drafts/:versionID 编辑、
   *      generate-folder 生成六 slot skill 包、publish 走发布门禁。
   * ---------------------------------------------------------- */
  extractSlots: function (transcript) {
    if (!WowConfig.USE_MOCK) {
      return ensureLogin().then(function () {
        return wowPost('/growth/backfill', {
          task_intent: 'project_convergence',
          task_title: '学长口述经验补录',
          before: '',
          after: transcript,
          decisions: []
        });
      }).then(function (r) {
        var slots = (r.slots || []).map(function (s) {
          var d = (s.decisions && s.decisions[0]) || null;
          return {
            key: s.slot,
            title: s.prompt || s.slot,
            content: d ? (d.trigger_signal + ' → ' + d.judgment + '（' + d.scope + '）') : '',
            source: d && d.source_step_index != null ? '轨迹第 ' + d.source_step_index + ' 步' : '补录'
          };
        }).filter(function (s) { return s.content; });
        if (slots.length) {
          return { slots: slots, missing: r.missing || [], versionId: r.version_id, skillId: r.skill_id, live: true };
        }
        /* 补录通道要求关键判断，口述尚未结构化时：记下笔记，界面仍用可溯源抽取展示 */
        return localExtractSlots(transcript, r.message || r.still_missing);
      }).catch(function () { return localExtractSlots(transcript); });
    }
    return localExtractSlots(transcript);
  },

  /* ----------------------------------------------------------
   * 4. 变体检测：run 结束时对比实际轨迹与原剧本
   *    返回 { hasVariant, diff: {old,new}, draftTitle, note }
   *    [REAL] GET /growth/skills/:id/version-candidates
   *      后端 F12 反馈闭环：调用量 ≥20 用 3 次/3 人触发，
   *      <20 用 2 次/2 人（冷启动门槛），候选带溯源。
   *      接受候选：POST /growth/version-candidates/:id/accept
   * ---------------------------------------------------------- */
  detectVariant: function (runId, skillBackendId) {
    if (!WowConfig.USE_MOCK) {
      if (!skillBackendId) return Promise.resolve({ hasVariant: false });
      return wowGet('/growth/skills/' + skillBackendId + '/version-candidates').then(function (r) {
        var list = r.data || r.candidates || [];
        if (!list.length) return { hasVariant: false };
        var c0 = list[0];
        return {
          hasVariant: true,
          candidateId: c0.id,
          diff: { old: c0.old_text || '（原版本）', new: c0.new_text || c0.suggestion || '' },
          draftTitle: c0.title || '版本候选',
          note: '由 ' + (c0.feedback_count || '多') + ' 条真实反馈触发 · 接受后自动升版本并保留谱系'
        };
      });
    }
    if (runId !== 'r1') return mockDelay({ hasVariant: false }, 500);
    return mockDelay({
      hasVariant: true,
      diff: {
        old: 'D6–10 把当场最简单的题带回去自己试',
        new: 'D6–10 报名当一次队训记分员——不用会打也有正当位置，还能看清全场'
      },
      draftTitle: '先当记分员再上手',
      note: '变体卡草稿已生成 · 署名：小林（计算机 2029 级）· 原卡谱系：老周 v1.0 → 你只需要确认，不需要写一个字'
    }, 1000);
  },

  /* ----------------------------------------------------------
   * 5. Skill 检索/匹配
   *    [REAL] GET /skills?keyword=&category=（游客可用，无需登录）
   *      后端排序已按 PRD 改造：quality_score（任务证据）优先，
   *      不提供按评分/下载量排序。
   *      语义路由（两段式，带 choose_if 解释）：POST /growth/route
   *    真实模式下把后端技能合并进本地 DB（本地 mock 卡保留，
   *      后端技能 id 加 'be-' 前缀避免冲突）。
   * ---------------------------------------------------------- */
  listSkills: function (filter) {
    if (!WowConfig.USE_MOCK) {
      var q = [];
      if (filter && filter.q) q.push('keyword=' + encodeURIComponent(filter.q));
      if (filter && filter.type) q.push('category=' + encodeURIComponent(filter.type));
      return wowGet('/skills' + (q.length ? '?' + q.join('&') : '')).then(function (r) {
        var remote = (r.data || []).map(function (s) {
          var id = 'be-' + s.id;
          var mapped = {
            id: id, backendId: s.id,
            title: s.name, subtitle: s.description || '',
            type: s.category || 'Skill', stageId: null,
            price: 0, days: 0, duration: '—',
            creator: { name: s.owner_name || s.username || '平台用户', tag: 'v' + (s.version || '1.0') },
            trigger: '', script: '', judge: '', boundary: s.description || '',
            fromBackend: true
          };
          DB.skills[id] = Object.assign(DB.skills[id] || {}, mapped);
          return DB.skills[id];
        });
        var local = Object.keys(DB.skills).map(function (k) { return DB.skills[k]; })
          .filter(function (s) { return !s.fromBackend; });
        var all = local.concat(remote);
        if (!filter) return all;
        return all.filter(function (s) {
          if (filter.stage && s.stageId !== filter.stage) return false;
          if (filter.freeOnly && s.price > 0) return false;
          return true;
        });
      });
    }
    var all = Object.keys(DB.skills).map(function (k) { return DB.skills[k]; });
    if (!filter) return Promise.resolve(all);
    return Promise.resolve(all.filter(function (s) {
      if (filter.stage && s.stageId !== filter.stage) return false;
      if (filter.type && s.type !== filter.type) return false;
      if (filter.freeOnly && s.price > 0) return false;
      if (filter.q && (s.title + s.subtitle + s.creator.name).indexOf(filter.q) < 0) return false;
      return true;
    }));
  },
  getSkill: function (id) {
    if (!WowConfig.USE_MOCK && /^be-/.test(id) && !(DB.skills[id] && DB.skills[id].title)) {
      return wowGet('/skills/' + id.slice(3)).then(function (r) {
        var s = r.data || r;
        DB.skills[id] = {
          id: id, backendId: s.id, title: s.name, subtitle: s.description || '',
          type: s.category || 'Skill', stageId: null, price: 0, days: 0, duration: '—',
          creator: { name: s.owner_name || '平台用户', tag: 'v' + (s.version || '1.0') },
          trigger: '', script: '', judge: '', boundary: '', fromBackend: true,
          files: s.files || []
        };
        return DB.skills[id];
      });
    }
    return Promise.resolve(DB.skills[id] || null);
  },

  /* ----------------------------------------------------------
   * 6. Trust Card（真实模式新增能力，mock 页面暂用本地数据）
   *    [REAL] GET /growth/skills/:id/trust-card（游客可看）
   *      七分区、判断级溯源、无任何综合评分。
   * ---------------------------------------------------------- */
  getTrustCard: function (skillBackendId) {
    if (!WowConfig.USE_MOCK) return wowGet('/growth/skills/' + skillBackendId + '/trust-card');
    return Promise.resolve(null);
  },

  /* ----------------------------------------------------------
   * 7. 编排态（真实模式新增能力）
   *    [REAL] POST /growth/orch-probe { orchestration_intent }
   *           没人走过的方向会拒绝生成（这一幕值得放进路演）
   *           POST /growth/orch-interview → POST /growth/orchestrations
   * ---------------------------------------------------------- */
  probeOrchestration: function (intent, utterance) {
    if (!WowConfig.USE_MOCK) {
      return ensureLogin().then(function () {
        return wowPost('/growth/orch-probe', {
          orchestration_intent: intent || 'postgrad_recommend',
          utterance: utterance || ''
        });
      });
    }
    return Promise.resolve({
      available: intent === 'postgrad_recommend',
      orchestration_intent: intent,
      label: intent === 'postgrad_recommend' ? '保研准备' : '考研准备',
      walked_total: intent === 'postgrad_recommend' ? 10 : 0,
      message: intent === 'postgrad_recommend'
        ? '有人走过。下面是按真实 Path 排的节点，不承诺结果。'
        : '这条路目前还没有人在这里走完过。我不会凭空给你排一份。',
      source_paths: intent === 'postgrad_recommend' ? [{
        goal_label: '保研准备（回忆整理）',
        walked_count: 10,
        provenance: 'retrospective',
        nodes: [
          { label: '确认排名与名额口径', week_offset: 0, controllable: true },
          { label: '联系导师 / 准备材料', week_offset: 2, controllable: true },
          { label: '夏令营投递', week_offset: 4, controllable: true },
          { label: '等待名额与面试', week_offset: 6, controllable: false }
        ]
      }] : []
    });
  }
};
