import { Avatar } from "@/components/Avatar";
import { useDemoStore } from "@/store/DemoStore";
import { getUser, recommendationReason, taskMatchScore } from "@/utils";
import {
  ArrowRight,
  Check,
  FileText,
  Route,
  ShieldCheck,
  Sparkles,
  Upload,
  X,
} from "lucide-react";
import { type FormEvent, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";

const defaultTask = "我想第一次参加大学生创新创业比赛，但不知道该从哪里开始，也不清楚自己适合做什么。";

export default function HomePage() {
  const { state } = useDemoStore();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const resultDialogRef = useRef<HTMLDialogElement>(null);
  const [task, setTask] = useState(defaultTask);
  const [fileName, setFileName] = useState("");
  const [status, setStatus] = useState<"idle" | "analyzing" | "error">("idle");
  const [routeReady, setRouteReady] = useState(false);

  const matches = useMemo(
    () => state.assets
      .filter((asset) => asset.kind === "skill" && asset.visibility === "public" && asset.generationStatus === "ready")
      .sort((a, b) => taskMatchScore(b, task, state.reports) - taskMatchScore(a, task, state.reports))
      .slice(0, 3),
    [state.assets, state.reports, task],
  );

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (task.trim().length < 8) {
      setStatus("error");
      return;
    }
    setStatus("analyzing");
    setRouteReady(false);
    window.setTimeout(() => {
      setStatus("idle");
      resultDialogRef.current?.showModal();
    }, 850);
  };

  return (
    <section className="home-page">
      <div className="home-hero">
        <h1>你想把哪一次，<span>真正做成？</span></h1>
        <p className="home-lead">说出眼前的真实任务，AI 会为你匹配经过验证的 Skill，并组合成可执行路径。</p>

        <form
          className={`task-composer${status === "error" ? " is-error" : ""}`}
          onSubmit={submit}
          aria-busy={status === "analyzing"}
        >
          <label className="sr-only" htmlFor="home-task">你现在想完成什么？</label>
          <textarea
            id="home-task"
            className="task-composer__prompt"
            rows={3}
            value={task}
            aria-invalid={status === "error"}
            aria-describedby={status === "error" ? "home-task-error" : undefined}
            onChange={(event) => {
              setTask(event.target.value);
              if (status === "error") setStatus("idle");
            }}
            onKeyDown={(event) => {
              if ((event.ctrlKey || event.metaKey) && event.key === "Enter") event.currentTarget.form?.requestSubmit();
            }}
          />
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*,.pdf"
            hidden
            onChange={(event) => setFileName(event.target.files?.[0]?.name ?? "")}
          />
          <button
            className="upload-button"
            type="button"
            title={fileName || "添加截图或 PDF 作为参考"}
            aria-label={fileName ? `已选择 ${fileName}，重新选择参考文件` : "添加截图或 PDF 作为参考"}
            onClick={() => fileInputRef.current?.click()}
          >
            <Upload size={18} />
          </button>
          {status === "error" && <span id="home-task-error" className="sr-only" role="alert">请先写下一句具体任务</span>}
          <button className="task-submit" type="submit" disabled={status === "analyzing"}>
            {status === "analyzing" ? <><span className="spinner" />正在匹配</> : "匹配 Skill"}
          </button>
        </form>
      </div>

      <dialog ref={resultDialogRef} className="modal match-modal" onClick={(event) => event.target === event.currentTarget && resultDialogRef.current?.close()}>
        <button className="modal__close icon-button" type="button" onClick={() => resultDialogRef.current?.close()} aria-label="关闭匹配结果"><X size={20} /></button>
        {routeReady ? (
          <div className="route-plan">
            <span className="eyebrow"><Route size={14} />行动路径</span>
            <h2>从今晚能完成的一步开始</h2>
            <p>目标：三周内完成竞赛选题验证，并形成第一版项目说明。</p>
            <div className="route-steps">
              {["今晚：从经历中提取 3 个问题", "本周：完成 5 次真实访谈", "下周：用证据确定一个方向"].map((step, index) => (
                <div key={step}><span>{index + 1}</span><strong>{step}</strong><Check size={17} /></div>
              ))}
            </div>
            <div className="modal-actions">
              <button className="button button--secondary" type="button" onClick={() => setRouteReady(false)}>返回 Skill 组合</button>
              <button className="button button--primary" type="button" onClick={() => navigate(`/explore?q=${encodeURIComponent(task)}`)}>带着任务去探索</button>
            </div>
          </div>
        ) : (
          <>
            <span className="eyebrow"><Sparkles size={14} />按任务适配度与可信证据排序</span>
            <h2>先澄清方向，再验证需求</h2>
            <p className="modal-intro">不是简单按热度推荐。下面的 Skill 同时考虑了任务关键词、评测分数和维护状态。</p>
            <div className="match-list">
              {matches.map((asset, index) => {
                const author = getUser(state.users, asset.authorId)!;
                const report = state.reports.find((item) => item.assetId === asset.id);
                return (
                  <article className={index === 0 ? "is-primary" : ""} key={asset.id}>
                    <div className="match-rank">0{index + 1}</div>
                    <div className="match-content">
                      <div className="match-title-row"><h3>{asset.title}</h3>{report && <span className="trust-chip"><ShieldCheck size={14} />{report.score} 分</span>}</div>
                      <p>{asset.summary}</p>
                      <div className="match-meta"><span><FileText size={14} />{asset.output}</span><span>{recommendationReason(asset, task, state.reports)}</span></div>
                      <div className="author-line"><Avatar user={author} size="sm" /><span>{author.name} · {author.school}</span></div>
                    </div>
                  </article>
                );
              })}
            </div>
            <div className="modal-actions modal-actions--split">
              <Link className="text-link" to={`/explore?q=${encodeURIComponent(task)}`} onClick={() => resultDialogRef.current?.close()}>查看全部匹配结果</Link>
              <button className="button button--primary" type="button" onClick={() => setRouteReady(true)}>组合成行动路径 <ArrowRight size={16} /></button>
            </div>
          </>
        )}
      </dialog>
    </section>
  );
}
