// F1 目标识别与四筛判定（PRD 第 6 章 F1、第 1.5 节伪需求拒绝策略）
// 核心约束：五类伪需求一律不进任务流、不落 Experience。
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// sieveResult 四把筛子
type sieveResult struct {
	Amortizable   bool   `json:"amortizable"`    // 可摊销：是否在大量人身上重复出现
	Testable      bool   `json:"testable"`       // 可测试：是否有可判断的完成标准
	Transferable  bool   `json:"transferable"`   // 可转移：关键判断能否脱离具体某个人
	ShortLoop     bool   `json:"short_loop"`     // 短链路：执行到看见结果是否够快
	ReasonIfFalse string `json:"reason_if_false"`
}

func (s sieveResult) allPassed() bool {
	return s.Amortizable && s.Testable && s.Transferable && s.ShortLoop
}

// intentResult P1 Prompt 的输出结构
type intentResult struct {
	TaskIntent      string      `json:"task_intent"`
	Confidence      float64     `json:"confidence"`
	Sieve           sieveResult `json:"sieve"`
	CurrentPosition string      `json:"current_position"`
	Gap             []string    `json:"gap"`
	NextStep        string      `json:"next_step"`
	ClarifyQuestion string      `json:"clarify_question"`
	RouteExit       string      `json:"route_exit"`
	Reply           string      `json:"reply"`
	Heard           string      `json:"heard"`
	Junction        string      `json:"junction_id"`
	Stage           string      `json:"stage_id"`
	OrchIntent      string      `json:"orchestration_intent"`
}

