// F7 准入四层 + F8 两段式路由排序
// 先辨别哪些 Skill 值得进入候选集，再把适合当前任务、用户和环境的排到前面。
// 硬约束：接口响应不得包含 rank_score 或任何综合分数字段，只给解释。
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ---------- 第一层：准入检查 ----------

type admissionResult struct {
	Passed   bool
	Failures []string
}

// 判断挂在版本上，按 Skill 计数要经关联表绕到当前版本。
// 抽成常量避免三处各写一遍写歪。
const decisionCountBySkillSQL = `SELECT COUNT(*) FROM skill_version_decisions l
	JOIN skills s ON s.current_version_id = l.skill_version_id
	WHERE s.id = ?`

const validDecisionCountBySkillSQL = `SELECT COUNT(*) FROM skill_version_decisions l
	JOIN decisions d ON d.id = l.decision_id
	JOIN skills s ON s.current_version_id = l.skill_version_id
	WHERE s.id = ? AND d.invalidated_at IS NULL`

// runAdmissionCheck 结构、依赖、权限、数据边界、适用范围的静态审查。
// 降权信号：缺少必要文件、依赖失效、权限过大、边界模糊。
func runAdmissionCheck(skillID int64, ver *SkillVersion) admissionResult {
	res := admissionResult{Passed: true, Failures: []string{}}

	// 结构：六个 slot 里 SKILL.md 与 evals 是硬要求
	var hasSkillMD, hasEvals int
	db.QueryRow(`SELECT COUNT(*) FROM skill_files WHERE skill_id = ? AND LOWER(file_path) LIKE '%skill.md'`, skillID).
		Scan(&hasSkillMD)
	db.QueryRow(`SELECT COUNT(*) FROM skill_files WHERE skill_id = ? AND file_path LIKE '%/evals/%'`, skillID).
		Scan(&hasEvals)
	if hasSkillMD != 1 {
		res.Failures = append(res.Failures, fmt.Sprintf("缺少必要文件：SKILL.md 必须且只能有一个（当前 %d 个）", hasSkillMD))
	}
	if hasEvals < 1 {
		res.Failures = append(res.Failures, "缺少必要文件：evals 至少要有一个测试文件")
	}

	if ver != nil {
		// 边界模糊
		var boundary struct {
			NotApplicable  []string `json:"not_applicable"`
			HandoffTrigger []string `json:"handoff_trigger"`
		}
		json.Unmarshal([]byte(ver.Boundary), &boundary)
		if len(boundary.NotApplicable) == 0 || len(boundary.HandoffTrigger) == 0 {
			res.Failures = append(res.Failures, "边界模糊：不适用条件与人工接管触发点都必须写清")
		}

		// 适用范围
		if strings.TrimSpace(ver.Goal) == "" {
			res.Failures = append(res.Failures, "适用范围不明：没写清它到底帮谁完成什么")
		}
		var criteria []string
		json.Unmarshal([]byte(ver.DoneCriteria), &criteria)
		if len(criteria) == 0 {
			res.Failures = append(res.Failures, "缺少可判断的完成标准")
		} else {
			// SKU 要「可比较」。全是自定义标准的 Skill 无法与同类对齐，
			// 不阻止发布，但要提示——这是它进不了比较视图的原因。
			var intent string
			db.QueryRow(`SELECT COALESCE(task_intent,'') FROM skills WHERE id = ?`, skillID).Scan(&intent)
			if len(CriteriaVocab[intent]) > 0 && comparableCriteriaCount(intent, ver.DoneCriteria) == 0 {
				res.Failures = append(res.Failures,
					"完成标准全是自定义的，无法与同类 Skill 比较：至少从公共标准里选一条")
			}
		}

		// 权限过大：声明了不可逆操作却没有对应的人工接管
		var contract struct {
			Permissions []string `json:"permissions"`
		}
		json.Unmarshal([]byte(ver.Contract), &contract)
		for _, p := range contract.Permissions {
			if isIrreversiblePermission(p) && len(boundary.HandoffTrigger) == 0 {
				res.Failures = append(res.Failures,
					"权限过大："+p+" 属于不可逆操作，但没有任何人工接管触发点覆盖它")
			}
		}

		// 依赖失效：声明的前置 Skill 是否还在
		var deps struct {
			PrerequisiteSkillIDs []int64 `json:"prerequisite_skill_ids"`
		}
		json.Unmarshal([]byte(ver.Contract), &deps)
		for _, pid := range deps.PrerequisiteSkillIDs {
			var st string
			if err := db.QueryRow(`SELECT COALESCE(status,'') FROM skills WHERE id = ?`, pid).Scan(&st); err != nil {
				res.Failures = append(res.Failures, fmt.Sprintf("依赖失效：前置 Skill %d 不存在", pid))
			} else if st == SkillStatusArchived || st == SkillStatusDeprecated {
				res.Failures = append(res.Failures, fmt.Sprintf("依赖失效：前置 Skill %d 已下线", pid))
			}
		}
	}

	res.Passed = len(res.Failures) == 0
	return res
}

