// 完成标准的受控词表。
//
// 赛题说 SKU 要「可识别、可比较、可交易」。可识别我们做到了，可交易设计了，
// 但可比较其实一直是空的——两个简历 Skill，一个的完成标准写「量化项不少于三条」，
// 另一个写「HR 能一眼读懂」，这两个根本没法比。每个 Skill 自说自话，
// 就不存在市场，只存在一堆孤立的东西。
//
// 所以同一个 task_intent 下的 done_criteria 要从公共维度里选。
// 允许加自定义，但自定义不参与比较——这是刻意的：想被比较就用公共语言。
package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// criterion 一条公共完成标准
type criterion struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Hint  string `json:"hint"`
}

// CriteriaVocab 按任务类型的受控词表。
// 只收录「能被第三方一眼判断真假」的标准——这是它能用来比较的前提。
var CriteriaVocab = map[string][]criterion{
	IntentThesisTopic: {
		{"one_sentence", "能用一句话说清研究问题", "说不清就说明范围还没收窄"},
		{"material_supported", "手上材料能支撑这个问题", "数据、算力、访谈对象是否够"},
		{"scope_bounded", "范围能在一个学期内做完", "做不完的题等于没定"},
		{"novelty_checked", "已检索确认不是重复劳动", "查过再定，不凭印象"},
		{"advisor_passed", "导师放行", "最终判据在导师那儿"},
	},
	IntentResumeRewrite: {
		{"quantified", "关键经历有可验证的数字", "至少三条带量化结果"},
		{"jd_aligned", "用词与目标岗位的说法一致", "不用实验室内部术语"},
		{"result_first", "每段以结果开头而不是任务开头", "写做成了什么，不是写做了什么"},
		{"one_page", "控制在一页内", "超过一页说明没做取舍"},
		{"non_expert_readable", "非本专业的人能读懂", "找一个外行读一遍"},
	},
	IntentResumeJDAlign: {
		{"keyword_covered", "JD 的硬性要求逐条对应上", "缺哪条要知道"},
		{"gap_explicit", "明确知道自己缺什么", "缺口写下来才能补"},
		{"priority_ordered", "按 JD 权重重排了顺序", "最相关的放最前"},
	},
	IntentReportStructure: {
		{"one_slide_conclusion", "结论能放进一页", "讲不完说明结构没收敛"},
		{"question_anticipated", "预演过三个最可能被追问的问题", "答不上来就是没准备好"},
		{"evidence_attached", "每个结论都挂着依据", "没依据的结论会被拆"},
		{"time_fitted", "在规定时间内讲完", "超时是最常见的失败"},
	},
	IntentProjectConverge: {
		{"acceptance_defined", "有明确的验收标准", "没标准就会一直扩功能"},
		{"scope_cut", "砍掉了至少一个想做的功能", "一个都没砍说明没收敛"},
		{"demo_runnable", "有一条能跑通的主链路", "能演比功能多重要"},
		{"fallback_ready", "关键环节有兜底方案", "现场挂了要有退路"},
	},
	IntentLiteratureReview: {
		{"query_strategy", "检索策略写下来了", "写下来才能复现和迭代"},
		{"core_papers_found", "找到了该领域的必读几篇", "找不到说明检索词不对"},
		{"structure_drafted", "有一个可以往里填的结构", "边读边搭结构，不是读完再想"},
	},
}

// vocabKeys 该 intent 下的合法 key 集合
func vocabKeys(intent string) map[string]bool {
	out := map[string]bool{}
	for _, c := range CriteriaVocab[intent] {
		out[c.Key] = true
	}
	return out
}

// getCriteriaVocab GET /api/growth/criteria-vocab?intent=thesis_topic
func getCriteriaVocab(c *gin.Context) {
	intent := strings.TrimSpace(c.Query("intent"))
	if intent == "" {
		// 不带参数就返回全量，前端可以一次缓存
		c.JSON(http.StatusOK, gin.H{"data": CriteriaVocab})
		return
	}
	list, ok := CriteriaVocab[intent]
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"data": []criterion{},
			"note": "这个任务类型还没有公共完成标准，你写的会算作自定义，不参与比较。",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": list,
		"note": "从这里选的标准可以和同类 Skill 直接比较；自己写的不参与比较。",
	})
}

// splitCriteria 把存下来的 done_criteria 拆成「公共的」和「自定义的」。
//
// done_criteria 里存的既可能是词表 key，也可能是用户自己写的一句话。
// 公共的才进比较视图，自定义的只展示。
func splitCriteria(intent, rawJSON string) (common []criterion, custom []string) {
	var items []string
	json.Unmarshal([]byte(rawJSON), &items)
	keys := vocabKeys(intent)
	byKey := map[string]criterion{}
	for _, cr := range CriteriaVocab[intent] {
		byKey[cr.Key] = cr
	}
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		if keys[it] {
			common = append(common, byKey[it])
		} else {
			custom = append(custom, it)
		}
	}
	return common, custom
}

// comparableCriteriaCount 有多少条完成标准是可比较的。
// 准入检查用它来判断这个 SKU 到底能不能进市场比较。
func comparableCriteriaCount(intent, rawJSON string) int {
	common, _ := splitCriteria(intent, rawJSON)
	return len(common)
}
