// 单一入口 Agent：用户始终只面对一个对话流，由 agent 决定下一步给什么卡片。
//
// 这个文件的设计有两条刻意的边界，破了任何一条都会出问题：
//
//  1. **状态驱动用规则，意图识别才用模型。**
//     「你现在该干什么」完全可以从数据库推出来，是确定性的——演一百遍都一样。
//     只有「用户新说了一句话」才调模型。如果每一步都让模型决定，Demo 现场判偏一次就断链。
//
//  2. **agent 不代理业务调用。**
//     它只回答「下一步是什么」并装配卡片；真正的动作还是由前端打原有端点。
//     所有门禁、硬约束、口径都留在原处，agent 碰不到，也就改不坏。
package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 卡片类型。前端按 type 分发渲染。
const (
	CardIdle           = "idle"            // 没有待办，引导说一句话
	CardContinueTask   = "continue_task"   // 有进行中的执行
	CardVerdict        = "verdict"         // 用了别人的 Skill 还没表态
	CardDistill        = "distill"          // 执行完了可以固化
	CardDraftGate      = "draft_gate"      // 草稿卡在门禁
	CardUpgrade        = "upgrade"          // 有版本候选待处理
	CardOrchReview     = "orch_review"     // 编排本周待复核
	CardResumeReady    = "resume_ready"    // 有资产但还没导出过简历
	CardTask           = "task"            // 任务卡（来自意图识别）
	CardOrchEntry      = "orch_entry"      // 编排态入口
	CardReject         = "reject"          // 拒绝态
	CardNotSkillable   = "not_skillable"   // 四筛没过
	CardClarify        = "clarify"         // 需要澄清一轮
	CardManualFallback = "manual_fallback" // 模型不可用，手选
)

// agentCard 统一的卡片结构。data 里放该卡片需要的业务数据，
// actions 告诉前端可以打哪个原有端点——agent 自己不打。
type agentCard struct {
	Type    string   `json:"type"`
	Say     string   `json:"say"`               // agent 说的那句话
	Why     string   `json:"why,omitempty"`     // 为什么现在给这张卡（可解释性）
	Data    gin.H    `json:"data,omitempty"`
	Actions []gin.H  `json:"actions,omitempty"` // {label, method, path, primary}
	Deep    string   `json:"deep_link,omitempty"` // 想看完整页面时的深链
}

// ---------- GET /api/growth/agent/state ----------

// agentState 纯规则推导「现在该做什么」。
//
// 优先级顺序本身就是产品主张：前台只卖下一步，所以永远只给一件事。
// 顺序的理由写在每个分支上，改顺序等于改产品行为，不要随手动。
func agentState(c *gin.Context) {
	uid := c.GetInt64("userID")

	// ① 有进行中的执行 —— 做事优先于一切。别的都能等，手上的活不能撂着。
	if card := checkRunningExecution(uid); card != nil {
		respondCard(c, card)
		return
	}

	// ② 用了别人的 Skill 但还没表态 —— 这是调用的代价，而且它影响采纳率口径。
	//    放在固化之前，因为它是「欠别人的」，固化是「为自己的」。
	if card := checkPendingVerdict(uid); card != nil {
		respondCard(c, card)
		return
	}

	// ③ 执行完了且可以固化 —— 供给是执行的副产品，趁热打铁。
	if card := checkDistillable(uid); card != nil {
		respondCard(c, card)
		return
	}

	// ④ 草稿卡在门禁 —— 已经投入过的东西别烂在半路。
	if card := checkDraftGate(uid); card != nil {
		respondCard(c, card)
		return
	}

	// ⑤ 有版本候选待处理 —— 维护者的责任，也是闭环的最后一步。
	if card := checkVersionCandidate(uid); card != nil {
		respondCard(c, card)
		return
	}

	// ⑥ 编排本周还没复核 —— 低频但重要，它是编排唯一的有效性证据。
	if card := checkOrchReview(uid); card != nil {
		respondCard(c, card)
		return
	}

	// ⑦ 有资产但从没导出过简历 —— 这是对贡献者的回报，最后提醒。
	if card := checkResumeReady(uid); card != nil {
		respondCard(c, card)
		return
	}

	// ⑧ 什么都没有 —— 回到唯一的入口：说一句你现在卡在哪。
	respondCard(c, &agentCard{
		Type: CardIdle,
		Say:  "说一句你现在卡在哪就行，用你自己的话。不用先想清楚需要什么能力，那是我的事。",
		Data: gin.H{
			"examples": []string{
				"我的选题被导师退了两次，说范围太大",
				"科研经历怎么写进产品岗的简历",
				"我决定保研了，接下来几周该做什么",
			},
		},
	})
}

