// 调用的代价：用完之后留下一条「这次它在你的情况下成立吗」。
//
// 为什么需要这个：SKU 要流转，交换必须对等。原来的设计里调用者获得能力、
// 创作者只获得几个影响力数字，这个交换太不对等，所以流转缺动力。
//
// 实现方式不是弹窗强制（PRD 明令提示不许阻塞流程），而是改口径：
// **没提交 verdict 的调用不计入 adoption_rate**。
// 理由站得住——一次什么都没反馈的调用，我们凭什么说它「被采纳了」。
// 这让对价变成激励结构，同时让 adoption 这个指标本身更诚实。
package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// verdict 取值
const (
	VerdictHeld          = "held"           // 在我的情况下成立
	VerdictFailed        = "failed"         // 不成立
	VerdictNotApplicable = "not_applicable" // 不适用于我的情况
)

func validVerdict(v string) bool {
	switch v {
	case VerdictHeld, VerdictFailed, VerdictNotApplicable:
		return true
	}
	return false
}

// NewBoundaryMinIndependent 一条新边界要被几次独立执行验证才触发版本候选。
// 对应 PRD F12 的 new_boundary_verified 规则——之前一直没有落地，
// 因为没有任何地方产生「独立验证」这个事件。verdict 补上了这个来源。
const NewBoundaryMinIndependent = 2

// getPendingVerdicts GET /api/growth/executions/:id/pending-verdicts
// 一次执行用了哪个 Skill、该对哪几条判断表态。
func getPendingVerdicts(c *gin.Context) {
	uid := c.GetInt64("userID")
	execID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var owner int64
	var versionID sql.NullInt64
	if err := db.QueryRow(`SELECT user_id, skill_version_id FROM executions WHERE id = ?`, execID).
		Scan(&owner, &versionID); err != nil || owner != uid {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
		return
	}
	if !versionID.Valid || versionID.Int64 == 0 {
		// 裸任务执行没有用任何 Skill，没有要表态的对象
		c.JSON(http.StatusOK, gin.H{"required": false, "decisions": []gin.H{}})
		return
	}

	decisions := loadDecisions(versionID.Int64)
	submitted := map[int64]string{}
	rows, err := db.Query(`SELECT decision_id, verdict FROM decision_verdicts WHERE execution_id = ?`, execID)
	if err == nil {
		for rows.Next() {
			var did int64
			var v string
			if rows.Scan(&did, &v) == nil {
				submitted[did] = v
			}
		}
		rows.Close()
	}

	out := []gin.H{}
	pending := 0
	for _, d := range decisions {
		if d.InvalidatedAt != nil {
			continue
		}
		item := gin.H{
			"decision_id":    d.ID,
			"slot":           d.Slot,
			"slot_prompt":    slotPrompt(d.Slot),
			"trigger_signal": d.TriggerSignal,
			"judgment":       d.Judgment,
			"scope":          d.Scope,
		}
		if v, ok := submitted[d.ID]; ok {
			item["my_verdict"] = v
		} else {
			pending++
		}
		out = append(out, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"required":       len(out) > 0,
		"pending_count":  pending,
		"decisions":      out,
		"why":            "这一步是调用的代价：你留下一句它在你的情况下成不成立，这个方法才会变好。",
		"adoption_note":  "没有表态的调用不会计入这个 Skill 的采纳率——一次什么都没反馈的调用，我们不会说它被采纳了。",
	})
}

