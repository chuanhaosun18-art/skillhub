// WowSkillLand 多轮对话：服务端会话记忆 + 检索经验/入口/Skill。
// 不测评、不建议、不承诺、不选边。未到动手不推销 Skill。
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type wowChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type wowForkBranch struct {
	Name   string   `json:"name"`
	Count  int      `json:"count"`
	Quotes []string `json:"quotes"`
}

type wowForkPack struct {
	Title      string          `json:"title"`
	Total      int             `json:"total"`
	Source     string          `json:"source"`
	SwitchNote string          `json:"switch_note"`
	Branches   []wowForkBranch `json:"branches"`
}

type wowMemory struct {
	Situation    string   `json:"situation"`
	Facts        []string `json:"facts"`
	Constraints  []string `json:"constraints"`
	JunctionID   string   `json:"junction_id"`
	Task         string   `json:"task"`
	OpenQuestion string   `json:"open_question"`
}

type wowDiscussOut struct {
	Reply     string    `json:"reply"`
	Next      string    `json:"next"`
	RouteExit string    `json:"route_exit"`
	Topic     string    `json:"topic"`
	Memory    wowMemory `json:"memory"`
}

func initWowChatSchema() {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS wow_sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  memory TEXT DEFAULT '{}',
  route_exit TEXT DEFAULT '',
  next TEXT DEFAULT 'talk',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS wow_session_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id INTEGER NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_wow_sess_user ON wow_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_wow_msg_sess ON wow_session_messages(session_id);
`)
	if err != nil {
		panic("init wow chat schema failed: " + err.Error())
	}
}

func wowDiscuss(c *gin.Context) {
	uid := c.GetInt64("userID")
	var body struct {
		SessionID int64         `json:"session_id"`
		Utterance string        `json:"utterance"`
		RouteExit string        `json:"route_exit"`
		History   []wowChatTurn `json:"history"`
		Forks     *wowForkPack  `json:"forks"`
		Skills    []string      `json:"skills"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	utterance := strings.TrimSpace(body.Utterance)
	if utterance == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "再说一句"})
		return
	}

	sid, mem, _ := wowLoadOrCreateSession(uid, body.SessionID, body.RouteExit, body.History)

	if looksEmotionalCrisis(utterance) {
		wowAppendMsg(sid, "user", utterance)
		reply := heuristicReply("emotion", utterance, clipRunes(utterance, 20))
		wowAppendMsg(sid, "assistant", reply)
		wowSaveSession(sid, mem, "emotion", "stop")
		c.JSON(http.StatusOK, gin.H{
			"session_id": sid,
			"route_exit": "emotion",
			"next":       "stop",
			"reply":      reply,
			"memory":     mem,
		})
		return
	}

	wowAppendMsg(sid, "user", utterance)
	history := wowLoadMessages(sid, 20)

	query := utterance + " " + mem.Situation + " " + strings.Join(mem.Facts, " ")
	retrieved := wowRetrieve(query, mem.JunctionID, true)
	if body.Forks != nil && len(retrieved.Junctions) == 0 {
		retrieved.Junctions = append(retrieved.Junctions, wowJunctionDoc{
			Title: body.Forks.Title, Total: body.Forks.Total, Source: body.Forks.Source,
			SwitchNote: body.Forks.SwitchNote, Branches: body.Forks.Branches,
		})
	}

	sys := wowDiscussSystemPrompt(mem, retrieved, len(history) > 2)
	msgs := []chatMsg{{Role: "system", Content: sys}}
	for i, t := range history {
		if i == len(history)-1 && t.Role == "user" && t.Content == utterance {
			continue
		}
		role := t.Role
		if role != "user" && role != "assistant" {
			continue
		}
		if strings.TrimSpace(t.Content) == "" {
			continue
		}
		msgs = append(msgs, chatMsg{Role: role, Content: t.Content})
	}
	msgs = append(msgs, chatMsg{Role: "user", Content: utterance})

	raw, err := callDeepSeekKeyTemp(context.Background(), msgs, "DEEPSEEK_API_KEY", 0.4, true)
	out := wowDiscussOut{Next: "talk", RouteExit: "explore", Memory: mem}
	if err != nil {
		out = fallbackTaggedReply(utterance, mem, retrieved, len(history) > 2)
		c.JSON(http.StatusOK, wowDiscussResponse(sid, out, retrieved, true))
		wowAppendMsg(sid, "assistant", out.Reply)
		wowSaveSession(sid, out.Memory, out.RouteExit, out.Next)
		return
	}
	parsed, perr := parseWowDiscussJSON(raw)
	if perr != nil || strings.TrimSpace(parsed.Reply) == "" {
		out = fallbackTaggedReply(utterance, mem, retrieved, len(history) > 2)
		if strings.TrimSpace(parsed.Reply) != "" {
			out.Reply = parsed.Reply
		}
	} else {
		out = parsed
	}
	applyLLMTags(&out, retrieved, utterance, mem)
	if looksLikeWowDump(out.Reply) && len(history) > 2 {
		out.Reply = fallbackFollowupReply(utterance, retrievedForks(retrieved))
	}
	out.Memory = mergeWowMemory(mem, out.Memory, utterance, out.Memory.JunctionID, retrieved)
	wowAppendMsg(sid, "assistant", out.Reply)
	wowSaveSession(sid, out.Memory, out.RouteExit, out.Next)
	c.JSON(http.StatusOK, wowDiscussResponse(sid, out, retrieved, false))
}

