// WowSkillLand 对话可检索的本地知识：入口（路口）、经验原话、可装载 Skill。
// 检索是关键词打分，不编数据；模型只能引用这里出现过的人数和原话。
package main

import (
	"strings"
	"unicode/utf8"
)

type wowExpDoc struct {
	JunctionID string
	Branch     string
	Quote      string
	By         string
}

type wowJunctionDoc struct {
	ID         string
	Title      string
	Total      int
	Source     string
	SwitchNote string
	Keywords   string
	Branches   []wowForkBranch
	SkillIDs   []string
}

type wowSkillDoc struct {
	ID        string
	Title     string
	Subtitle  string
	Match     string
	Boundary  string
	Keywords  string
	Junctions []string
}

type wowRetrieved struct {
	Junctions   []wowJunctionDoc
	Experiences []wowExpDoc
	Skills      []wowSkillDoc
}

func wowKnowledgeJunctions() []wowJunctionDoc {
	return []wowJunctionDoc{
		{
			ID: "j-friend", Title: "大一 · 一个朋友都没有之后", Total: 31,
			Source: "本校 2022–2025 级开学社交回顾（n=31）· 只显示绝对人数，不给建议",
			SwitchNote: "改道最常发生在开学第三四周——31 人里有 9 人说，真正开始有朋友，不是因为性格变了，是找到一个固定会出现的场合。",
			Keywords: "交朋友 没朋友 一个朋友 没有朋友 孤独 社恐 搭子 认识人 社交 开口 破冰",
			SkillIDs: []string{"s04", "s05"},
			Branches: []wowForkBranch{
				{Name: "🤝 从搭子开始", Count: 15, Quotes: []string{"我没想交挚友。食堂问一句这儿有人吗，后来那个人成了大学第一个固定饭局。 ——2024 级 · 化工"}},
				{Name: "📚 先把课和宿舍稳住", Count: 10, Quotes: []string{"那时候我连开口都怕。先把早八去上了，脸熟是后来才发生的。 ——2023 级 · 数学"}},
				{Name: "🚶 等别人来找我", Count: 6, Quotes: []string{"等了一个月，宿舍以外还是零。后来才知道大多数人也在等。 ——2025 级 · 外语"}},
			},
		},
		{
			ID: "j-love", Title: "要不要谈 · 有好感之后", Total: 26,
			Source: "本校 2022–2025 级情感选择回顾（n=26）· 只显示绝对人数，不给建议",
			SwitchNote: "改道最常发生在「还没说出口的那两周」——26 人里有 8 人说，真正改变决定的不是对方，是自己那阵子的课业和时间。",
			Keywords: "恋爱 谈恋爱 好感 表白 对象 暗恋 在一起 分手 该不该谈 学业 课业 余量",
			SkillIDs: []string{"s04", "s05"},
			Branches: []wowForkBranch{
				{Name: "💌 说清楚，开始处", Count: 9, Quotes: []string{"我以为开始了会更乱，其实乱的是瞒着自己的那个月。说开了反而能排时间。 ——2023 级 · 同班"}},
				{Name: "📚 先不谈，把这学期稳住", Count: 11, Quotes: []string{"不是不喜欢，是那学期我没有余量。后来还是处了，只是换了个时间点。 ——2022 级 · 大二下"}},
				{Name: "🚶 好感放着，先当普通朋友", Count: 6, Quotes: []string{"我怕问出口会把现在的关系弄没。先当朋友处了半年，才知道自己是不是真想谈。 ——2024 级 · 社团认识"}},
			},
		},
		{
			ID: "j-y3", Title: "大三 · 保研 / 考研 / 就业 / 考公 / 出国", Total: 41,
			Source: "本校 2022–2025 级真实去向问卷（n=41）· 只显示绝对人数",
			SwitchNote: "改道最常发生在：大三上第 5–8 周——确认专业绩点排名之后。41 人里有 7 人在这个节点更换了原计划。",
			Keywords: "保研 考研 就业 考公 出国 大三 推免 夏令营 秋招 实习 排名 绩点",
			SkillIDs: []string{"s12", "s13", "s14"},
			Branches: []wowForkBranch{
				{Name: "🎓 保研", Count: 14, Quotes: []string{"绩点是从大一开始攒的，大三才决定的人基本都在赌前两年的底子。 ——2022 级 · 计算机 · 已保研本校", "夏令营材料季那一个月，什么都干不了，要提前清场。 ——2023 级 · 软件工程 · 保研外校"}},
				{Name: "💼 就业", Count: 10, Quotes: []string{"秋招比想象早得多，八月就开了。我最后悔的是大三下才做第一份简历。 ——2022 级 · 信息管理 · 后端开发"}},
				{Name: "📚 考研", Count: 9, Quotes: []string{"决定考研那天我退了两个社团，这是我做过最难但最必要的减法。 ——2022 级 · 数学 · 上岸华五"}},
				{Name: "🏛️ 考公", Count: 4, Quotes: []string{"先看你专业能报的岗位数，再决定投入——顺序反了会浪费一年。 ——2020 级 · 行政管理 · 省考上岸"}},
				{Name: "✈️ 出国", Count: 4, Quotes: []string{"语言成绩要在大三上考完，我拖到大三下，直接少了一轮申请。 ——2022 级 · 自动化 · 港三在读"}},
			},
		},
		{
			ID: "j-major", Title: "大二 · 转专业还是留下", Total: 23,
			Source: "本校 2021–2024 级转专业意向追踪（n=23）· 只显示绝对人数",
			SwitchNote: "最集中的后悔不是「转错了」，而是「决定前没去旁听过一节目标专业的课」——23 人中 14 人如此表示。",
			Keywords: "转专业 专业 旁听 辅修 双学位 合不合适 留下",
			SkillIDs: []string{"s11", "s10"},
			Branches: []wowForkBranch{
				{Name: "🔁 转了专业", Count: 9, Quotes: []string{"转过来才发现，逃离的动力撑不过第一个学期，喜欢的动力才可以。 ——2021 级 · 材料→计算机"}},
				{Name: "🏠 留下 + 辅修/双学位", Count: 11, Quotes: []string{"我最后发现我讨厌的不是本专业，是那两门课。辅修给了我第二条腿。 ——2022 级 · 机械 + 辅修计科"}},
				{Name: "⏸️ 先旁听一学期再决定", Count: 3, Quotes: []string{"旁听了一学期数据结构，我确定了要转，也确定了不怕跟不上。 ——2022 级 · 土木→软件"}},
			},
		},
		{
			ID: "j-y0", Title: "大0 · 本省 / 外省 / 复读", Total: 28,
			Source: "本校 2023–2025 级入学去向问卷（n=28）· 只显示绝对人数",
			SwitchNote: "改道最常发生在出分后 48 小时——家人意见和位次表对上的那一晚。28 人里有 6 人改了第一志愿城市。",
			Keywords: "高考 报志愿 本省 外省 复读 择校 城市 学校",
			SkillIDs: []string{"s01", "s02"},
			Branches: []wowForkBranch{
				{Name: "🏠 留在本省", Count: 15, Quotes: []string{"不是没分，是家里希望我周末能回去。这个约束比分数先落地。 ——2024 级 · 本省 211"}},
				{Name: "🚄 去外省", Count: 10, Quotes: []string{"我按专业排，不按城市排。到了才发现想家，但课是对的。 ——2023 级 · 外省 985"}},
				{Name: "🔁 复读一年", Count: 3, Quotes: []string{"复读的代价是同龄人已经在过大学生活。想清楚再上桌。 ——2022 级 · 二战入学"}},
			},
		},
		{
			ID: "j-y4", Title: "大四 · 工作 / 读研 / 空档", Total: 19,
			Source: "本校 2022–2025 级毕业去向追踪（n=19）· 只显示绝对人数",
			SwitchNote: "改道最常发生在春招第一波 offer 落地后——19 人里有 5 人在「签还是再等」之间换了主意。",
			Keywords: "毕业 offer gap 延毕 春招 工作 读研 空档",
			SkillIDs: []string{"s16", "s17"},
			Branches: []wowForkBranch{
				{Name: "💼 去工作", Count: 9, Quotes: []string{"两个 offer 我比的不是薪资，是第一年会不会把人掏空。 ——2023 级 · 已入职"}},
				{Name: "🎓 读研", Count: 7, Quotes: []string{"保研确认那天反而空，因为四年的惯性突然没了。 ——2024 级 · 本校研一"}},
				{Name: "⏸️ 空一档", Count: 3, Quotes: []string{"gap 不是休息，是自己给自己发工资的一年。想清楚再停。 ——2022 级 · gap 后入职"}},
			},
		},
	}
}