const intentSystemPrompt = `你是大学生迷茫期路由器。不测评、不建议、不承诺。
你的工作不是回答「该怎么办」，而是听懂用户这句话，送到四个出口之一，并用他的原话处境说话。

四个出口 route_exit（必须恰好一个）：
- emotion：只用于明确的心理危机：撑不住、崩溃、活不下去、想不开、自伤、自杀、重度焦虑到没法过活。除此之外一律不是 emotion。
- decide：还在「该不该 / A还是B」，没决定。包括学业（转专业、保研还是就业）和感情（该不该谈、要不要表白、该不该分手）。不给选边。
- action：已经决定方向，问接下来怎么做。不承诺结果。
- explore：其余所有处境。孤独、没朋友、想家、有好感但没问该不该、失恋但还能说话、宿舍处不好、论文焦虑、想试一件事，都是 explore。

判断顺序（必须按此，不要跳）：
1. 只有心理危机 → emotion。
2. 「该不该」或二选一且还没决定 → decide。
3. 已经决定 → action。
4. 其余 → explore。

禁止标成 emotion（这些全是 explore 或 decide）：
有好感、喜欢一个人、想谈恋爱、要不要表白、该不该分手、失恋、孤独、没朋友、想家、室友矛盾、宿舍处不好、焦虑论文、压力大、好烦、emo、想逃课。
有「该不该」时，即使夹了难过、焦虑、喜欢，也是 decide，不是 emotion。

junction_id：decide 时必填；explore 若是交朋友/孤独/没朋友也填 j-friend。只能是：
j-y0（高考/报志愿/本省外省/复读）、j-major（转专业/专业合不合适）、j-y3（保研/考研/就业/出国）、j-y4（毕业/offer/gap）、j-friend（交朋友/孤独/没朋友/社恐）、j-love（谈恋爱/表白/好感/在一起）
交朋友和谈恋爱是两件不同的事。孤独、没朋友、想交朋友、社恐开口 → j-friend，绝不要标成 j-love。只有明确在问该不该谈/表白/分手/对某人有好感要不要处，才是 j-love。
stage_id 只能是 y0 y1 y2 y3 y4 g1
orchestration_intent 只在 action 时填，只能是：postgrad_recommend / postgrad_exam / study_abroad / job_season / research_entry / competition_season

task_intent 仍要填（给旧任务链路）：
允许：thesis_topic, resume_rewrite, resume_jd_align, report_structure, mock_interview, interview_review, project_convergence, literature_review, content_script
拒绝：emotional_support, life_decision, zero_sum_competition, realtime_fact, resource_dependent
对不上允许类时：emotion→emotional_support；decide/action→life_decision；explore→最接近的允许类，实在没有就 project_convergence。

heard：用不超过 20 字复述用户卡在哪，不要评价。
reply：2–5 句，必须点到用户原话里的具体处境（不要套「识别为任务：论文选题」这种标签）。禁止给建议、打分、承诺结果。

四把筛子（仅 explore 且 task_intent 属于允许类时认真填，其余可全 true）：
amortizable / testable / transferable / short_loop

confidence 低于 0.6 时给 clarify_question，只问一个问题。

例子：
「我大一，一个朋友都没有，很孤独」→ explore, stage_id=y1, junction_id=j-friend, heard=大一没有朋友很孤独
「我想交朋友」→ explore, stage_id=y1, junction_id=j-friend, heard=想交朋友
「我对有个人感到好感」→ explore, stage_id=y1, heard=对有人有好感
「该不该转专业」→ decide, junction_id=j-major, heard=纠结要不要转专业
「我现在到底该不该谈恋爱，我对有个人感到好感」→ decide, junction_id=j-love, heard=对有人有好感，在问该不该谈
「要不要表白」→ decide, junction_id=j-love, heard=在问要不要表白
「该不该分手」→ decide, junction_id=j-love, heard=在问该不该分手
「失恋了，有点难受」→ explore, heard=失恋了还难受
「和室友处不好」→ explore, heard=宿舍关系处不好
「论文好焦虑，选题还没定」→ explore, task_intent=thesis_topic, stage_id=y4, heard=论文选题焦虑
「保研还是就业」→ decide, junction_id=j-y3, heard=保研和就业之间还没定
「我决定保研了，接下来怎么准备」→ action, orchestration_intent=postgrad_recommend, heard=已决定保研，要排准备
「最近好累，感觉撑不住了」→ emotion, heard=已经撑不住
「活不下去了」→ emotion, heard=活不下去

严格只输出 JSON，不要 markdown。格式：
{"route_exit":"explore","heard":"","reply":"","junction_id":"","stage_id":"y1","orchestration_intent":"","task_intent":"","confidence":0.0,"sieve":{"amortizable":true,"testable":true,"transferable":true,"short_loop":true,"reason_if_false":""},"current_position":"","gap":[],"next_step":"","clarify_question":""}`

var growthJSONBlockRe = regexp.MustCompile("(?s)\\{.*\\}")

// extractJSON 从模型输出里抠出第一个 JSON 对象（容忍模型偶尔套 markdown 代码块）
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	if m := growthJSONBlockRe.FindString(s); m != "" {
		return m
	}
	return s
}