func respondCard(c *gin.Context, card *agentCard) {
	c.JSON(http.StatusOK, gin.H{"card": card})
}

// ---------- 各个规则分支 ----------

func checkRunningExecution(uid int64) *agentCard {
	var id int64
	var intent, title string
	var steps int
	if err := db.QueryRow(`SELECT id, task_intent, COALESCE(task_title,'') FROM executions
		WHERE user_id = ? AND status = ? ORDER BY id DESC LIMIT 1`,
		uid, ExecRunning).Scan(&id, &intent, &title); err != nil {
		return nil
	}
	db.QueryRow(`SELECT COUNT(*) FROM execution_steps WHERE execution_id = ?`, id).Scan(&steps)

	// 是不是正卡在一个待决策的判断上
	var pendingStep int
	var slot, signal, options string
	waiting := db.QueryRow(`SELECT step_index, decision_slot, input, output FROM execution_steps
		WHERE execution_id = ? AND step_type = ? AND user_choice = ''
		ORDER BY step_index LIMIT 1`, id, StepUserDecision).
		Scan(&pendingStep, &slot, &signal, &options) == nil

	if waiting {
		return &agentCard{
			Type: CardContinueTask,
			Say:  "这里需要你自己判断一下，我不替你选。",
			Why:  "遇到了关键判断点。你这一选会成为这个方法里的一条判断，所以不能由我来定。",
			Data: gin.H{
				"execution_id": id, "task_label": AllowedIntents[intent], "task_title": title,
				"awaiting_decision": true,
				"step_index":        pendingStep,
				"slot":              slot,
				"slot_prompt":       slotPrompt(slot),
				"signal":            signal,
				"options":           rawOrDefault(options, "[]"),
			},
			Actions: []gin.H{
				{"label": "就这么定", "method": "POST",
					"path": "/api/growth/executions/" + itoa(id) + "/decide", "primary": true},
			},
			Deep: "/workbench?id=" + itoa(id),
		}
	}

	return &agentCard{
		Type: CardContinueTask,
		Say:  "你手上这件事还没做完，接着往下走吧。",
		Why:  "有一个进行中的任务。做事优先于其他所有事——别的都能等。",
		Data: gin.H{
			"execution_id": id, "task_label": AllowedIntents[intent],
			"task_title": title, "step_count": steps,
		},
		Actions: []gin.H{
			{"label": "下一步", "method": "POST",
				"path": "/api/growth/executions/" + itoa(id) + "/advance", "primary": true},
			{"label": "我做完了", "method": "POST",
				"path": "/api/growth/executions/" + itoa(id) + "/complete"},
			{"label": "先不做了", "method": "POST",
				"path": "/api/growth/executions/" + itoa(id) + "/abandon"},
		},
		Deep: "/workbench?id=" + itoa(id),
	}
}

