// F18 能力简历：SKU 组成的不是库存，是一个人。
//
// 这一块补的是供给端唯一真实的回报缺口——在它之前，贡献者拿到的只是影响力数字，
// 对他本人没有直接效用。而「我的经历变成一份招聘方能点开验证的简历」是有效用的。
//
// 三条硬约束（简历可量化之后必须堵的漏）：
//  1. 每一项都能下钻到原始证据——刷出来的数字扛不住下钻。
//  2. 失败强制在场，且**没有关闭开关**。只有成功的能力简历没有可信度。
//  3. 不给总分。只有结构化声明加证据。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 简历里允许用户勾选是否公开的模块。
// 注意：failures 不在这里——失败项恒定包含，这是刻意的。
var resumeToggleable = []string{"quantified", "skills", "decisions", "paths", "timeline"}

// initResumeSchema 快照表。由 initGrowthSchema 调用。
func initResumeSchema() {
	schema := `
CREATE TABLE IF NOT EXISTS profile_snapshots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  share_token TEXT NOT NULL UNIQUE,
  content TEXT NOT NULL,
  checksum TEXT NOT NULL,
  included TEXT DEFAULT '[]',
  expires_at DATETIME,
  revoked_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_snap_user ON profile_snapshots(user_id);
CREATE INDEX IF NOT EXISTS idx_snap_token ON profile_snapshots(share_token);
`
	if _, err := db.Exec(schema); err != nil {
		log.Printf("init resume schema failed: %v", err)
	}
}

// ---------- 四类可量化口径 ----------

// quantified 允许出现在简历上的量化项。全部是行为，没有一项是自评。
// 这里刻意只有四类——不是所有东西都该量化。
func quantified(uid int64) gin.H {
	// ① 做过多少次：按任务方向分组的已完成执行
	byIntent := []gin.H{}
	totalDone := 0
	rows, err := db.Query(`SELECT task_intent, COUNT(*) FROM executions
		WHERE user_id = ? AND status = ? GROUP BY task_intent ORDER BY COUNT(*) DESC`,
		uid, ExecCompleted)
	if err == nil {
		for rows.Next() {
			var intent string
			var n int
			if rows.Scan(&intent, &n) != nil {
				continue
			}
			if label, ok := AllowedIntents[intent]; ok {
				byIntent = append(byIntent, gin.H{
					"task_intent": intent, "task_label": label, "count": n,
					"drilldown": "/api/growth/my-profile", // 可下钻到成长路线
				})
				totalDone += n
			}
		}
		rows.Close()
	}

	// ② 产出多少条被验证的判断
	var verifiedDecisions, totalVerifications int
	db.QueryRow(`SELECT COUNT(*) FROM decisions
		WHERE origin_user_id = ? AND verified_by_count > 0 AND invalidated_at IS NULL`,
		uid).Scan(&verifiedDecisions)
	db.QueryRow(`SELECT COALESCE(SUM(verified_by_count), 0) FROM decisions
		WHERE origin_user_id = ?`, uid).Scan(&totalVerifications)

	// ③ 我的能力被多少不同的人成功用过（去重用户，且产物被导出）
	var helpedPeople int
	db.QueryRow(`SELECT COUNT(DISTINCT e.user_id) FROM executions e
		JOIN skill_versions v ON v.id = e.skill_version_id
		JOIN skills s ON s.id = v.skill_id
		WHERE COALESCE(s.maintainer_id, s.owner_id) = ? AND e.user_id != ?
		  AND e.status = ? AND e.completion_signal LIKE '%"exported":true%'`,
		uid, uid, ExecCompleted).Scan(&helpedPeople)

	// ④ 走完了哪几条长周期路径
	paths := []gin.H{}
	r2, err := db.Query(`SELECT o.orchestration_intent, o.goal_label, o.status,
		(SELECT COUNT(*) FROM orchestration_reviews r WHERE r.orchestration_id = o.id)
		FROM orchestrations o WHERE o.user_id = ? AND o.status IN (?, ?)`,
		uid, OrchCompleted, OrchActive)
	if err == nil {
		for r2.Next() {
			var intent, goal, status string
			var reviews int
			if r2.Scan(&intent, &goal, &status, &reviews) != nil {
				continue
			}
			// 只有真的跟了 3 周以上、或者已完成的才算「走过」
			if status != OrchCompleted && reviews < 3 {
				continue
			}
			paths = append(paths, gin.H{
				"orchestration_intent": intent,
				"label":                OrchestrationIntents[intent],
				"goal_label":           goal,
				"status":               status,
				"weeks_reviewed":       reviews,
			})
		}
		r2.Close()
	}

	return gin.H{
		"tasks_done_total":      totalDone,
		"tasks_by_intent":       byIntent,
		"verified_decisions":    verifiedDecisions,
		"total_verifications":   totalVerifications,
		"helped_people":         helpedPeople,
		"paths_walked":          paths,
		"note":                  "这四类是唯一允许量化的东西，全部来自行为记录，没有一项是自评。",
		"no_composite_score":    true,
	}
}