func wowDiscussResponse(sid int64, out wowDiscussOut, retrieved wowRetrieved, degraded bool) gin.H {
	exps := make([]gin.H, 0, len(retrieved.Experiences))
	for _, e := range retrieved.Experiences {
		exps = append(exps, gin.H{"junction_id": e.JunctionID, "branch": e.Branch, "quote": e.Quote})
	}
	juncs := make([]gin.H, 0, len(retrieved.Junctions))
	for _, j := range retrieved.Junctions {
		juncs = append(juncs, gin.H{"id": j.ID, "title": j.Title, "total": j.Total})
	}
	sks := make([]gin.H, 0, len(retrieved.Skills))
	for _, s := range retrieved.Skills {
		sks = append(sks, gin.H{"id": s.ID, "title": s.Title, "subtitle": s.Subtitle, "boundary": s.Boundary})
	}
	return gin.H{
		"session_id": sid,
		"reply":      out.Reply,
		"route_exit": out.RouteExit,
		"next":       out.Next,
		"memory":     out.Memory,
		"topic":      out.Topic,
		"context": gin.H{
			"junctions":   juncs,
			"experiences": exps,
			"skills":      sks,
		},
		"degraded": degraded,
	}
}

func wowDiscussSystemPrompt(mem wowMemory, retrieved wowRetrieved, followup bool) string {
	b := strings.Builder{}
	b.WriteString("你是 WowSkillLand 的多轮对话员。不测评、不建议、不承诺、不选边。\n")
	b.WriteString("每一轮你同时做两件事：1) 对用户说话 2) 根据完整对话打结构化标签。标签由你判断，不要被上一轮标签锁死，不要用关键词规则代替理解。\n")
	b.WriteString("禁止：成功率、该不该选哪边、你应该、保证、鸡汤、编造检索结果里没有的人数或原话、把完整分叉表重贴一遍。\n")
	b.WriteString("每次只问一个问题。回复 4–8 句，口语，接上记忆里已有的事实。\n")
	b.WriteString("交朋友和谈恋爱是两件不同的事。用户说交朋友/孤独/没朋友/社恐 → topic=friendship、junction_id=j-friend，禁止收成恋爱。用户说该不该谈/表白/好感要不要处 → topic=romance、j-love。用户纠正「我是交朋友不是谈恋爱」必须立刻换标签并承认上一轮判错。\n")
	b.WriteString("引用经验时写清出处。未到 next=match 不要推销 Skill。\n")
	b.WriteString("必须只输出一个 JSON：{\"reply\":\"对用户说的话\",\"topic\":\"friendship|romance|academic|career|other\",\"route_exit\":\"explore|decide|action|emotion\",\"next\":\"talk|match|stop\",\"memory\":{\"situation\":\"一句话处境\",\"facts\":[\"已确认事实\"],\"constraints\":[\"约束\"],\"junction_id\":\"入口id或空\",\"task\":\"要做的事或空\",\"open_question\":\"下一问\"}}\n")
	b.WriteString("打标签口径：\n")
	b.WriteString("- explore：还在看清处境，没问该不该，也还没要动手。next=talk\n")
	b.WriteString("- decide：在问该不该 / 选哪边。不选边。next=talk\n")
	b.WriteString("- action：已经要动手，或问我该怎么做/应该怎么做/怎么开始/怎么交朋友。next=match。reply 只确认听懂要做的事，不要列卡名。\n")
	b.WriteString("- emotion：仅心理危机。next=stop\n")
	if followup {
		b.WriteString("这是追问。只回答这一句新问题，从检索里挑相关的一条人数+一句原话。\n")
	}

	b.WriteString("\n【会话记忆，必须沿用并更新；若用户纠正主题，记忆里的 junction_id 也要改】\n")
	if strings.TrimSpace(mem.Situation) == "" && len(mem.Facts) == 0 {
		b.WriteString("（新会话，从这一句开始记）\n")
	} else {
		b.WriteString("处境：" + mem.Situation + "\n")
		if len(mem.Facts) > 0 {
			b.WriteString("已确认：" + strings.Join(mem.Facts, "；") + "\n")
		}
		if len(mem.Constraints) > 0 {
			b.WriteString("约束：" + strings.Join(mem.Constraints, "；") + "\n")
		}
		if mem.JunctionID != "" {
			b.WriteString("入口：" + mem.JunctionID + "\n")
		}
		if mem.Task != "" {
			b.WriteString("想做的事：" + mem.Task + "\n")
		}
		if mem.OpenQuestion != "" {
			b.WriteString("未问完：" + mem.OpenQuestion + "\n")
		}
	}

	if len(retrieved.Junctions) > 0 {
		b.WriteString("\n【检索到的入口，只能用这里的数字】\n")
		for _, j := range retrieved.Junctions {
			b.WriteString("- " + j.ID + " " + j.Title + "（" + strconv.Itoa(j.Total) + " 人）来源：" + j.Source + "\n")
			for _, br := range j.Branches {
				b.WriteString("  " + br.Name + "：" + strconv.Itoa(br.Count) + " 人\n")
			}
			if j.SwitchNote != "" {
				b.WriteString("  改道：" + j.SwitchNote + "\n")
			}
		}
	}
	if len(retrieved.Experiences) > 0 {
		b.WriteString("\n【检索到的经验原话，只能引用这些】\n")
		for _, e := range retrieved.Experiences {
			b.WriteString("- [" + e.Branch + "] " + e.Quote + "\n")
		}
	}
	if len(retrieved.Skills) > 0 {
		b.WriteString("\n【检索到的 Skill，仅在 next=match 时可以在记忆里记下，reply 里不要点名推销】\n")
		for _, s := range retrieved.Skills {
			b.WriteString("- " + s.ID + " " + s.Title + "：" + s.Match + "。边界：" + s.Boundary + "\n")
		}
	}
	return b.String()
}