func checkPendingVerdict(uid int64) *agentCard {
	// 找一次「用了别人的 Skill、已完成、但没有任何表态」的执行
	var execID, versionID int64
	if err := db.QueryRow(`SELECT e.id, e.skill_version_id FROM executions e
		WHERE e.user_id = ? AND e.status = ? AND e.skill_version_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM decision_verdicts dv WHERE dv.execution_id = e.id)
		ORDER BY e.id DESC LIMIT 1`, uid, ExecCompleted).Scan(&execID, &versionID); err != nil {
		return nil
	}
	decisions := loadDecisions(versionID)
	if len(decisions) == 0 {
		return nil
	}
	var skillName string
	db.QueryRow(`SELECT s.name FROM skills s JOIN skill_versions v ON v.skill_id = s.id
		WHERE v.id = ?`, versionID).Scan(&skillName)

	items := []gin.H{}
	for _, d := range decisions {
		if d.InvalidatedAt != nil {
			continue
		}
		items = append(items, gin.H{
			"decision_id": d.ID, "slot_prompt": slotPrompt(d.Slot),
			"statement": "当" + d.TriggerSignal + "时，" + d.Judgment,
			"scope":     d.Scope,
		})
	}
	return &agentCard{
		Type: CardVerdict,
		Say:  "你刚用了《" + skillName + "》。它里面这几条判断，在你的情况下成立吗？",
		Why:  "这是使用的代价。你留下一句，这个方法才会变准——而且没表态的调用不会计入它的采纳率。",
		Data: gin.H{"execution_id": execID, "skill_name": skillName, "decisions": items},
		Actions: []gin.H{
			{"label": "提交表态", "method": "POST",
				"path": "/api/growth/executions/" + itoa(execID) + "/verdicts", "primary": true},
		},
	}
}

func checkDistillable(uid int64) *agentCard {
	rows, err := db.Query(`SELECT id, task_intent, COALESCE(task_title,'') FROM executions
		WHERE user_id = ? AND status = ? ORDER BY id DESC LIMIT 5`, uid, ExecCompleted)
	if err != nil {
		return nil
	}
	type cand struct {
		id     int64
		intent string
		title  string
	}
	list := []cand{}
	for rows.Next() {
		var x cand
		if rows.Scan(&x.id, &x.intent, &x.title) == nil {
			list = append(list, x)
		}
	}
	rows.Close()

	for _, x := range list {
		// 已经固化过的跳过
		var done int
		db.QueryRow(`SELECT COUNT(*) FROM skill_versions WHERE source_execution_id = ?`, x.id).Scan(&done)
		if done > 0 {
			continue
		}
		exec, err := loadExecution(x.id)
		if err != nil || !canDistill(exec) {
			continue
		}
		return &agentCard{
			Type: CardDistill,
			Say:  "刚才那件事你做成了。要不要把这次的方法留下来？下一个卡在同样地方的人用得上。",
			Why:  "这次执行里有你做过的关键判断，够蒸馏出一个别人能用的方法了。你只需要确认，不用写。",
			Data: gin.H{
				"execution_id": x.id, "task_label": AllowedIntents[x.intent], "task_title": x.title,
			},
			Actions: []gin.H{
				{"label": "把方法留下来", "method": "POST",
					"path": "/api/growth/executions/" + itoa(x.id) + "/distill", "primary": true},
				{"label": "这次先算了", "method": "SKIP", "path": ""},
			},
		}
	}
	return nil
}