// resumeSkills 我维护的已发布 Skill，带可下钻入口
func resumeSkills(uid int64) []gin.H {
	rows, err := db.Query(`SELECT s.id, s.name, COALESCE(s.task_intent,''), COALESCE(v.version,''),
		COALESCE(v.proof_type,'platform_trace')
		FROM skills s LEFT JOIN skill_versions v ON v.id = s.current_version_id
		WHERE COALESCE(s.maintainer_id, s.owner_id) = ? AND COALESCE(s.status,'') = ?
		ORDER BY s.id DESC`, uid, SkillStatusPublished)
	if err != nil {
		return []gin.H{}
	}
	type row struct {
		id      int64
		name    string
		intent  string
		version string
		proof   string
	}
	list := []row{}
	for rows.Next() {
		var r row
		if rows.Scan(&r.id, &r.name, &r.intent, &r.version, &r.proof) == nil {
			list = append(list, r)
		}
	}
	rows.Close()

	// SQLite 单连接：先收完再逐条统计，避免自锁
	out := []gin.H{}
	for _, r := range list {
		var callers, effective, decCount int
		db.QueryRow(`SELECT COUNT(DISTINCT e.user_id) FROM executions e
			JOIN skill_versions v ON v.id = e.skill_version_id
			WHERE v.skill_id = ? AND e.user_id != ?`, r.id, uid).Scan(&callers)
		db.QueryRow(`SELECT COUNT(*) FROM executions e
			JOIN skill_versions v ON v.id = e.skill_version_id
			WHERE v.skill_id = ? AND e.user_id != ? AND e.status = ?
			  AND e.completion_signal LIKE '%"exported":true%'`,
			r.id, uid, ExecCompleted).Scan(&effective)
		db.QueryRow(validDecisionCountBySkillSQL, r.id).Scan(&decCount)

		out = append(out, gin.H{
			"skill_id":            r.id,
			"name":                r.name,
			"task_label":          AllowedIntents[r.intent],
			"version":             r.version,
			"callers":             callers,
			"effective_uses":      effective,
			"traceable_decisions": decCount,
			"backfilled":          r.proof == ProofArtifactUpload,
			// 下钻入口：招聘方点这里进 Trust Card
			"drilldown": fmt.Sprintf("/trust/%d", r.id),
		})
	}
	return out
}

