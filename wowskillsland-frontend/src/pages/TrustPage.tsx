import { Avatar } from "@/components/Avatar";
import { makeId, useDemoStore } from "@/store/DemoStore";
import type { TrustDimensions, TrustReport } from "@/types";
import { formatDate, formatFileSize, getUser } from "@/utils";
import {
  AlertTriangle,
  ArrowRight,
  Check,
  CheckCircle2,
  ChevronRight,
  CircleDashed,
  FileCheck2,
  FileUp,
  Gauge,
  Link2,
  LoaderCircle,
  LockKeyhole,
  RefreshCcw,
  Search,
  ShieldCheck,
  Square,
  UploadCloud,
  Wrench,
  X,
} from "lucide-react";
import { type CSSProperties, type FormEvent, useEffect, useMemo, useRef, useState } from "react";

type SubmitStatus = "idle" | "analyzing" | "draft" | "ready" | "error";

const dimensionLabels: Record<keyof TrustDimensions, string> = {
  validity: "有效性",
  reliability: "可靠性",
  clarity: "可理解性",
  traceability: "可追溯性",
  acceptance: "接受度",
};

export default function TrustPage() {
  const { state } = useDemoStore();
  const [selectedId, setSelectedId] = useState(state.reports[0]?.id ?? "");
  const [query, setQuery] = useState("");
  const [submitOpen, setSubmitOpen] = useState(false);
  const reports = useMemo(() => state.reports.filter((report) => !query || report.title.toLowerCase().includes(query.toLowerCase()) || getUser(state.users, report.authorId)?.name.includes(query)), [query, state.reports, state.users]);
  const selected = state.reports.find((report) => report.id === selectedId) ?? reports[0];

  return (
    <div className="product-page trust-page">
      <section className="page-heading page-container">
        <div><span className="eyebrow"><ShieldCheck size={14} />证据、边界与维护状态</span><h1>可信，不只是一枚徽章</h1><p>每份报告都说明它为何有效、在哪里失效，以及创作者如何维护它。</p></div>
        <button className="button button--primary" type="button" onClick={() => setSubmitOpen(true)}>提交模拟评测 <FileUp size={16} /></button>
      </section>

      <section className="trust-overview page-container">
        <div><Gauge size={20} /><span><strong>{state.reports.length}</strong> 份公开报告</span></div>
        <div><CheckCircle2 size={20} /><span><strong>{state.reports.filter((item) => item.status === "valid").length}</strong> 份状态良好</span></div>
        <div><Wrench size={20} /><span><strong>每学期</strong> 建议复核一次</span></div>
        <p>评测结果是基于提供材料的模拟判断，不构成现实世界的专业认证。</p>
      </section>

      <section className="trust-layout page-container">
        <aside className="report-list-panel">
          <div className="search-box search-box--small"><Search size={17} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索报告或作者" /></div>
          <div className="report-list">
            {reports.map((report) => {
              const author = getUser(state.users, report.authorId)!;
              return (
                <button type="button" key={report.id} className={selected?.id === report.id ? "is-active" : ""} onClick={() => setSelectedId(report.id)}>
                  <span className={`report-score report-score--${report.status}`}>{report.score}</span>
                  <span><strong>{report.title.replace(" · 可信评测", "")}</strong><small>{author.name} · {formatDate(report.createdAt)}</small></span>
                  <ChevronRight size={17} />
                </button>
              );
            })}
          </div>
        </aside>
        {selected ? <ReportDetail report={selected} /> : <div className="empty-state empty-state--large"><Search size={26} /><h2>没有找到报告</h2></div>}
      </section>

      {submitOpen && <EvaluationSubmit onClose={() => setSubmitOpen(false)} />}
    </div>
  );
}