// submitVerdicts POST /api/growth/executions/:id/verdicts
func submitVerdicts(c *gin.Context) {
	uid := c.GetInt64("userID")
	execID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Verdicts []struct {
			DecisionID int64  `json:"decision_id"`
			Verdict    string `json:"verdict"`
			Note       string `json:"note"`
		} `json:"verdicts"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Verdicts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少要对一条判断表态"})
		return
	}

	var owner int64
	var versionID sql.NullInt64
	if err := db.QueryRow(`SELECT user_id, skill_version_id FROM executions WHERE id = ?`, execID).
		Scan(&owner, &versionID); err != nil || owner != uid {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution not found"})
		return
	}
	if !versionID.Valid || versionID.Int64 == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "这次执行没有使用任何 Skill"})
		return
	}
	var skillID int64
	db.QueryRow(`SELECT skill_id FROM skill_versions WHERE id = ?`, versionID.Int64).Scan(&skillID)

	// 只接受本版本真实引用的判断，防止对任意 decision_id 刷验证数
	allowed := map[int64]bool{}
	for _, d := range loadDecisions(versionID.Int64) {
		allowed[d.ID] = true
	}

	accepted, rejected := 0, 0
	newBoundaries := []int64{}
	for _, v := range body.Verdicts {
		if !validVerdict(v.Verdict) || !allowed[v.DecisionID] {
			rejected++
			continue
		}
		// 同一执行对同一判断只算一次（表里有 UNIQUE 约束兜底）
		res, err := db.Exec(`INSERT OR IGNORE INTO decision_verdicts
			(decision_id, user_id, execution_id, verdict, note) VALUES (?, ?, ?, ?, ?)`,
			v.DecisionID, uid, execID, v.Verdict, strings.TrimSpace(v.Note))
		if err != nil {
			rejected++
			continue
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue // 已经表态过
		}
		accepted++

		switch v.Verdict {
		case VerdictHeld:
			// verified_by_count 终于有了真实来源。在这之前它一直是 0。
			db.Exec(`UPDATE decisions SET verified_by_count = verified_by_count + 1 WHERE id = ?`,
				v.DecisionID)
		case VerdictFailed:
			// 不成立 → 补进反例。已有反例就追加，不覆盖别人写的。
			if strings.TrimSpace(v.Note) != "" {
				db.Exec(`UPDATE decisions SET counter_example =
					CASE WHEN COALESCE(counter_example,'') = '' THEN ?
					     ELSE counter_example || ' / ' || ? END
					WHERE id = ?`, v.Note, v.Note, v.DecisionID)
			}
			// 被足够多次独立执行判为不成立 → 触发 new_boundary_verified
			if independentFailures(v.DecisionID) >= NewBoundaryMinIndependent {
				newBoundaries = append(newBoundaries, v.DecisionID)
			}
		}
	}

	// 触发版本候选：这是「一条新边界被验证后进入下一版」的落地
	created := false
	if len(newBoundaries) > 0 && skillID > 0 {
		created = createBoundaryCandidate(skillID, newBoundaries)
	}
	if skillID > 0 {
		recomputeSkillScore(skillID)
	}

	resp := gin.H{
		"accepted": accepted,
		"rejected": rejected,
		"message":  "记下了。这个方法因为你这一句变得更准了一点。",
	}
	if created {
		resp["version_candidate_created"] = true
		resp["trigger_rule"] = "new_boundary_verified"
		resp["note"] = "同一条判断被两次独立执行判为不成立，已生成版本候选并通知维护者。"
	}
	c.JSON(http.StatusOK, resp)
}

// independentFailures 有多少个不同用户、在不同执行里把这条判断判为不成立
func independentFailures(decisionID int64) int {
	var n int
	db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM decision_verdicts
		WHERE decision_id = ? AND verdict = ?`, decisionID, VerdictFailed).Scan(&n)
	return n
}

// createBoundaryCandidate 生成 new_boundary_verified 类型的版本候选
func createBoundaryCandidate(skillID int64, decisionIDs []int64) bool {
	var existing int
	db.QueryRow(`SELECT COUNT(*) FROM version_candidates
		WHERE skill_id = ? AND status = 'open' AND trigger_rule = 'new_boundary_verified'`,
		skillID).Scan(&existing)
	if existing > 0 {
		return false
	}
	evidence := gin.H{
		"decision_ids": decisionIDs,
		"reason":       "这些判断被多次独立执行判为不成立，说明适用边界比原来写的窄",
	}
	if _, err := db.Exec(`INSERT INTO version_candidates (skill_id, trigger_rule, evidence, status)
		VALUES (?, 'new_boundary_verified', ?, 'open')`, skillID, jsonOrEmpty(evidence)); err != nil {
		log.Printf("create boundary candidate failed: %v", err)
		return false
	}
	db.Exec(`UPDATE skills SET status = ? WHERE id = ? AND COALESCE(status,'') = ?`,
		SkillStatusNeedsReview, skillID, SkillStatusPublished)
	return true
}

// hasVerdict 这次执行有没有表态过。adoption 口径依赖它。
func hasVerdict(execID int64) bool {
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM decision_verdicts WHERE execution_id = ?`, execID).Scan(&n)
	return n > 0
}