// resumeDecisions 我贡献的判断。这是「我到底会什么」最硬的一层证据。
func resumeDecisions(uid int64) []gin.H {
	rows, err := db.Query(`SELECT id, slot, trigger_signal, judgment, scope,
		verified_by_count, COALESCE(counter_example,''), invalidated_at
		FROM decisions WHERE origin_user_id = ?
		ORDER BY verified_by_count DESC, id DESC LIMIT 20`, uid)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var slot, trigger, judgment, scope, counter string
		var verified int
		var inval interface{}
		if rows.Scan(&id, &slot, &trigger, &judgment, &scope, &verified, &counter, &inval) != nil {
			continue
		}
		out = append(out, gin.H{
			"decision_id":       id,
			"slot_prompt":       slotPrompt(slot),
			"statement":         fmt.Sprintf("当%s时，%s", trigger, judgment),
			"scope":             scope,
			"verified_by_count": verified,
			"has_counter":       strings.TrimSpace(counter) != "",
			"invalidated":       inval != nil,
			"drilldown":         fmt.Sprintf("/api/growth/decisions/%d/trace", id),
		})
	}
	return out
}

// resumeFailures 停下来的地方。**没有开关，恒定包含。**
// 一份只有成功的能力简历没有可信度，带失败的反而可信。
func resumeFailures(uid int64) gin.H {
	var abandoned, insights, invalidated int
	db.QueryRow(`SELECT COUNT(*) FROM executions WHERE user_id = ? AND status = ?`,
		uid, ExecAbandoned).Scan(&abandoned)
	db.QueryRow(`SELECT COUNT(*) FROM insights WHERE user_id = ?`, uid).Scan(&insights)
	db.QueryRow(`SELECT COUNT(*) FROM decisions
		WHERE origin_user_id = ? AND invalidated_at IS NOT NULL`, uid).Scan(&invalidated)

	items := []gin.H{}
	rows, err := db.Query(`SELECT task_intent, COALESCE(task_title,''), COALESCE(abandoned_at_step,0)
		FROM executions WHERE user_id = ? AND status = ? ORDER BY id DESC LIMIT 5`,
		uid, ExecAbandoned)
	if err == nil {
		for rows.Next() {
			var intent, title string
			var step int
			if rows.Scan(&intent, &title, &step) == nil {
				items = append(items, gin.H{
					"task_label": AllowedIntents[intent], "task_title": title, "stopped_at_step": step,
				})
			}
		}
		rows.Close()
	}

	return gin.H{
		"abandoned_executions": abandoned,
		"insight_notes":        insights,
		"invalidated_decisions": invalidated,
		"recent":               items,
		"always_included":      true,
		"why":                  "这一段没有关闭开关。只有成功的能力简历没有可信度——知道什么时候该停，本身就是判断力的一部分。",
	}
}

// ---------- 接口 ----------

// getMyResume GET /api/growth/resume
func getMyResume(c *gin.Context) {
	uid := c.GetInt64("userID")
	c.JSON(http.StatusOK, buildResume(uid, resumeToggleable))
}

// buildResume 组装简历内容。included 决定放哪些可选模块；失败项永远在。
func buildResume(uid int64, included []string) gin.H {
	has := map[string]bool{}
	for _, k := range included {
		has[k] = true
	}
	user, err := getUserByID(uid)
	out := gin.H{
		"generated_at":       time.Now().UTC(),
		"no_composite_score": true,
		"principle":          "这份简历上的每一个数字都能点开验到那次真实执行。刷出来的数字扛不住下钻。",
	}
	if err == nil {
		out["who"] = gin.H{
			"username": user.Username, "school": user.School,
			"major": user.Major, "grade": user.Grade,
		}
	}
	if has["quantified"] {
		out["quantified"] = quantified(uid)
	}
	if has["skills"] {
		out["skills"] = resumeSkills(uid)
	}
	if has["decisions"] {
		out["decisions"] = resumeDecisions(uid)
	}
	if has["timeline"] {
		out["timeline"] = loadTimeline(uid)
	}
	// 失败项恒定包含，不看 included
	out["failures"] = resumeFailures(uid)
	return out
}