func parseWowDiscussJSON(raw string) (wowDiscussOut, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			raw = raw[i : j+1]
		}
	}
	var out wowDiscussOut
	err := json.Unmarshal([]byte(raw), &out)
	return out, err
}

func applyLLMTags(out *wowDiscussOut, retrieved wowRetrieved, utterance string, mem wowMemory) {
	switch out.RouteExit {
	case "explore", "decide", "action", "emotion":
	default:
		out.RouteExit = "explore"
	}
	if out.RouteExit == "emotion" && !looksEmotionalCrisis(utterance) {
		out.RouteExit = "explore"
	}
	switch out.Topic {
	case "friendship", "romance", "academic", "career", "other":
	default:
		out.Topic = "other"
	}
	switch out.Next {
	case "talk", "match", "stop":
	default:
		out.Next = "talk"
	}
	if out.RouteExit == "action" {
		out.Next = "match"
	}
	if out.RouteExit == "emotion" {
		out.Next = "stop"
	}
	jid := out.Memory.JunctionID
	if jid == "" {
		jid = mem.JunctionID
	}
	if jid != "" && !wowJunctions[jid] {
		jid = ""
	}
	if jid == "" && len(retrieved.Junctions) > 0 {
		jid = retrieved.Junctions[0].ID
	}
	out.Memory.JunctionID = jid
}

func fallbackTaggedReply(utterance string, mem wowMemory, retrieved wowRetrieved, followup bool) wowDiscussOut {
	out := wowDiscussOut{Memory: mem, RouteExit: "explore", Next: "talk", Topic: "other"}
	if looksHowTo(utterance) {
		out.RouteExit = "action"
		out.Next = "match"
		out.Reply = "听到你要动手了。不讨论该不该。下一步去匹配一张能帮你把这件事做完的卡——没有就不编。"
	} else if looksUndecided(utterance) {
		out.RouteExit = "decide"
		out.Reply = fallbackDiscussReply("decide", retrievedForks(retrieved), "talk", utterance, followup)
	} else {
		out.Reply = fallbackDiscussReply("explore", retrievedForks(retrieved), "talk", utterance, followup)
	}
	if looksFriendship(utterance) {
		out.Topic = "friendship"
		out.Memory.JunctionID = "j-friend"
	} else if looksRelationship(utterance) {
		out.Topic = "romance"
		out.Memory.JunctionID = "j-love"
	} else if out.Memory.JunctionID == "" && len(retrieved.Junctions) > 0 {
		out.Memory.JunctionID = retrieved.Junctions[0].ID
	}
	return out
}