func checkDraftGate(uid int64) *agentCard {
	var skillID, versionID int64
	var name, status string
	if err := db.QueryRow(`SELECT s.id, COALESCE(s.current_version_id,0), s.name, COALESCE(s.status,'')
		FROM skills s WHERE s.owner_id = ? AND COALESCE(s.status,'') IN (?, ?)
		ORDER BY s.id DESC LIMIT 1`,
		uid, SkillStatusDraft, SkillStatusGated).Scan(&skillID, &versionID, &name, &status); err != nil {
		return nil
	}
	if versionID == 0 {
		return nil
	}
	exec, ver, decisions, err := loadDraftParts(versionID)
	if err != nil {
		return nil
	}
	detail := computeDistill(exec, ver, decisions)
	ok, missing := detail.publishable()

	if !ok {
		return &agentCard{
			Type: CardDraftGate,
			Say:  "《" + name + "》还差一点就能发布了。",
			Why:  "蒸馏度还没到发布线。缺的这几项补上就行——补不齐也可以先存成经验笔记，不会丢。",
			Data: gin.H{
				"skill_id": skillID, "version_id": versionID, "skill_name": name,
				"score": detail.total(), "threshold": DistillationThreshold,
				"detail": detail, "still_missing": missing,
				"labels": dimensionLabels,
			},
			Actions: []gin.H{
				{"label": "去补齐", "method": "GOTO", "path": "/creator?v=" + itoa(versionID), "primary": true},
				{"label": "先存成经验笔记", "method": "POST",
					"path": "/api/growth/drafts/" + itoa(versionID) + "/downgrade"},
			},
			Deep: "/creator?v=" + itoa(versionID),
		}
	}

	// 蒸馏度够了，看四问跑没跑
	unrun := []string{}
	for _, t := range []string{EvalDiscoverability, EvalCompletion, EvalStability, EvalBoundaryStop} {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM eval_runs WHERE version_id = ? AND eval_type = ?`,
			versionID, t).Scan(&n)
		if n == 0 {
			unrun = append(unrun, evalLabel(t))
		}
	}
	return &agentCard{
		Type: CardDraftGate,
		Say:  "《" + name + "》够格了，还得过一道门禁才能进市场。",
		Why:  "发布是门禁不是按钮。四问里「遇到边界时是否知道停下来」必须 100% 通过，这是安全项。",
		Data: gin.H{
			"skill_id": skillID, "version_id": versionID, "skill_name": name,
			"score": detail.total(), "unrun": unrun,
		},
		Actions: []gin.H{
			{"label": "跑发布前四问", "method": "POST",
				"path": "/api/growth/skills/" + itoa(skillID) + "/evals/run", "primary": true},
			{"label": "看门禁全貌", "method": "GOTO", "path": "/gate?skill=" + itoa(skillID)},
		},
		Deep: "/gate?skill=" + itoa(skillID),
	}
}

func checkVersionCandidate(uid int64) *agentCard {
	var candID, skillID int64
	var rule, evidence, name string
	if err := db.QueryRow(`SELECT c.id, c.skill_id, c.trigger_rule, COALESCE(c.evidence,'{}'), s.name
		FROM version_candidates c JOIN skills s ON s.id = c.skill_id
		WHERE c.status = 'open' AND COALESCE(s.maintainer_id, s.owner_id) = ?
		ORDER BY c.id DESC LIMIT 1`, uid).Scan(&candID, &skillID, &rule, &evidence, &name); err != nil {
		return nil
	}
	why := "同类问题重复出现，说明不是个例。"
	if rule == "new_boundary_verified" {
		why = "有判断被两次独立执行判为不成立，说明它的适用边界比原来写的窄。"
	}
	return &agentCard{
		Type: CardUpgrade,
		Say:  "《" + name + "》该升一版了。",
		Why:  why + "单独一条反馈我不会来烦你——要重复出现或被独立验证过才会。",
		Data: gin.H{
			"candidate_id": candID, "skill_id": skillID, "skill_name": name,
			"trigger_rule": rule, "evidence": rawOrDefault(evidence, "{}"),
		},
		Actions: []gin.H{
			{"label": "接受并升级", "method": "POST",
				"path": "/api/growth/version-candidates/" + itoa(candID) + "/accept", "primary": true},
		},
		Deep: "/trust/" + itoa(skillID),
	}
}

func checkOrchReview(uid int64) *agentCard {
	var orchID int64
	var intent, goal string
	var horizon int
	if err := db.QueryRow(`SELECT id, orchestration_intent, goal_label, horizon_weeks
		FROM orchestrations WHERE user_id = ? AND status = ?
		ORDER BY id DESC LIMIT 1`, uid, OrchActive).Scan(&orchID, &intent, &goal, &horizon); err != nil {
		return nil
	}
	// 已复核到第几周
	var reviewed int
	db.QueryRow(`SELECT COALESCE(MAX(week_index), 0) FROM orchestration_reviews
		WHERE orchestration_id = ?`, orchID).Scan(&reviewed)
	next := reviewed + 1
	if next > horizon {
		return nil
	}
	var todo int
	db.QueryRow(`SELECT COUNT(*) FROM orchestration_items
		WHERE orchestration_id = ? AND week_index = ? AND controllable = 1`,
		orchID, next).Scan(&todo)
	if todo == 0 {
		return nil
	}
	return &agentCard{
		Type: CardOrchReview,
		Say:  "第 " + itoa(int64(next)) + " 周的事该过一遍了。",
		Why:  "编排唯一能证明有用的东西就是你会不会回来勾。没用的时间表，人不会连续三周回来。",
		Data: gin.H{
			"orchestration_id": orchID, "label": OrchestrationIntents[intent],
			"goal_label": goal, "week_index": next, "item_count": todo,
		},
		Actions: []gin.H{
			{"label": "复核这一周", "method": "GOTO",
				"path": "/orchestration?id=" + itoa(orchID), "primary": true},
		},
		Deep: "/orchestration?id=" + itoa(orchID),
	}
}

func checkResumeReady(uid int64) *agentCard {
	var published, verifiedDec int
	db.QueryRow(`SELECT COUNT(*) FROM skills
		WHERE COALESCE(maintainer_id, owner_id) = ? AND COALESCE(status,'') = ?`,
		uid, SkillStatusPublished).Scan(&published)
	db.QueryRow(`SELECT COUNT(*) FROM decisions
		WHERE origin_user_id = ? AND verified_by_count > 0`, uid).Scan(&verifiedDec)
	if published == 0 && verifiedDec == 0 {
		return nil
	}
	var snaps int
	db.QueryRow(`SELECT COUNT(*) FROM profile_snapshots WHERE user_id = ?`, uid).Scan(&snaps)
	if snaps > 0 {
		return nil // 已经导出过就不再催
	}
	return &agentCard{
		Type: CardResumeReady,
		Say:  "你现在的记录已经够撑起一份能力简历了。",
		Why:  "有 " + itoa(int64(published)) + " 个方法在被人用，" + itoa(int64(verifiedDec)) +
			" 条判断被验证过。这些是别人点开就能验真假的东西，比写「参与了某项目」有用。",
		Data: gin.H{"published_skills": published, "verified_decisions": verifiedDec},
		Actions: []gin.H{
			{"label": "看看我的能力简历", "method": "GOTO", "path": "/resume", "primary": true},
		},
		Deep: "/resume",
	}
}

// ---------- POST /api/growth/agent/say ----------

// agentSay 用户新说了一句话。这是唯一调模型的地方。
// 三态分发的规则与 interpretGoal 完全一致，这里只是把结果包成卡片。
func agentSay(c *gin.Context) {
	var body struct {
		Utterance string `json:"utterance"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	utterance := strings.TrimSpace(body.Utterance)
	if len([]rune(utterance)) < 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "再多说两句，我需要知道你现在卡在哪"})
		return
	}

	// 原话一律入语料库：它既是召回索引，也是需求缺口信号
	db.Exec(`INSERT INTO description_corpus (utterance, source) VALUES (?, 'agent_say')`, utterance)

	res, err := classifyIntent(utterance)
	if err != nil {
		options := []gin.H{}
		for k, v := range AllowedIntents {
			options = append(options, gin.H{"task_intent": k, "label": v})
		}
		respondCard(c, &agentCard{
			Type: CardManualFallback,
			Say:  "我暂时没法自动判断你的情况，直接选一个更接近的吧。",
			Data: gin.H{"options": options},
		})
		return
	}

	// ① 情绪类：立即返回，不再做任何后续判断
	if res.TaskIntent == IntentEmotionalSupport {
		respondCard(c, &agentCard{
			Type: CardReject,
			Say:  rejectionResponse(IntentEmotionalSupport),
			Why:  RejectedIntents[IntentEmotionalSupport],
			Data: gin.H{"resources": rejectionResources(IntentEmotionalSupport), "records_created": false},
		})
		return
	}

	// ② 「该不该」型：不给建议，只给别人走过的分支
	if res.TaskIntent == IntentLifeDecision && looksUndecided(utterance) {
		respondCard(c, &agentCard{
			Type: CardReject,
			Say:  rejectionResponse(IntentLifeDecision),
			Why:  RejectedIntents[IntentLifeDecision],
			Data: gin.H{
				"branches":        lifeDecisionBranches(),
				"records_created": false,
			},
		})
		return
	}

	// ③ 编排态：长周期方向
	if orchIntent, ok := OrchestrationRouteIntents[res.TaskIntent]; ok ||
		res.TaskIntent == IntentLifeDecision {
		if orchIntent == "" {
			orchIntent = guessOrchestrationIntent(utterance)
		}
		respondCard(c, &agentCard{
			Type: CardOrchEntry,
			Say:  "这件事的结果我不敢承诺，但接下来几周该做什么可以排清楚——用别人真走过的路来排。",
			Why:  "这不是一件能一次做完的事，所以我不给你一个任务，给你一份带时间的编排。",
			Data: gin.H{
				"orchestration_intent": orchIntent,
				"label":                OrchestrationIntents[orchIntent],
				"utterance":            utterance,
			},
			Actions: []gin.H{
				{"label": "看看有没有人走过这条路", "method": "POST",
					"path": "/api/growth/orch-probe", "primary": true},
			},
			Deep: "/orchestration?intent=" + orchIntent,
		})
		return
	}

	// ④ 其余拒绝类（如实时信息）
	if reason, rejected := RejectedIntents[res.TaskIntent]; rejected {
		respondCard(c, &agentCard{
			Type: CardReject,
			Say:  rejectionResponse(res.TaskIntent),
			Why:  reason,
			Data: gin.H{"resources": rejectionResources(res.TaskIntent), "records_created": false},
		})
		return
	}

	// ⑤ 不在允许集合内 → 手选兜底
	if _, ok := AllowedIntents[res.TaskIntent]; !ok {
		options := []gin.H{}
		for k, v := range AllowedIntents {
			options = append(options, gin.H{"task_intent": k, "label": v})
		}
		respondCard(c, &agentCard{
			Type: CardManualFallback,
			Say:  "这件事我还没有把握，选一个更接近的任务吧。",
			Data: gin.H{"options": options},
		})
		return
	}

	// ⑥ 置信度不足 → 只问一轮
	if res.Confidence < 0.6 && strings.TrimSpace(res.ClarifyQuestion) != "" {
		respondCard(c, &agentCard{
			Type: CardClarify,
			Say:  res.ClarifyQuestion,
			Why:  "我只问这一轮，问完就开始。",
			Data: gin.H{"task_intent": res.TaskIntent},
		})
		return
	}

	// ⑦ 四筛没过 → 说明为什么不做成 Skill
	if !res.Sieve.allPassed() {
		respondCard(c, &agentCard{
			Type: CardNotSkillable,
			Say:  "这类问题我们不做成方法：" + res.Sieve.ReasonIfFalse,
			Why:  "真需求不等于适合被系统化。做不成的东西硬做出来只会误导你。",
			Data: gin.H{"sieve": res.Sieve},
		})
		return
	}

	// ⑧ 任务卡 + 顺手把能用的能力也带上（一次往返拿全，少一次等待）
	var routed gin.H
	if list := quickRoute(c.GetInt64("userID"), utterance, res.TaskIntent); list != nil {
		routed = list
	}
	respondCard(c, &agentCard{
		Type: CardTask,
		Say:  res.NextStep,
		Why:  "这是你今天就能动手的第一步。",
		Data: gin.H{
			"task_intent":      res.TaskIntent,
			"task_label":       AllowedIntents[res.TaskIntent],
			"current_position": res.CurrentPosition,
			"gap":              res.Gap,
			"next_step":        res.NextStep,
			"utterance":        utterance,
			"routed":           routed,
		},
		Actions: []gin.H{
			{"label": "在这儿把它做完", "method": "POST",
				"path": "/api/growth/executions", "primary": true},
		},
	})
}

// quickRoute 顺带取一次可用能力。失败就返回 nil，不影响任务卡。
func quickRoute(uid int64, utterance, intent string) gin.H {
	rows, err := db.Query(`SELECT s.id, s.name, COALESCE(v.version,'')
		FROM skills s LEFT JOIN skill_versions v ON v.id = s.current_version_id
		LEFT JOIN skill_scores sc ON sc.skill_id = s.id
		WHERE COALESCE(s.status,'') = ? AND COALESCE(sc.is_candidate_eligible,0) = 1
		  AND COALESCE(s.task_intent,'') = ?
		ORDER BY COALESCE(sc.quality_score,0) DESC LIMIT 3`,
		SkillStatusPublished, intent)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, version string
		if rows.Scan(&id, &name, &version) == nil {
			out = append(out, gin.H{"skill_id": id, "name": name, "version": version})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return gin.H{"count": len(out), "skills": out, "note": "排序理由与「为什么没选另一个」在详情里"}
}

// itoa 拼路径用。单独包一层是因为这个文件里出现得很密。
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