function ReportDetail({ report }: { report: TrustReport }) {
  const { state } = useDemoStore();
  const author = getUser(state.users, report.authorId)!;
  return (
    <article className="report-detail">
      <header className="report-detail__header">
        <div><span className={`report-state report-state--${report.status}`}>{report.status === "valid" ? "状态良好" : report.status === "watch" ? "建议关注" : "有限适用"}</span><h2>{report.title}</h2><div className="author-line"><Avatar user={author} size="sm" /><span>{author.name} · {author.school}</span><time>{formatDate(report.createdAt, true)}</time></div></div>
        <div className="score-ring" style={{ "--score": `${report.score * 3.6}deg` } as CSSProperties}><span><strong>{report.score}</strong><small>综合可信分</small></span></div>
      </header>
      <section className="dimension-section"><h3>五项评测维度</h3><div className="dimension-grid">{(Object.entries(report.dimensions) as [keyof TrustDimensions, number][]).map(([key, value]) => <div key={key}><span><strong>{dimensionLabels[key]}</strong><b>{value}</b></span><div className="score-bar"><i style={{ width: `${value}%` }} /></div></div>)}</div></section>
      <div className="report-grid">
        <section><h3><FileCheck2 size={17} />可追溯证据</h3><ul>{report.evidence.map((item) => <li key={item}><Check size={14} />{item}</li>)}</ul></section>
        <section className="risk-section"><h3><AlertTriangle size={17} />风险与失败案例</h3><ul>{[...report.risks, ...report.failureCases].map((item) => <li key={item}><span />{item}</li>)}</ul></section>
        <section><h3><Square size={17} />适用边界</h3><p>{report.boundary}</p></section>
        <section><h3><LockKeyhole size={17} />权限说明</h3><p>{report.permissions}</p></section>
      </div>
      <footer className="maintenance-row"><RefreshCcw size={17} /><span><strong>维护状态</strong>{report.maintenance}</span></footer>
    </article>
  );
}