func looksLikeWowDump(text string) bool {
	return strings.Contains(text, "只把走过的人摊开") && strings.Contains(text, "你现在更卡哪一块")
}

func mergeWowMemory(old, neu wowMemory, utterance, prefer string, retrieved wowRetrieved) wowMemory {
	out := old
	if strings.TrimSpace(neu.Situation) != "" {
		out.Situation = clipRunes(neu.Situation, 80)
	} else if out.Situation == "" {
		out.Situation = clipRunes(utterance, 40)
	}
	out.Facts = uniqClip(append(old.Facts, neu.Facts...), 8, 40)
	out.Constraints = uniqClip(append(old.Constraints, neu.Constraints...), 6, 40)
	if neu.JunctionID != "" {
		out.JunctionID = neu.JunctionID
	} else if out.JunctionID == "" {
		out.JunctionID = prefer
		if out.JunctionID == "" && len(retrieved.Junctions) > 0 {
			out.JunctionID = retrieved.Junctions[0].ID
		}
	}
	if strings.TrimSpace(neu.Task) != "" {
		out.Task = clipRunes(neu.Task, 40)
	}
	if strings.TrimSpace(neu.OpenQuestion) != "" {
		out.OpenQuestion = clipRunes(neu.OpenQuestion, 40)
	}
	return out
}

func uniqClip(in []string, maxN, maxLen int) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		s = clipRunes(s, maxLen)
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) >= maxN {
			break
		}
	}
	return out
}

func retrievedForks(r wowRetrieved) *wowForkPack {
	if len(r.Junctions) == 0 {
		return nil
	}
	j := r.Junctions[0]
	return &wowForkPack{Title: j.Title, Total: j.Total, Source: j.Source, SwitchNote: j.SwitchNote, Branches: j.Branches}
}

func guessWowJunctionID(s string) string {
	if looksFriendship(s) {
		return "j-friend"
	}
	if looksRelationship(s) {
		return "j-love"
	}
	if strings.Contains(s, "转专业") {
		return "j-major"
	}
	if strings.Contains(s, "高考") || strings.Contains(s, "报志愿") || strings.Contains(s, "本省") || strings.Contains(s, "外省") {
		return "j-y0"
	}
	if strings.Contains(s, "毕业") || strings.Contains(s, "offer") || strings.Contains(s, "gap") {
		return "j-y4"
	}
	if strings.Contains(s, "保研") || strings.Contains(s, "考研") || strings.Contains(s, "就业") || strings.Contains(s, "考公") {
		return "j-y3"
	}
	return ""
}