func wowKnowledgeSkills() []wowSkillDoc {
	return []wowSkillDoc{
		{ID: "s04", Title: "开学 21 天破冰剧本（社恐版）", Subtitle: "不需要变外向，只需要三个低成本动作", Match: "想有朋友但害怕主动", Boundary: "持续情绪低落期先别装载", Keywords: "破冰 社恐 开口 朋友 孤独 邀约 交朋友 没朋友", Junctions: []string{"j-friend"}},
		{ID: "s05", Title: "找到第一个搭子的 5 个真实场景", Subtitle: "从功能性关系开始，最不尴尬", Match: "不缺勇气缺场合", Boundary: "不要把搭子当挚友预期", Keywords: "搭子 食堂 场合 朋友 社交 交朋友", Junctions: []string{"j-friend"}},
		{ID: "s11", Title: "去旁听目标专业的一节专业课", Subtitle: "决定前先见过", Match: "该不该转专业之前", Boundary: "不回答该不该转", Keywords: "转专业 旁听 专业课", Junctions: []string{"j-major"}},
		{ID: "s10", Title: "给实验室发一封旁听邮件", Subtitle: "看看真实科研长什么样", Match: "因别人都进组了而慌", Boundary: "大四上不适合短期", Keywords: "实验室 科研 保研 进组 邮件", Junctions: []string{"j-major", "j-y3"}},
		{ID: "s12", Title: "决定考研后的第一个 14 天", Subtitle: "先跑通节奏，别用力过猛", Match: "刚下决心考研", Boundary: "保研边缘人先别装；决心未定先看路口", Keywords: "考研 准备 节奏 14天", Junctions: []string{"j-y3"}},
		{ID: "s13", Title: "考公信息差扫盲清单", Subtitle: "先看能报的岗位再投入", Match: "想考公但还没摸清信息", Boundary: "不是上岸保证", Keywords: "考公 岗位 省考", Junctions: []string{"j-y3"}},
		{ID: "s14", Title: "用课程项目换第一份实习：简历第一版", Subtitle: "先有一版能投的", Match: "大三下才做简历就晚了", Boundary: "不承诺拿到实习", Keywords: "简历 实习 秋招 就业 项目", Junctions: []string{"j-y3"}},
		{ID: "s01", Title: "分数出来后 72 小时决策清单", Subtitle: "家庭第一代大学生版", Match: "出分后家人意见和位次对不上", Boundary: "不替你选学校", Keywords: "高考 报志愿 出分 家庭", Junctions: []string{"j-y0"}},
		{ID: "s02", Title: "城市 / 学校 / 专业排序提问卡", Subtitle: "把约束问清楚", Match: "本省外省纠结", Boundary: "不给录取预测", Keywords: "城市 学校 专业 本省 外省", Junctions: []string{"j-y0"}},
		{ID: "s16", Title: "两个 offer 怎么比：代价清单提问卡", Subtitle: "比的不是薪资是第一年会不会被掏空", Match: "手里有 offer 在比", Boundary: "不替你签", Keywords: "offer 比较 工作 毕业", Junctions: []string{"j-y4"}},
		{ID: "s17", Title: "毕设 + 春招并行的周模板", Subtitle: "两件硬事叠在一起时怎么排", Match: "大四两边都在赶", Boundary: "不承诺都做成", Keywords: "毕设 春招 并行 大四", Junctions: []string{"j-y4"}},
	}
}