function EvaluationSubmit({ onClose }: { onClose: () => void }) {
  const { state, dispatch } = useDemoStore();
  const fileRef = useRef<HTMLInputElement>(null);
  const timerRef = useRef<number>();
  const [status, setStatus] = useState<SubmitStatus>("idle");
  const [url, setUrl] = useState("");
  const [file, setFile] = useState<{ name: string; size: number; type: string } | null>(null);
  const [error, setError] = useState("");
  const [draft, setDraft] = useState<TrustReport | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current) window.clearTimeout(timerRef.current);
    };
  }, []);

  const analyze = (event?: FormEvent) => {
    event?.preventDefault();
    if (!url.trim() && !file) { setStatus("error"); setError("请提供 Skill URL 或上传一个文件"); return; }
    if (url.trim() && !/^https?:\/\//i.test(url.trim())) { setStatus("error"); setError("URL 需要以 http:// 或 https:// 开头"); return; }
    if (file && !["application/pdf", "text/markdown", "text/plain", "application/json"].includes(file.type) && !/\.(pdf|md|txt|json)$/i.test(file.name)) { setStatus("error"); setError("目前只演示 PDF、Markdown、TXT 或 JSON 文件评测"); return; }
    setError(""); setStatus("analyzing");
    timerRef.current = window.setTimeout(() => {
      const name = file?.name.replace(/\.(pdf|md|txt|json)$/i, "") || new URL(url).hostname.replace(/^www\./, "");
      setDraft({
        id: makeId("report"), title: `${name} · 可信评测`, authorId: state.currentUserId, status: "watch", score: 81,
        dimensions: { validity: 84, reliability: 78, clarity: 88, traceability: 80, acceptance: 76 },
        evidence: ["输入与输出定义完整", "包含至少一项可验证结果", "关键步骤可以人工复核"],
        risks: ["当前结果来自前端模拟分析，尚未接入真实评测服务"],
        boundary: "适用于低风险校园学习与项目协作场景，不用于医疗、法律、金融等高风险决策。",
        permissions: "本地原型只保存文件名称、类型与大小，不把文件内容写入 localStorage。",
        maintenance: "首次模拟评测，建议发布前由创作者确认维护周期。",
        failureCases: ["缺少特定领域样本时，可靠性会下降"], createdAt: new Date().toISOString(),
      });
      setStatus("draft");
    }, 1500);
  };

  const cancel = () => { if (timerRef.current) window.clearTimeout(timerRef.current); setStatus("idle"); setError(""); };
  const publish = () => {
    if (!draft) return;
    dispatch({ type: "ADD_REPORT", report: draft });
    setStatus("ready");
  };

  return (
    <div className="evaluation-layer" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="evaluation-panel" role="dialog" aria-modal="true" aria-labelledby="evaluation-title">
        <header><div><span className="eyebrow">模拟流程</span><h2 id="evaluation-title">提交可信评测</h2></div><button className="icon-button" type="button" onClick={onClose} aria-label="关闭提交评测"><X size={20} /></button></header>
        {status === "ready" ? (
          <div className="evaluation-success"><span><CheckCircle2 size={30} /></span><h3>评测报告已生成</h3><p>报告已加入公开列表。这个结果只用于交互原型，不代表真实认证。</p><button className="button button--primary" type="button" onClick={onClose}>查看报告</button></div>
        ) : status === "draft" && draft ? (
          <div className="evaluation-draft"><span className="eyebrow">分析草稿，等待确认</span><div className="draft-score"><strong>{draft.score}</strong><span>综合可信分</span></div><h3>{draft.title}</h3><div className="mini-dimensions">{Object.entries(draft.dimensions).map(([key, value]) => <div key={key}><span>{dimensionLabels[key as keyof TrustDimensions]}</span><strong>{value}</strong></div>)}</div><div className="evaluation-warning"><AlertTriangle size={17} />当前含一项风险提示，发布前请确认适用边界。</div><div className="modal-actions"><button className="button button--secondary" type="button" onClick={() => setStatus("idle")}>返回修改</button><button className="button button--primary" type="button" onClick={publish}>确认并生成报告</button></div></div>
        ) : (
          <form onSubmit={analyze}>
            <p className="evaluation-intro">输入公开 Skill 链接，或上传说明文件。模拟器会检查证据、边界、权限和维护状态。</p>
            <label className="evaluation-url"><span>Skill URL</span><div><Link2 size={17} /><input value={url} onChange={(event) => { setUrl(event.target.value); setStatus("idle"); }} placeholder="https://wowskillsland.example/skill/..." /></div></label>
            <div className="or-divider"><span>或</span></div>
            <input ref={fileRef} type="file" accept=".pdf,.md,.txt,.json" hidden onChange={(event) => { const selected = event.target.files?.[0]; if (selected) { setFile({ name: selected.name, size: selected.size, type: selected.type }); setStatus("idle"); } }} />
            <button className={`file-drop${file ? " has-file" : ""}`} type="button" onClick={() => fileRef.current?.click()}>{file ? <><FileCheck2 size={24} /><span><strong>{file.name}</strong><small>{formatFileSize(file.size)} · 仅保留元数据</small></span></> : <><UploadCloud size={25} /><span><strong>选择评测文件</strong><small>PDF、Markdown、TXT 或 JSON</small></span></>}</button>
            {status === "error" && <div className="form-error" role="alert"><AlertTriangle size={15} />{error}</div>}
            {status === "analyzing" && <div className="analysis-progress"><div><LoaderCircle className="spin" size={18} /><span><strong>正在分析材料</strong><small>检查输出、证据、边界与权限…</small></span></div><div className="indeterminate-bar"><i /></div></div>}
            <footer><small>不会上传到真实服务，也不会保存文件正文。</small><div>{status === "analyzing" && <button className="button button--secondary" type="button" onClick={cancel}>取消</button>}{status === "error" && <button className="button button--secondary" type="button" onClick={() => { setStatus("idle"); setError(""); }}><RefreshCcw size={15} />重试</button>}<button className="button button--primary" type="submit" disabled={status === "analyzing"}>{status === "analyzing" ? "分析中" : "开始模拟评测"} <ArrowRight size={15} /></button></div></footer>
          </form>
        )}
      </section>
    </div>
  );
}