func wowLoadOrCreateSession(uid, sid int64, routeExit string, history []wowChatTurn) (int64, wowMemory, string) {
	var mem wowMemory
	var storedExit string
	if sid > 0 {
		var owner int64
		var raw string
		err := db.QueryRow(`SELECT user_id, memory, route_exit FROM wow_sessions WHERE id = ?`, sid).Scan(&owner, &raw, &storedExit)
		if err == nil && owner == uid {
			_ = json.Unmarshal([]byte(raw), &mem)
			if storedExit == "" {
				storedExit = routeExit
			}
			return sid, mem, storedExit
		}
	}
	res, err := db.Exec(`INSERT INTO wow_sessions (user_id, memory, route_exit) VALUES (?, '{}', ?)`, uid, routeExit)
	if err != nil {
		return 0, mem, routeExit
	}
	id, _ := res.LastInsertId()
	for _, t := range history {
		role := strings.ToLower(strings.TrimSpace(t.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if strings.TrimSpace(t.Content) == "" {
			continue
		}
		wowAppendMsg(id, role, t.Content)
	}
	return id, mem, routeExit
}

func wowSaveSession(sid int64, mem wowMemory, routeExit, next string) {
	raw, _ := json.Marshal(mem)
	db.Exec(`UPDATE wow_sessions SET memory = ?, route_exit = ?, next = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(raw), routeExit, next, sid)
}

func wowAppendMsg(sid int64, role, content string) {
	if sid <= 0 || strings.TrimSpace(content) == "" {
		return
	}
	db.Exec(`INSERT INTO wow_session_messages (session_id, role, content) VALUES (?, ?, ?)`, sid, role, content)
}

func wowLoadMessages(sid int64, limit int) []wowChatTurn {
	if sid <= 0 {
		return nil
	}
	rows, err := db.Query(`SELECT role, content FROM wow_session_messages WHERE session_id = ? ORDER BY id ASC`, sid)
	if err != nil {
		return nil
	}
	var all []wowChatTurn
	for rows.Next() {
		var t wowChatTurn
		if rows.Scan(&t.Role, &t.Content) == nil {
			all = append(all, t)
		}
	}
	rows.Close()
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all
}

func looksReadyForTask(s string) bool {
	if looksUndecided(s) && !looksHowTo(s) {
		return false
	}
	return looksHowTo(s)
}

func looksHowTo(s string) bool {
	markers := []string{"我想做", "帮我做", "帮我写", "帮我准备", "帮我弄", "开始做", "怎么做", "该怎么", "应该怎么", "我该怎么", "怎么开始", "怎么交", "怎么开口", "怎么认识", "具体怎么", "第一步", "怎么准备", "接下来怎么", "有没有卡", "装载", "动手", "试一张", "匹配一张", "教我", "带我做", "我想试试", "我想开始", "开始准备"}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func fallbackDiscussReply(exit string, forks *wowForkPack, next, utterance string, followup bool) string {
	if next == "match" || exit == "action" {
		return "听到你要动手了。不讨论该不该。下一步去匹配一张能帮你把这件事做完的卡——没有就不编。"
	}
	if followup {
		return fallbackFollowupReply(utterance, forks)
	}
	if forks == nil || len(forks.Branches) == 0 {
		if exit == "decide" {
			return "这类选择我不选边。你继续说你卡在哪一块就行。"
		}
		return "我先听。你接着说现在最卡住的那一点。"
	}
	br := forks.Branches[0]
	var b strings.Builder
	b.WriteString("不替你选。和这件事对上的入口是「")
	b.WriteString(forks.Title)
	b.WriteString("」，")
	b.WriteString(strconv.Itoa(forks.Total))
	b.WriteString(" 人走过。")
	if len(br.Quotes) > 0 {
		b.WriteString("有人说过：「")
		b.WriteString(br.Quotes[0])
		b.WriteString("」")
	}
	b.WriteString(" 整张表不念了。你更卡哪一块，或者直接说「我该怎么做」。")
	return b.String()
}

func fallbackFollowupReply(utterance string, forks *wowForkPack) string {
	if forks == nil || len(forks.Branches) == 0 {
		return "刚才的分叉不再重复。你接着说现在最卡住的那一点就行。"
	}
	idx := 0
	best := -1
	for i, br := range forks.Branches {
		blob := br.Name + strings.Join(br.Quotes, " ")
		score := 0
		if strings.Contains(utterance, "学业") || strings.Contains(utterance, "课业") || strings.Contains(utterance, "成绩") || strings.Contains(utterance, "这学期") {
			if strings.Contains(blob, "学期") || strings.Contains(blob, "稳住") || strings.Contains(blob, "课业") || strings.Contains(blob, "余量") {
				score += 8
			}
			if strings.Contains(blob, "时间") || strings.Contains(blob, "乱") {
				score += 2
			}
		}
		if score > best {
			best = score
			idx = i
		}
	}
	br := forks.Branches[idx]
	var b strings.Builder
	b.WriteString("你刚问的这一点，完整分叉不再贴一遍。")
	b.WriteString(strconv.Itoa(forks.Total))
	b.WriteString(" 人里，走「")
	b.WriteString(br.Name)
	b.WriteString("」的有 ")
	b.WriteString(strconv.Itoa(br.Count))
	b.WriteString(" 人。")
	if len(br.Quotes) > 0 && strings.TrimSpace(br.Quotes[0]) != "" {
		b.WriteString("有人说过：「")
		b.WriteString(br.Quotes[0])
		b.WriteString("」")
	}
	if strings.TrimSpace(forks.SwitchNote) != "" && (strings.Contains(utterance, "学业") || strings.Contains(utterance, "课业") || strings.Contains(utterance, "时间")) {
		b.WriteString(" ")
		b.WriteString(forks.SwitchNote)
	}
	b.WriteString(" 两头都有代价，我不选边。要动手了就说「我该怎么做」，去匹配能做完这件事的卡。")
	return b.String()
}