// interpretGoal POST /api/growth/goals/interpret
// 输入用户原话，返回任务卡；伪需求走拒绝策略且不落 Experience。
func interpretGoal(c *gin.Context) {
	var body struct {
		Utterance string `json:"utterance"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	utterance := strings.TrimSpace(body.Utterance)
	if len([]rune(utterance)) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "再说一句你现在卡在哪"})
		return
	}
	if len([]rune(utterance)) > 500 {
		utterance = string([]rune(utterance)[:500])
	}

	db.Exec(`INSERT INTO description_corpus (utterance, source) VALUES (?, 'goal_input')`, utterance)

	res, err := classifyIntent(utterance)
	if err != nil {
		log.Printf("intent classify failed: %v", err)
		res = heuristicRoute(utterance)
	} else {
		normalizeIntentResult(res, utterance)
	}
	applyRouteCorrection(res, utterance)
	exit := res.RouteExit

	// 只有危机词才拦截。模型把好感/孤独/焦虑标成 emotion 的，这里已经纠正。
	if exit == "emotion" {
		c.JSON(http.StatusOK, attachRoute(gin.H{
			"mode":        "rejected",
			"task_intent": IntentEmotionalSupport,
			"reason":      RejectedIntents[IntentEmotionalSupport],
			"response":    firstNonEmpty(res.Reply, rejectionResponse(IntentEmotionalSupport)),
			"resources":   rejectionResources(IntentEmotionalSupport),
		}, res))
		return
	}

	// 第二顺位：还在「该不该」
	if exit == "decide" || (res.TaskIntent == IntentLifeDecision && looksUndecided(utterance)) {
		c.JSON(http.StatusOK, attachRoute(gin.H{
			"mode":        "rejected",
			"task_intent": IntentLifeDecision,
			"reason":      RejectedIntents[IntentLifeDecision],
			"response":    firstNonEmpty(res.Reply, rejectionResponse(IntentLifeDecision)),
			"resources":   rejectionResources(IntentLifeDecision),
			"branches":    lifeDecisionBranches(),
		}, res))
		return
	}

	// 第三顺位：已经决定，进编排
	if exit == "action" {
		orch := orchIntentOf(res, utterance)
		c.JSON(http.StatusOK, attachRoute(gin.H{
			"mode":                 "orchestration",
			"task_intent":          res.TaskIntent,
			"orchestration_intent": orch,
			"label":                OrchestrationIntents[orch],
			"message":              firstNonEmpty(res.Reply, "这件事的结果我不敢承诺，但接下来几周该做什么可以排清楚——用别人真走过的路来排。"),
			"next":                 "probe",
		}, res))
		return
	}

	if orchIntent, ok := OrchestrationRouteIntents[res.TaskIntent]; ok {
		if res.OrchIntent != "" {
			orchIntent = res.OrchIntent
		}
		c.JSON(http.StatusOK, attachRoute(gin.H{
			"mode":                 "orchestration",
			"task_intent":          res.TaskIntent,
			"orchestration_intent": orchIntent,
			"label":                OrchestrationIntents[orchIntent],
			"message":              firstNonEmpty(res.Reply, "这件事的结果我不敢承诺，但接下来几周该做什么可以排清楚——用别人真走过的路来排。"),
			"next":                 "probe",
		}, res))
		return
	}

	// 其余拒绝类（实时事实等）
	if reason, rejected := RejectedIntents[res.TaskIntent]; rejected && exit != "explore" {
		c.JSON(http.StatusOK, attachRoute(gin.H{
			"mode":        "rejected",
			"task_intent": res.TaskIntent,
			"reason":      reason,
			"response":    firstNonEmpty(res.Reply, rejectionResponse(res.TaskIntent)),
			"resources":   rejectionResources(res.TaskIntent),
		}, res))
		return
	}

	// explore：对得上允许任务且四筛过 → 任务卡；否则仍按探索出口返回，不再丢去「手选任务」
	if _, ok := AllowedIntents[res.TaskIntent]; ok && res.Confidence >= 0.6 &&
		(res.Sieve.allPassed() || res.Sieve.ReasonIfFalse == "") && strings.TrimSpace(res.ClarifyQuestion) == "" {
		c.JSON(http.StatusOK, attachRoute(gin.H{
			"mode": "task",
			"task_card": gin.H{
				"task_intent":      res.TaskIntent,
				"task_label":       AllowedIntents[res.TaskIntent],
				"current_position": res.CurrentPosition,
				"gap":              res.Gap,
				"next_step":        res.NextStep,
				"sieve":            res.Sieve,
			},
		}, res))
		return
	}

	if res.Confidence < 0.6 && strings.TrimSpace(res.ClarifyQuestion) != "" && res.Reply == "" {
		c.JSON(http.StatusOK, attachRoute(gin.H{
			"mode":             "clarify",
			"task_intent":      res.TaskIntent,
			"clarify_question": res.ClarifyQuestion,
		}, res))
		return
	}

	c.JSON(http.StatusOK, attachRoute(gin.H{
		"mode":        "explore",
		"task_intent": res.TaskIntent,
		"message":     firstNonEmpty(res.Reply, res.ClarifyQuestion, "先从一件能试的小事开始。"),
	}, res))
}

func attachRoute(h gin.H, res *intentResult) gin.H {
	if res == nil {
		return h
	}
	if res.RouteExit != "" {
		h["route_exit"] = res.RouteExit
	}
	if res.Reply != "" {
		h["reply"] = res.Reply
	}
	if res.Heard != "" {
		h["heard"] = res.Heard
	}
	if res.Junction != "" {
		h["junction_id"] = res.Junction
	}
	if res.Stage != "" {
		h["stage_id"] = res.Stage
	}
	if res.OrchIntent != "" {
		h["orchestration_intent"] = res.OrchIntent
	}
	return h
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func exitWasEmotionReply(s string) bool {
	return strings.Contains(s, "心理支持") || strings.Contains(s, "挺不好过") || strings.Contains(s, "帮不上")
}

var wowJunctions = map[string]bool{"j-y0": true, "j-major": true, "j-y3": true, "j-y4": true, "j-friend": true, "j-love": true}
var wowStages = map[string]bool{"y0": true, "y1": true, "y2": true, "y3": true, "y4": true, "g1": true}

func normalizeIntentResult(res *intentResult, utterance string) {
	res.RouteExit = normalizeRouteExit(res.RouteExit)
	if !wowJunctions[res.Junction] {
		res.Junction = ""
	}
	if !wowStages[res.Stage] {
		res.Stage = ""
	}
	if _, ok := OrchestrationIntents[res.OrchIntent]; !ok {
		res.OrchIntent = ""
	}
	if strings.TrimSpace(res.Heard) == "" {
		res.Heard = clipRunes(utterance, 20)
	}
}

func applyRouteCorrection(res *intentResult, utterance string) {
	if res == nil {
		return
	}
	prev := res.RouteExit
	exit := correctRouteExit(utterance, res.RouteExit)
	res.RouteExit = exit
	switch exit {
	case "emotion":
		res.TaskIntent = IntentEmotionalSupport
	case "decide":
		res.TaskIntent = IntentLifeDecision
		if res.Junction == "" {
			res.Junction = guessJunctionID(utterance)
		}
	case "action":
		if res.OrchIntent == "" {
			res.OrchIntent = guessOrchestrationIntent(utterance)
		}
	case "explore":
		if res.Stage == "" {
			res.Stage = guessStageID(utterance)
		}
		if looksFriendship(utterance) {
			res.Junction = "j-friend"
		}
	}
	if looksFriendship(utterance) && !looksRomanceExplicit(utterance) {
		res.Junction = "j-friend"
	}
	if looksRomanceExplicit(utterance) && !looksFriendship(utterance) && res.Junction == "j-friend" {
		res.Junction = "j-love"
	}
	if strings.TrimSpace(res.Reply) == "" || (prev == "emotion" && exit != "emotion") || (exit != "emotion" && exitWasEmotionReply(res.Reply)) {
		res.Reply = heuristicReply(exit, utterance, res.Heard)
	}
}

func correctRouteExit(utterance, suggested string) string {
	if looksEmotionalCrisis(utterance) {
		return "emotion"
	}
	if looksUndecided(utterance) {
		return "decide"
	}
	if looksCommitted(utterance) {
		return "action"
	}
	s := normalizeRouteExit(suggested)
	if s == "emotion" || s == "" {
		return "explore"
	}
	return s
}

func normalizeRouteExit(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "explore", "decide", "action", "emotion":
		return strings.ToLower(strings.TrimSpace(s))
	}
	return ""
}

func inferRouteExit(res *intentResult, utterance string) string {
	suggested := ""
	if res != nil {
		suggested = res.RouteExit
	}
	return correctRouteExit(utterance, suggested)
}

func orchIntentOf(res *intentResult, utterance string) string {
	if res != nil && res.OrchIntent != "" {
		if _, ok := OrchestrationIntents[res.OrchIntent]; ok {
			return res.OrchIntent
		}
	}
	if res != nil {
		if mapped, ok := OrchestrationRouteIntents[res.TaskIntent]; ok {
			return mapped
		}
	}
	return guessOrchestrationIntent(utterance)
}

func clipRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

func guessJunctionID(s string) string {
	switch {
	case looksFriendship(s):
		return "j-friend"
	case looksRelationship(s):
		return "j-love"
	case strings.Contains(s, "转专业") || strings.Contains(s, "专业合不合适") || strings.Contains(s, "适不适合这个专业"):
		return "j-major"
	case strings.Contains(s, "高考") || strings.Contains(s, "报志愿") || strings.Contains(s, "本省") || strings.Contains(s, "外省") || strings.Contains(s, "复读") || strings.Contains(s, "择校"):
		return "j-y0"
	case strings.Contains(s, "毕业") || strings.Contains(s, "offer") || strings.Contains(s, "gap") || strings.Contains(s, "延毕"):
		return "j-y4"
	}
	return "j-y3"
}

func looksFriendship(s string) bool {
	if looksRomanceExplicit(s) && !(strings.Contains(s, "交朋友") || strings.Contains(s, "没朋友") || strings.Contains(s, "孤独") || strings.Contains(s, "社恐")) {
		return false
	}
	keys := []string{"交朋友", "没朋友", "一个朋友", "没有朋友", "孤独", "社恐", "搭子", "认识人", "社交从零", "想有朋友"}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func looksRomanceExplicit(s string) bool {
	keys := []string{"恋爱", "表白", "暗恋", "分手", "在一起", "对象", "该不该谈"}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func looksRelationship(s string) bool {
	if looksFriendship(s) && !looksRomanceExplicit(s) {
		return false
	}
	keys := []string{"恋爱", "表白", "分手", "暗恋", "在一起", "对象"}
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	if strings.Contains(s, "好感") && !looksFriendship(s) {
		return true
	}
	return strings.Contains(s, "喜欢") && (strings.Contains(s, "他") || strings.Contains(s, "她") || strings.Contains(s, "一个人") || strings.Contains(s, "同学") || strings.Contains(s, "同桌")) && !looksFriendship(s)
}

func guessStageID(s string) string {
	switch {
	case strings.Contains(s, "高考") || strings.Contains(s, "报志愿") || strings.Contains(s, "选专业") || strings.Contains(s, "择校"):
		return "y0"
	case strings.Contains(s, "大一") || strings.Contains(s, "孤独") || strings.Contains(s, "社团"):
		return "y1"
	case strings.Contains(s, "大二") || strings.Contains(s, "转专业") || strings.Contains(s, "竞赛"):
		return "y2"
	case strings.Contains(s, "大三") || strings.Contains(s, "保研") || strings.Contains(s, "考研") || strings.Contains(s, "秋招"):
		return "y3"
	case strings.Contains(s, "大四") || strings.Contains(s, "毕业") || strings.Contains(s, "毕设"):
		return "y4"
	case strings.Contains(s, "研") || strings.Contains(s, "导师") || strings.Contains(s, "论文"):
		return "g1"
	}
	return "y1"
}

func heuristicRoute(utterance string) *intentResult {
	res := &intentResult{
		Heard:      clipRunes(utterance, 20),
		Confidence: 0.58,
		Sieve:      sieveResult{Amortizable: true, Testable: true, Transferable: true, ShortLoop: true},
	}
	s := utterance
	switch {
	case looksEmotionalCrisis(s):
		res.RouteExit = "emotion"
		res.TaskIntent = IntentEmotionalSupport
	case looksUndecided(s):
		res.RouteExit = "decide"
		res.TaskIntent = IntentLifeDecision
		res.Junction = guessJunctionID(s)
	case looksCommitted(s):
		res.RouteExit = "action"
		res.TaskIntent = IntentLifeDecision
		res.OrchIntent = guessOrchestrationIntent(s)
	default:
		res.RouteExit = "explore"
		res.TaskIntent = IntentProjectConverge
		res.Stage = guessStageID(s)
	}
	res.Reply = heuristicReply(res.RouteExit, s, res.Heard)
	return res
}

func looksEmotionalCrisis(s string) bool {
	markers := []string{"撑不住", "崩溃", "活不下去", "想不开", "不想活", "自杀", "自残", "割腕", "结束生命", "重度焦虑"}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func looksCommitted(s string) bool {
	if looksUndecided(s) {
		return false
	}
	markers := []string{"我决定", "已经决定", "已经想好", "定了", "接下来怎么准备", "怎么排"}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func heuristicReply(exit, utterance, heard string) string {
	if heard == "" {
		heard = clipRunes(utterance, 20)
	}
	switch exit {
	case "emotion":
		return "听到了。你说的是「" + heard + "」——这句话不需要被解决，也不会变成任何数据。如果只是累，歇一会儿再来。如果这种感觉持续了一段时间，校心理支持中心比任何流程都合适。"
	case "decide":
		return "你卡在「" + heard + "」。我不会替你选边——这类选择没有「做对了」的标准。我能给的是走过这个路口的人去了哪、付出了什么。"
	case "action":
		return "听到你已经定了方向（「" + heard + "」）。那就不试了，按别人真走完的路排接下来几周——不承诺结果。"
	default:
		if looksFriendship(utterance) {
			return "听到的是：「" + heard + "」。这是交朋友，不是谈恋爱。我不会把两件事混在一起。"
		}
		return "听到的是：「" + heard + "」。这还不是「该不该」的题，也先不用选边。我们从一件能试的小事开始。"
	}
}

// classifyIntent 调用 LLM 做识别，并对输出做 schema 校验（失败重试一次）
func classifyIntent(utterance string) (*intentResult, error) {
	msgs := []chatMsg{
		{Role: "system", Content: intentSystemPrompt},
		{Role: "user", Content: utterance},
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := callDeepSeekKeyTemp(context.Background(), msgs, "DEEPSEEK_API_KEY", 0.2, true)
		if err != nil {
			lastErr = err
			continue
		}
		var res intentResult
		if err := json.Unmarshal([]byte(extractJSON(raw)), &res); err != nil {
			lastErr = err
			continue
		}
		if strings.TrimSpace(res.TaskIntent) == "" && strings.TrimSpace(res.RouteExit) == "" {
			lastErr = errEmptyIntent
			continue
		}
		return &res, nil
	}
	return nil, lastErr
}

// errEmptyIntent 模型返回了空 intent
var errEmptyIntent = &intentError{"模型未给出 task_intent"}

type intentError struct{ msg string }

func (e *intentError) Error() string { return e.msg }

// rejectionResponse 五类伪需求各自的回应口径。
// 注意：情绪类不做评判、不展开、不追问，只承接并给资源。
func rejectionResponse(intent string) string {
	switch intent {
	case IntentEmotionalSupport:
		return "听起来这段时间挺不好过的。这件事我帮不上——它需要的是能坐下来听你说话的人，不是一套流程。如果你愿意，学校的心理支持是免费的，也不会留在任何记录里。"
	case IntentLifeDecision:
		return "这个我不给建议。不是回避，是这类选择没有「做对了」的标准，别人的答案套到你身上大概率是错的。我可以给你看几个真实走过不同路的人，以及他们各自付出的代价，你自己判断。"
	case IntentZeroSum:
		return "这件事的结果由分配规则和其他人的表现决定，任何声称能提高成功率的方法都在骗你。我能做的是把规则讲清楚，以及帮你做好其中你能控制的那部分。"
	case IntentRealtimeFact:
		return "这是一条需要查最新信息的事实，不是一套可复用的方法。我给你查，但不会把它做成 Skill——那样明年就是错的。"
	case IntentResourceDep:
		return "这件事的关键变量不在方法上，而在你拿不到的资源上。我可以帮你做其中能转移的那部分，比如怎么写第一封联系邮件；剩下的部分我不会假装能解决。"
	}
	return ""
}

// looksUndecided 判断是不是「该不该」型措辞。
// 这是三态里最细的一条线：还在犹豫 → 不给建议；已经决定 → 给编排。
func looksUndecided(s string) bool {
	if looksHowTo(s) && !(strings.Contains(s, "该不该") || strings.Contains(s, "要不要")) {
		return false
	}
	markers := []string{"该不该", "要不要", "值不值", "值得吗", "好还是", "还是考", "选哪个", "怎么选", "纠结", "不知道该", "不知道要"}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	if strings.Contains(s, "还是") {
		sides := []string{"保研", "考研", "就业", "出国", "转", "考公", "工作", "实习", "本省", "外省", "复读", "留学", "恋爱", "表白", "分手"}
		for _, x := range sides {
			if strings.Contains(s, x) {
				return true
			}
		}
	}
	if strings.Contains(s, "是否") && !strings.Contains(s, "是否已") {
		return true
	}
	return false
}

// guessOrchestrationIntent 从原话粗判编排方向。判不出来时给保研（当前唯一有 Path 的方向）。
func guessOrchestrationIntent(s string) string {
	switch {
	case strings.Contains(s, "保研") || strings.Contains(s, "推免") || strings.Contains(s, "夏令营"):
		return "postgrad_recommend"
	case strings.Contains(s, "考研"):
		return "postgrad_exam"
	case strings.Contains(s, "出国") || strings.Contains(s, "留学") || strings.Contains(s, "申请"):
		return "study_abroad"
	case strings.Contains(s, "秋招") || strings.Contains(s, "春招") || strings.Contains(s, "找工作") || strings.Contains(s, "求职"):
		return "job_season"
	case strings.Contains(s, "进组") || strings.Contains(s, "科研") || strings.Contains(s, "导师"):
		return "research_entry"
	case strings.Contains(s, "竞赛") || strings.Contains(s, "比赛"):
		return "competition_season"
	}
	return "postgrad_recommend"
}

// lifeDecisionBranches 「该不该」型问题的回应：只给别人走过的分支与代价，不给建议。
func lifeDecisionBranches() []gin.H {
	rows, err := db.Query(`SELECT goal_label, walked_count, COALESCE(branch_summary,'{}'), provenance
		FROM paths ORDER BY walked_count DESC LIMIT 5`)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var goal, branch, prov string
		var walked int
		if rows.Scan(&goal, &walked, &branch, &prov) == nil {
			out = append(out, gin.H{
				"goal_label":      goal,
				"walked_count":    walked,
				"branch_summary":  rawOrDefault(branch, "{}"),
				"provenance":      prov,
				"provenance_note": provenanceNote(prov),
			})
		}
	}
	return out
}

// rejectionResources 拒绝时给出的替代动作
func rejectionResources(intent string) []gin.H {
	switch intent {
	case IntentEmotionalSupport:
		return []gin.H{
			{"label": "学校心理健康中心", "hint": "多数高校提供免费预约咨询"},
			{"label": "先不聊这个，看看我的下一步", "action": "goto_home"},
		}
	case IntentLifeDecision:
		return []gin.H{
			{"label": "看看走过不同路的人", "action": "goto_graph"},
		}
	case IntentZeroSum:
		return []gin.H{
			{"label": "只做我能控制的那部分", "action": "goto_home"},
		}
	case IntentResourceDep:
		return []gin.H{
			{"label": "先写好联系邮件", "action": "goto_home"},
		}
	}
	return []gin.H{}
}