// createSnapshot POST /api/growth/resume/snapshots
// 对外分享的不是主页本体，而是一份冻结的快照。
func createSnapshot(c *gin.Context) {
	uid := c.GetInt64("userID")
	var body struct {
		Included  []string `json:"included"`
		ExpiresIn int      `json:"expires_in_days"` // 0 = 永久
	}
	c.ShouldBindJSON(&body)

	// 只接受白名单里的模块；failures 不接受（它恒定包含，不需要勾）
	included := []string{}
	for _, k := range body.Included {
		for _, allowed := range resumeToggleable {
			if k == allowed {
				included = append(included, k)
			}
		}
	}
	if len(included) == 0 {
		included = resumeToggleable
	}

	content := buildResume(uid, included)
	raw := jsonOrEmpty(content)
	sum := sha256.Sum256([]byte(raw))
	checksum := hex.EncodeToString(sum[:])[:16]
	token := hex.EncodeToString(sum[:])[16:32] + fmt.Sprintf("%d", time.Now().Unix()%100000)

	var expiresAt interface{}
	if body.ExpiresIn > 0 {
		expiresAt = time.Now().AddDate(0, 0, body.ExpiresIn).UTC().Format("2006-01-02 15:04:05")
	}

	if _, err := db.Exec(`INSERT INTO profile_snapshots (user_id, share_token, content,
		checksum, included, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		uid, token, raw, checksum, jsonOrEmpty(included), expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"share_token": token,
		"checksum":    checksum,
		"included":    included,
		"share_path":  "/r/" + token,
		"note":        "这份快照已冻结。之后你的数据变化不会影响它——这是为了让看的人相信它不是事后编的。",
		"failures_included": true,
	})
}

// listSnapshots GET /api/growth/resume/snapshots
func listSnapshots(c *gin.Context) {
	uid := c.GetInt64("userID")
	rows, err := db.Query(`SELECT id, share_token, checksum, included, expires_at, revoked_at, created_at
		FROM profile_snapshots WHERE user_id = ? ORDER BY id DESC`, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var token, checksum, included string
		var expires, revoked interface{}
		var created time.Time
		if rows.Scan(&id, &token, &checksum, &included, &expires, &revoked, &created) == nil {
			out = append(out, gin.H{
				"id": id, "share_token": token, "checksum": checksum,
				"included": rawOrDefault(included, "[]"),
				"expires_at": expires, "revoked": revoked != nil, "created_at": created,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// revokeSnapshot POST /api/growth/resume/snapshots/:token/revoke
func revokeSnapshot(c *gin.Context) {
	uid := c.GetInt64("userID")
	token := c.Param("token")
	res, err := db.Exec(`UPDATE profile_snapshots SET revoked_at = CURRENT_TIMESTAMP
		WHERE share_token = ? AND user_id = ?`, token, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "snapshot not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已吊销，链接立即失效"})
}

// getSharedSnapshot GET /api/growth/shared/:token
// 无需登录。已吊销或过期一律不泄露任何数据。
func getSharedSnapshot(c *gin.Context) {
	token := c.Param("token")
	var content, checksum string
	var createdAt time.Time
	var expires, revoked interface{}
	if err := db.QueryRow(`SELECT content, checksum, created_at, expires_at, revoked_at
		FROM profile_snapshots WHERE share_token = ?`, token).
		Scan(&content, &checksum, &createdAt, &expires, &revoked); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "链接无效"})
		return
	}
	if revoked != nil {
		c.JSON(http.StatusGone, gin.H{"error": "这份分享已被吊销"})
		return
	}
	if expires != nil {
		if s, ok := expires.(string); ok && s != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil && time.Now().After(t) {
				c.JSON(http.StatusGone, gin.H{"error": "这份分享已过期"})
				return
			}
		}
	}
	// 校验内容没被改过——佐证「这不是事后编的」
	sum := sha256.Sum256([]byte(content))
	intact := hex.EncodeToString(sum[:])[:16] == checksum

	c.JSON(http.StatusOK, gin.H{
		"content":       json.RawMessage(content),
		"checksum":      checksum,
		"intact":        intact,
		"generated_at":  createdAt,
		"verify_note":   "这是一份生成后就冻结的快照。上面每一项都能点开下钻到 Trust Card 与判断级溯源。",
	})
}