func isIrreversiblePermission(p string) bool {
	p = strings.ToLower(p)
	for _, k := range []string{"send", "write_external", "delete", "publish", "pay"} {
		if strings.Contains(p, k) {
			return true
		}
	}
	return false
}

// ---------- 四层评分 ----------

// recomputeSkillScore 计算并物化准入四层评分。
// 冷启动规则：线上证据不足时用离线分打折作先验，并标注样本不足——不允许因为没数据就给高分。
func recomputeSkillScore(skillID int64) *SkillScore {
	var versionID sql.NullInt64
	db.QueryRow(`SELECT current_version_id FROM skills WHERE id = ?`, skillID).Scan(&versionID)
	var ver *SkillVersion
	if versionID.Valid {
		ver, _ = loadSkillVersion(versionID.Int64)
	}

	adm := runAdmissionCheck(skillID, ver)

	// 第二层：离线评测
	offline := 0.0
	if versionID.Valid {
		weights := map[string]float64{
			EvalDiscoverability: 0.30,
			EvalCompletion:      0.30,
			EvalStability:       0.25,
			EvalBoundaryStop:    0.15,
		}
		for t, w := range weights {
			var rate float64
			if err := db.QueryRow(`SELECT pass_rate FROM eval_runs WHERE version_id = ? AND eval_type = ?
				ORDER BY id DESC LIMIT 1`, versionID.Int64, t).Scan(&rate); err == nil {
				offline += w * clamp01(rate)
			}
		}
	}

	// 第三层：在线证据（全部为行为信号，不含成果信号）
	var callCount int
	var adoption, abandon, correction, reuse float64
	db.QueryRow(`SELECT COUNT(*) FROM executions WHERE skill_version_id IN
		(SELECT id FROM skill_versions WHERE skill_id = ?)`, skillID).Scan(&callCount)

	sampleSufficient := callCount >= OnlineEvidenceMinCall
	if callCount > 0 {
		var completed, abandoned, reused int
		var avgCorrection sql.NullFloat64
		db.QueryRow(`SELECT
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END),
			SUM(CASE WHEN completion_signal LIKE '%"reused_within_7d":true%' THEN 1 ELSE 0 END),
			AVG(correction_ratio)
			FROM executions WHERE skill_version_id IN (SELECT id FROM skill_versions WHERE skill_id = ?)`,
			ExecCompleted, ExecAbandoned, skillID).
			Scan(&completed, &abandoned, &reused, &avgCorrection)
		_ = completed

		// adoption 口径（改动二）：不只看有没有导出，还要看用户有没有留下 verdict。
		// 一次什么都没反馈的调用不算「被采纳」——这既是调用的对价，
		// 也让这个指标本身更诚实（原来只看 exported 太松）。
		var adopted int
		db.QueryRow(`SELECT COUNT(DISTINCT e.id) FROM executions e
			JOIN decision_verdicts dv ON dv.execution_id = e.id
			WHERE e.skill_version_id IN (SELECT id FROM skill_versions WHERE skill_id = ?)
			  AND e.completion_signal LIKE '%"exported":true%'`, skillID).Scan(&adopted)

		adoption = float64(adopted) / float64(callCount)
		abandon = float64(abandoned) / float64(callCount)
		reuse = float64(reused) / float64(callCount)
		if avgCorrection.Valid {
			correction = avgCorrection.Float64
		}
	}

	online := 0.0
	if sampleSufficient {
		online = 0.35*clamp01(adoption) + 0.25*(1-clamp01(correction)) +
			0.25*(1-clamp01(abandon)) + 0.15*clamp01(reuse)
	} else {
		online = clamp01(offline * 0.8) // 冷启动先验
	}

	// 验证覆盖度（改动四）：被多少种不同背景验证过。
	// 一个被同一类用户刷 50 次的 Skill，库存质量不如被 5 种不同背景各验证一次的，
	// 但原来的公式会给前者更高分——因为它只数次数不看分布。
	coverage := 0.0
	if callCount > 0 {
		var buckets int
		db.QueryRow(`SELECT COUNT(DISTINCT COALESCE(u.grade,'') || '|' || COALESCE(u.major,''))
			FROM executions e JOIN users u ON u.id = e.user_id
			WHERE e.skill_version_id IN (SELECT id FROM skill_versions WHERE skill_id = ?)
			  AND e.status = ?`, skillID, ExecCompleted).Scan(&buckets)
		// 5 个不同背景桶视为覆盖充分
		coverage = clamp01(float64(buckets) / 5.0)
	}

	// 第四层：维护状态
	var daysSinceUpdate float64 = 999
	var days sql.NullFloat64
	db.QueryRow(`SELECT julianday('now') - julianday(COALESCE(updated_at, created_at)) FROM skills WHERE id = ?`, skillID).
		Scan(&days)
	if days.Valid {
		daysSinceUpdate = days.Float64
	}
	versionActivity := 1.0
	if daysSinceUpdate > 90 {
		versionActivity = math.Max(0, 1-(daysSinceUpdate-90)/270)
	}
	dependencyHealth := 1.0
	if !adm.Passed {
		for _, f := range adm.Failures {
			if strings.Contains(f, "依赖失效") {
				dependencyHealth = 0.5
			}
		}
	}
	var openCand, expiredCand int
	db.QueryRow(`SELECT COUNT(*) FROM version_candidates WHERE skill_id = ? AND status = 'open'`, skillID).Scan(&openCand)
	db.QueryRow(`SELECT COUNT(*) FROM version_candidates WHERE skill_id = ? AND status = 'expired'`, skillID).Scan(&expiredCand)
	issueResponse := 1.0
	if openCand+expiredCand > 0 {
		issueResponse = float64(openCand) / float64(openCand+expiredCand)
		if expiredCand > 0 && openCand == 0 {
			issueResponse = 0
		}
	}
	maintenance := 0.40*versionActivity + 0.35*dependencyHealth + 0.25*issueResponse

	// 从 online 的权重里挪 0.10 给覆盖度：同样的调用量，
	// 被更多种背景验证过的 Skill 应该排在前面。
	quality := 0.40*clamp01(offline) + 0.25*clamp01(online) +
		0.10*clamp01(coverage) + 0.25*clamp01(maintenance)

	var status string
	db.QueryRow(`SELECT COALESCE(status,'') FROM skills WHERE id = ?`, skillID).Scan(&status)
	eligible := adm.Passed && status == SkillStatusPublished && quality >= 0.50

	sc := &SkillScore{
		SkillID: skillID, AdmissionPassed: adm.Passed,
		AdmissionFailures: jsonOrEmpty(adm.Failures),
		OfflineScore:      clamp01(offline), OnlineScore: clamp01(online),
		MaintenanceScore: clamp01(maintenance), QualityScore: clamp01(quality),
		SampleSufficient: sampleSufficient, CandidateEligible: eligible,
	}
	sc.CoverageScore = clamp01(coverage)
	db.Exec(`INSERT INTO skill_scores (skill_id, admission_passed, admission_failures, offline_score,
		online_score, maintenance_score, coverage_score, quality_score, sample_sufficient,
		is_candidate_eligible, computed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(skill_id) DO UPDATE SET admission_passed=excluded.admission_passed,
		admission_failures=excluded.admission_failures, offline_score=excluded.offline_score,
		online_score=excluded.online_score, maintenance_score=excluded.maintenance_score,
		coverage_score=excluded.coverage_score,
		quality_score=excluded.quality_score, sample_sufficient=excluded.sample_sufficient,
		is_candidate_eligible=excluded.is_candidate_eligible, computed_at=CURRENT_TIMESTAMP`,
		skillID, boolToInt(adm.Passed), sc.AdmissionFailures, sc.OfflineScore, sc.OnlineScore,
		sc.MaintenanceScore, sc.CoverageScore, sc.QualityScore,
		boolToInt(sampleSufficient), boolToInt(eligible))
	db.Exec(`UPDATE skills SET quality_score = ? WHERE id = ?`, sc.QualityScore, skillID)
	return sc
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// recomputeAllScores POST /api/growth/admin/recompute-scores
// 相当于每日巡检的手动触发版
func recomputeAllScores(c *gin.Context) {
	rows, err := db.Query(`SELECT id FROM skills`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		recomputeSkillScore(id)
	}
	c.JSON(http.StatusOK, gin.H{"recomputed": len(ids)})
}

// ---------- F8 两段式路由 ----------

type routeCandidate struct {
	SkillID     int64
	Name        string
	Description string
	Version     string
	VersionID   int64
	TaskIntent  string
	Quality     float64
	SampleOK    bool
	CallCount   int
	TaskFit     float64
	UserFit     float64
	RiskPenalty float64
	Rank        float64
	Permissions []string
	Irreversible bool
}

// routeSkills POST /api/growth/route
// 第一段用 is_candidate_eligible 硬过滤，第二段排序，并且必须给出解释。
func routeSkills(c *gin.Context) {
	uid := c.GetInt64("userID")
	var body struct {
		Utterance  string `json:"utterance"`
		TaskIntent string `json:"task_intent"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	utterance := strings.TrimSpace(body.Utterance)

	// 用户画像（阶段适配用）
	stage := ""
	if u, err := getUserByID(uid); err == nil {
		stage = u.Grade
	}

	rows, err := db.Query(`SELECT s.id, s.name, COALESCE(v.description, s.description), COALESCE(v.version,''),
		COALESCE(v.id, 0), COALESCE(s.task_intent,''), COALESCE(sc.quality_score, 0),
		COALESCE(sc.sample_sufficient, 0), COALESCE(sc.is_candidate_eligible, 0), COALESCE(v.contract,'{}')
		FROM skills s
		LEFT JOIN skill_versions v ON v.id = s.current_version_id
		LEFT JOIN skill_scores sc ON sc.skill_id = s.id
		WHERE COALESCE(s.status,'') = ?`, SkillStatusPublished)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	qTerms := keyTerms(utterance)
	all := []routeCandidate{}
	filteredOut := 0
	filterReasons := map[string]int{}

	for rows.Next() {
		var rc routeCandidate
		var sampleOK, eligible int
		var contract string
		if err := rows.Scan(&rc.SkillID, &rc.Name, &rc.Description, &rc.Version, &rc.VersionID,
			&rc.TaskIntent, &rc.Quality, &sampleOK, &eligible, &contract); err != nil {
			continue
		}
		rc.SampleOK = sampleOK == 1

		// 第一段：硬过滤。不合格的一律不进候选集
		if eligible != 1 {
			filteredOut++
			filterReasons["未通过准入或质量分不足"]++
			continue
		}

		// task_fit：第一跳只看 name + description（渐进披露的硬约束）
		sim := overlapScore(qTerms, keyTerms(rc.Name+" "+rc.Description))
		exact := 0.0
		if body.TaskIntent != "" && rc.TaskIntent == body.TaskIntent {
			exact = 1
		}
		rc.TaskFit = 0.6*sim + 0.4*exact

		// user_fit：阶段与前置
		rc.UserFit = userFit(stage, rc.VersionID)

		// risk_penalty
		var ct struct {
			Permissions []string `json:"permissions"`
		}
		json.Unmarshal([]byte(contract), &ct)
		rc.Permissions = ct.Permissions
		breadth := clamp01(float64(len(ct.Permissions)) / 5.0)
		irr := 0.0
		for _, p := range ct.Permissions {
			if isIrreversiblePermission(p) {
				irr = 1
				rc.Irreversible = true
			}
		}
		rc.RiskPenalty = 0.5*breadth + 0.3*irr

		db.QueryRow(`SELECT COUNT(*) FROM executions WHERE skill_version_id = ?`, rc.VersionID).Scan(&rc.CallCount)

		rc.Rank = 0.30*rc.Quality + 0.35*rc.TaskFit + 0.25*rc.UserFit - 0.10*rc.RiskPenalty
		all = append(all, rc)
	}

	// 完全无关的直接不返回，避免勉强匹配
	kept := []routeCandidate{}
	for _, rc := range all {
		if rc.TaskFit <= 0 {
			filteredOut++
			filterReasons["与当前任务无关"]++
			continue
		}
		kept = append(kept, rc)
	}

	// 排序（简单选择排序，候选量小）
	for i := 0; i < len(kept); i++ {
		for j := i + 1; j < len(kept); j++ {
			if kept[j].Rank > kept[i].Rank {
				kept[i], kept[j] = kept[j], kept[i]
			}
		}
	}

	if len(kept) == 0 {
		// 空候选集不是失败，是需求信号：原话入语料库，驱动运营补货
		if utterance != "" {
			db.Exec(`INSERT INTO description_corpus (utterance, source, task_intent) VALUES (?, 'empty_candidate', ?)`,
				utterance, body.TaskIntent)
		}
		c.JSON(http.StatusOK, gin.H{
			"results":            []gin.H{},
			"filtered_out_count": filteredOut,
			"empty_reason":       "这个任务目前还没有可信的能力单元。你可以直接在工作台裸做一次——做完之后，它就是第一个。",
			"fallback":           gin.H{"action": "create_execution", "task_intent": body.TaskIntent},
		})
		return
	}

	// 取前 5，并生成解释
	top := kept
	if len(top) > 5 {
		top = top[:5]
	}
	var runnerUp *routeCandidate
	// 「为什么没选另一个」：优先挑热度高但排名靠后的那个
	if len(kept) > 1 {
		cand := pickHighAttentionLoser(kept)
		runnerUp = cand
	}

	results := []gin.H{}
	for i, rc := range top {
		item := gin.H{
			"skill_id": rc.SkillID,
			"name":     rc.Name,
			"version":  rc.Version,
			"why_this": explainWhy(rc),
			// choose_if：砍掉综合分之后，用户需要一个自己能判断的条件来选（v1.2 第 6 条）
			"choose_if": chooseIf(rc),
			"evidence":  evidenceFor(rc),
			"risk":      gin.H{"permissions": rc.Permissions, "irreversible": rc.Irreversible},
		}
		if i == 0 && runnerUp != nil && runnerUp.SkillID != rc.SkillID {
			item["why_not_alternative"] = explainWhyNot(*runnerUp)
		}
		results = append(results, item)
	}

	reasons := []gin.H{}
	for k, v := range filterReasons {
		reasons = append(reasons, gin.H{"reason": k, "count": v})
	}
	c.JSON(http.StatusOK, gin.H{
		"results":            results,
		"filtered_out_count": filteredOut,
		"filter_reasons":     reasons,
		"empty_reason":       nil,
	})
}

// userFit 阶段与前置匹配
func userFit(stage string, versionID int64) float64 {
	if versionID == 0 {
		return 0.5
	}
	ver, err := loadSkillVersion(versionID)
	if err != nil {
		return 0.5
	}
	score := 0.0
	// stage_match：Skill 的适用阶段目前写在 goal 文本里，未知阶段给中位分。
	// 画像是执行副产品，第一次必然粗糙——我们不假装它精准。
	if stage == "" {
		score += 0.4 * 0.5
	} else if strings.Contains(ver.Goal, stage) {
		score += 0.4
	} else {
		score += 0.4 * 0.6
	}
	// constraint_match：暂以完成标准是否明确近似
	var criteria []string
	json.Unmarshal([]byte(ver.DoneCriteria), &criteria)
	if len(criteria) > 0 {
		score += 0.3
	}
	// prerequisite_satisfied：无前置视为满足
	var deps struct {
		PrerequisiteSkillIDs []int64 `json:"prerequisite_skill_ids"`
	}
	json.Unmarshal([]byte(ver.Contract), &deps)
	if len(deps.PrerequisiteSkillIDs) == 0 {
		score += 0.3
	}
	return clamp01(score)
}

// explainWhy 排序解释：只说证据类型，不出现任何数字评分
func explainWhy(rc routeCandidate) string {
	parts := []string{}
	if rc.Quality >= 0.7 {
		parts = append(parts, "四类测试都过了")
	} else if rc.Quality >= 0.5 {
		parts = append(parts, "关键测试已通过")
	}
	var hasSource int
	db.QueryRow(decisionCountBySkillSQL, rc.SkillID).Scan(&hasSource)
	if hasSource > 0 {
		parts = append(parts, fmt.Sprintf("%d 条判断都能溯源到真实执行", hasSource))
	}
	var gotchaCount int
	db.QueryRow(`SELECT COUNT(*) FROM skill_files WHERE skill_id = ? AND file_path LIKE '%/gotchas/%'`, rc.SkillID).
		Scan(&gotchaCount)
	if gotchaCount > 0 {
		parts = append(parts, "写清了会踩的坑")
	}
	if !rc.SampleOK {
		parts = append(parts, "线上样本还不足，暂按测试结果判断")
	}
	if len(parts) == 0 {
		return "与你现在要做的这件事最接近"
	}
	return strings.Join(parts, "，")
}

// chooseIf 一句「选它取决于什么」。
//
// 砍掉综合分是对的，但产生了一个真实的可用性问题：五个候选各有一堆维度，
// 用户只能依赖「系统替我排的序」，而对系统排序的信任本来就不高。
// 所以给一个用户自己能判断的条件——不是平台内部指标（质量分高不高、样本多不多），
// 而是他看一眼自己的处境就能回答的问题（材料齐不齐、有没有经历、时间够不够）。
func chooseIf(rc routeCandidate) string {
	// 依据该 Skill 的流程起点判断它假设了什么前提
	var versionID int64
	db.QueryRow(`SELECT COALESCE(current_version_id,0) FROM skills WHERE id = ?`, rc.SkillID).Scan(&versionID)
	if versionID == 0 {
		return ""
	}
	ver, err := loadSkillVersion(versionID)
	if err != nil {
		return ""
	}
	var steps []struct {
		Index int    `json:"index"`
		Title string `json:"title"`
	}
	json.Unmarshal([]byte(ver.Workflow), &steps)
	if len(steps) == 0 {
		return ""
	}
	first := steps[0].Title

	// 第一步是"盘点/收集"类 → 适合还没准备好的人；否则适合材料已就绪的人
	inventoryWords := []string{"盘", "收集", "整理", "梳理", "清点", "找出", "列出"}
	isInventory := false
	for _, w := range inventoryWords {
		if strings.Contains(first, w) {
			isInventory = true
			break
		}
	}
	if isInventory {
		return "如果你连手上有哪些材料都还没盘清，选这个——它第一步就是帮你盘点。"
	}
	return "如果你手上材料已经比较全，选这个——它直接从取舍开始，不重复做盘点。"
}

// explainWhyNot 为什么没选另一个——平台辨别力的可见形式
func explainWhyNot(rc routeCandidate) string {
	reasons := []string{}
	var evalCount int
	db.QueryRow(`SELECT COUNT(*) FROM eval_runs WHERE skill_id = ?`, rc.SkillID).Scan(&evalCount)
	if evalCount == 0 {
		reasons = append(reasons, "没有任务测试")
	}
	var decCount int
	db.QueryRow(decisionCountBySkillSQL, rc.SkillID).Scan(&decCount)
	if decCount == 0 {
		reasons = append(reasons, "没有可溯源的判断")
	}
	var dl int
	db.QueryRow(`SELECT COALESCE(download_count,0) FROM skills WHERE id = ?`, rc.SkillID).Scan(&dl)
	prefix := rc.Name
	if dl > 0 {
		prefix = fmt.Sprintf("%s 的下载量更高", rc.Name)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "证据没有前一个充分")
	}
	return fmt.Sprintf("%s，但%s。热度反映注意力，任务证据才说明能力。", prefix, strings.Join(reasons, "、"))
}

// pickHighAttentionLoser 找出热度高但没排到第一的那个，用于对比解释
func pickHighAttentionLoser(list []routeCandidate) *routeCandidate {
	if len(list) < 2 {
		return nil
	}
	best := -1
	bestDL := -1
	for i := 1; i < len(list); i++ {
		var dl int
		db.QueryRow(`SELECT COALESCE(download_count,0) + COALESCE(view_count,0) FROM skills WHERE id = ?`,
			list[i].SkillID).Scan(&dl)
		if dl > bestDL {
			bestDL = dl
			best = i
		}
	}
	if best < 0 {
		return nil
	}
	return &list[best]
}

// evidenceFor 证据摘要。样本不足时必须标注，且不显示两位小数。
func evidenceFor(rc routeCandidate) gin.H {
	out := gin.H{"sample_size": rc.CallCount, "sample_sufficient": rc.SampleOK}
	pass := gin.H{}
	for _, t := range []string{EvalCompletion, EvalStability, EvalDiscoverability, EvalBoundaryStop} {
		var rate float64
		if err := db.QueryRow(`SELECT pass_rate FROM eval_runs WHERE skill_id = ? AND eval_type = ?
			ORDER BY id DESC LIMIT 1`, rc.SkillID, t).Scan(&rate); err == nil {
			pass[t] = rate
		}
	}
	out["eval_pass"] = pass
	var decCount int
	db.QueryRow(validDecisionCountBySkillSQL, rc.SkillID).Scan(&decCount)
	out["traceable_decisions"] = decCount

	// 判断被别的 Skill 也引用了多少次——这是「流转」的直接证据，
	// 也是它和一个静态文件最本质的区别。
	var reused int
	db.QueryRow(`SELECT COUNT(*) FROM skill_version_decisions l2
		WHERE l2.decision_id IN (
			SELECT l.decision_id FROM skill_version_decisions l
			JOIN skills s ON s.current_version_id = l.skill_version_id WHERE s.id = ?)
		AND l2.skill_version_id NOT IN (
			SELECT current_version_id FROM skills WHERE id = ? AND current_version_id IS NOT NULL)`,
		rc.SkillID, rc.SkillID).Scan(&reused)
	out["decisions_reused_elsewhere"] = reused

	// 被独立验证过多少次
	var verified int
	db.QueryRow(`SELECT COALESCE(SUM(d.verified_by_count), 0) FROM skill_version_decisions l
		JOIN decisions d ON d.id = l.decision_id
		JOIN skills s ON s.current_version_id = l.skill_version_id WHERE s.id = ?`,
		rc.SkillID).Scan(&verified)
	out["decision_verifications"] = verified
	return out
}