func wowRetrieve(query string, preferJunction string, wantSkills bool) wowRetrieved {
	_ = wantSkills
	q := strings.TrimSpace(query)
	var out wowRetrieved
	type scoredJ struct {
		doc   wowJunctionDoc
		score int
	}
	var js []scoredJ
	for _, j := range wowKnowledgeJunctions() {
		blob := j.Title + " " + j.Keywords + " " + j.SwitchNote
		for _, br := range j.Branches {
			blob += " " + br.Name + " " + strings.Join(br.Quotes, " ")
		}
		s := wowScore(q, blob)
		if preferJunction != "" && j.ID == preferJunction {
			s += 12
		}
		if s > 0 {
			js = append(js, scoredJ{j, s})
		}
	}
	for i := 0; i < len(js); i++ {
		for k := i + 1; k < len(js); k++ {
			if js[k].score > js[i].score {
				js[i], js[k] = js[k], js[i]
			}
		}
	}
	if len(js) == 0 && preferJunction != "" {
		for _, j := range wowKnowledgeJunctions() {
			if j.ID == preferJunction {
				js = append(js, scoredJ{j, 1})
				break
			}
		}
	}
	if len(js) > 2 {
		js = js[:2]
	}
	var skillBoost = map[string]int{}
	for _, item := range js {
		out.Junctions = append(out.Junctions, item.doc)
		for _, id := range item.doc.SkillIDs {
			skillBoost[id] += 6
		}
		type scoredE struct {
			e     wowExpDoc
			score int
		}
		var es []scoredE
		for _, br := range item.doc.Branches {
			for _, qt := range br.Quotes {
				e := wowExpDoc{JunctionID: item.doc.ID, Branch: br.Name, Quote: qt}
				es = append(es, scoredE{e, wowScore(q, br.Name+" "+qt) + item.score/2})
			}
		}
		for i := 0; i < len(es); i++ {
			for k := i + 1; k < len(es); k++ {
				if es[k].score > es[i].score {
					es[i], es[k] = es[k], es[i]
				}
			}
		}
		n := 2
		if n > len(es) {
			n = len(es)
		}
		for i := 0; i < n; i++ {
			out.Experiences = append(out.Experiences, es[i].e)
		}
	}
	type scoredS struct {
		doc   wowSkillDoc
		score int
	}
	var ss []scoredS
	for _, sk := range wowKnowledgeSkills() {
		blob := sk.Title + " " + sk.Subtitle + " " + sk.Match + " " + sk.Keywords
		s := wowScore(q, blob) + skillBoost[sk.ID]
		if preferJunction != "" {
			for _, jid := range sk.Junctions {
				if jid == preferJunction {
					s += 4
				}
			}
		}
		if s > 0 {
			ss = append(ss, scoredS{sk, s})
		}
	}
	for i := 0; i < len(ss); i++ {
		for k := i + 1; k < len(ss); k++ {
			if ss[k].score > ss[i].score {
				ss[i], ss[k] = ss[k], ss[i]
			}
		}
	}
	if len(ss) > 3 {
		ss = ss[:3]
	}
	for _, item := range ss {
		out.Skills = append(out.Skills, item.doc)
	}
	return out
}

func wowScore(query, blob string) int {
	if query == "" || blob == "" {
		return 0
	}
	s := 0
	keys := []string{
		"恋爱", "谈恋爱", "好感", "表白", "喜欢", "对象", "学业", "课业", "成绩", "绩点", "这学期", "余量", "时间",
		"保研", "考研", "就业", "考公", "出国", "实习", "简历", "秋招", "夏令营",
		"转专业", "旁听", "专业", "辅修",
		"高考", "报志愿", "本省", "外省", "复读",
		"毕业", "offer", "gap", "春招", "毕设",
		"交朋友", "没朋友", "孤独", "社恐", "搭子", "破冰", "开口",
		"帮我做", "怎么准备", "我想试试", "装载",
	}
	for _, k := range keys {
		if strings.Contains(query, k) && strings.Contains(blob, k) {
			s += utf8.RuneCountInString(k)
		}
	}
	if strings.Contains(blob, query) {
		s += 8
	}
	return s
}

func junctionDocByID(id string) *wowJunctionDoc {
	for _, j := range wowKnowledgeJunctions() {
		if j.ID == id {
			cp := j
			return &cp
		}
	}
	return nil
}
