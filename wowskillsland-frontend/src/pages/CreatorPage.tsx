import { Avatar } from "@/components/Avatar";
import { makeId, useDemoStore } from "@/store/DemoStore";
import type { AssetKind, AttachmentMeta, GeneratedAsset, GrowthRecord, PersonaConfig, Visibility } from "@/types";
import { assetKindLabel, formatDate, formatFileSize, visibilityLabel } from "@/utils";
import {
  ArrowRight,
  AtSign,
  Bot,
  Check,
  CheckCircle2,
  ChevronRight,
  CircleDashed,
  Copy,
  File,
  FileText,
  Globe2,
  Image,
  Link2,
  LoaderCircle,
  LockKeyhole,
  MessageCircle,
  PenLine,
  Plus,
  RefreshCcw,
  Send,
  Settings2,
  ShieldAlert,
  Sparkles,
  Upload,
  Users,
  WandSparkles,
  X,
} from "lucide-react";
import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { Link as RouterLink } from "react-router-dom";

type CreatorView = "records" | "assets" | "persona";

export default function CreatorPage() {
  const { state } = useDemoStore();
  const [view, setView] = useState<CreatorView>("records");
  const [selectedRecords, setSelectedRecords] = useState<string[]>([]);
  const currentUser = state.users.find((user) => user.id === state.currentUserId)!;
  const records = state.records.filter((record) => record.authorId === state.currentUserId);
  const assets = state.assets.filter((asset) => asset.authorId === state.currentUserId);
  const persona = state.personas.find((item) => item.authorId === state.currentUserId);

  const toggleRecord = (id: string) => {
    setSelectedRecords((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  };

  return (
    <div className="product-page creator-page">
      <section className="creator-home page-container">
        <aside className="creator-profile-card">
          <div className="creator-profile-card__cover" />
          <Avatar user={currentUser} size="xl" />
          <h1>{currentUser.name}</h1>
          <p className="creator-handle"><AtSign size={14} />{currentUser.handle}</p>
          <p className="creator-school">{currentUser.school}<br />{currentUser.major}</p>
          <p className="creator-bio">{currentUser.bio}</p>
          <dl className="creator-profile-stats">
            <div><dt>{records.length}</dt><dd>成长记录</dd></div>
            <div><dt>{assets.filter((item) => item.generationStatus === "ready").length}</dt><dd>公开资产</dd></div>
            <div><dt>{persona?.sourceRecordIds.length ?? 0}</dt><dd>人格资料</dd></div>
          </dl>
          <RouterLink className="creator-public-link" to={`/u/${currentUser.handle}`}>查看公开主页 <ArrowRight size={15} /></RouterLink>
          <div className="creator-privacy-summary">
            <div><Globe2 size={15} /><span>{records.filter((item) => item.visibility === "public").length} 条公开记录</span></div>
            <div><LockKeyhole size={15} /><span>{records.filter((item) => item.visibility !== "public").length} 条非公开记录</span></div>
            <div><Bot size={15} /><span>{persona?.publicEntry ? "赛博人格已开放" : "赛博人格未开放"}</span></div>
          </div>
        </aside>

        <main className="creator-home__main">
          <header className="creator-home__header">
            <div><h2>我的成长主页</h2><p>把经历按时间留下来，再决定哪些内容可以变成公开经验。</p></div>
            <div className="creator-tabs" role="tablist" aria-label="创作者中心功能">
              <button className={view === "records" ? "is-active" : ""} type="button" onClick={() => setView("records")}><PenLine size={17} />成长记录</button>
              <button className={view === "assets" ? "is-active" : ""} type="button" onClick={() => setView("assets")}><WandSparkles size={17} />资产工坊 {selectedRecords.length > 0 && <span>{selectedRecords.length}</span>}</button>
              <button className={view === "persona" ? "is-active" : ""} type="button" onClick={() => setView("persona")}><Bot size={17} />赛博人格 {persona?.publicEntry && <i />}</button>
            </div>
          </header>

          {view === "records" && <RecordsView records={records} selectedRecords={selectedRecords} toggleRecord={toggleRecord} goAssets={() => setView("assets")} />}
          {view === "assets" && <AssetsView records={records} assets={assets} selectedRecords={selectedRecords} toggleRecord={toggleRecord} />}
          {view === "persona" && <PersonaStudio records={records} persona={persona} />}
        </main>
      </section>
    </div>
  );
}

function RecordsView({
  records,
  selectedRecords,
  toggleRecord,
  goAssets,
}: {
  records: GrowthRecord[];
  selectedRecords: string[];
  toggleRecord: (id: string) => void;
  goAssets: () => void;
}) {
  return (
    <section className="creator-layout">
      <div className="creator-feed">
        <RecordComposer />
        <div className="section-title-row"><div><h2>成长时间线</h2><p>判断、尝试和失败都会留在这里。</p></div><span className="muted-count">{records.length} 条记录</span></div>
        <div className="timeline">
          {records.sort((a, b) => +new Date(b.createdAt) - +new Date(a.createdAt)).map((record) => (
            <CreatorRecordCard key={record.id} record={record} selected={selectedRecords.includes(record.id)} onToggle={() => toggleRecord(record.id)} />
          ))}
        </div>
      </div>
      {selectedRecords.length > 0 && (
        <div className="timeline-selection-bar">
          <span>{selectedRecords.length} 条记录已选中，可以组合生成 Skill、文章或模板。</span>
          <button type="button" onClick={goAssets}>去生成资产 <ArrowRight size={15} /></button>
        </div>
      )}
    </section>
  );
}

function RecordComposer() {
  const { state, dispatch } = useDemoStore();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [body, setBody] = useState("");
  const [link, setLink] = useState("");
  const [attachments, setAttachments] = useState<AttachmentMeta[]>([]);
  const [visibility, setVisibility] = useState<Visibility>("private");
  const [featured, setFeatured] = useState(false);
  const [allowAsset, setAllowAsset] = useState(true);
  const [allowPersona, setAllowPersona] = useState(false);
  const [error, setError] = useState("");

  const reset = () => {
    attachments.forEach((item) => item.sessionPreviewUrl && URL.revokeObjectURL(item.sessionPreviewUrl));
    setBody(""); setLink(""); setAttachments([]); setVisibility("private"); setFeatured(false); setAllowAsset(true); setAllowPersona(false); setError(""); setOpen(false);
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const validLink = !link.trim() || /^https?:\/\//i.test(link.trim());
    if (!body.trim() && !link.trim() && attachments.length === 0) {
      setError("至少写下一段记录，或添加一个附件或链接");
      return;
    }
    if (!validLink) {
      setError("外部链接需要以 http:// 或 https:// 开头");
      return;
    }
    const title = body.trim().split(/[。！？\n]/)[0].slice(0, 28) || (attachments[0]?.name ?? "新的成长记录");
    dispatch({
      type: "ADD_RECORD",
      record: {
        id: makeId("record"),
        authorId: state.currentUserId,
        title,
        body: body.trim() || "添加了一份新的过程材料。",
        createdAt: new Date().toISOString(),
        attachments,
        links: link.trim() ? [link.trim()] : [],
        tags: inferTags(body),
        visibility,
        featured: visibility === "public" && featured,
        allowAsset,
        allowPersona,
        likes: 0,
        likedByMe: false,
        bookmarkedByMe: false,
        comments: [],
      },
    });
    reset();
  };

  return (
    <div className={`record-composer${open ? " is-open" : ""}`}>
      {!open ? (
        <button type="button" onClick={() => setOpen(true)}><span className="composer-plus"><Plus size={20} /></span><span><strong>记录今天</strong><small>一个判断、一次失败，或刚刚学会的东西</small></span><ChevronRight size={18} /></button>
      ) : (
        <form onSubmit={submit}>
          <header><div><span className="eyebrow">只为自己开始</span><h2>今天有什么值得留下？</h2></div><button className="icon-button" type="button" onClick={reset} aria-label="关闭记录编辑器"><X size={19} /></button></header>
          <textarea autoFocus value={body} onChange={(event) => { setBody(event.target.value); setError(""); }} placeholder="写下发生了什么、你是怎么判断的，以及下一次会做什么不同的选择…" />
          <div className="composer-input-row"><Link2 size={17} /><input value={link} onChange={(event) => setLink(event.target.value)} placeholder="添加相关链接（可选）" aria-label="相关链接" /></div>
          {attachments.length > 0 && <div className="attachment-list">{attachments.map((item) => <span key={item.id}>{item.type.startsWith("image/") ? <Image size={15} /> : <File size={15} />}<strong>{item.name}</strong><small>{formatFileSize(item.size)}</small><button type="button" onClick={() => setAttachments((current) => current.filter((file) => file.id !== item.id))}><X size={14} /></button></span>)}</div>}
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*,.pdf"
            multiple
            hidden
            onChange={(event) => {
              const files = Array.from(event.target.files ?? []);
              setAttachments((current) => [...current, ...files.map((file) => ({ id: makeId("attachment"), name: file.name, type: file.type || "application/octet-stream", size: file.size, sessionPreviewUrl: file.type.startsWith("image/") ? URL.createObjectURL(file) : undefined }))]);
              event.target.value = "";
            }}
          />
          <div className="composer-settings">
            <label><span>谁可以看到</span><select value={visibility} onChange={(event) => { const value = event.target.value as Visibility; setVisibility(value); if (value !== "public") setFeatured(false); }}><option value="private">私密</option><option value="unlisted">仅链接</option><option value="public">公开</option></select></label>
            <label className="toggle-label"><input type="checkbox" checked={featured} disabled={visibility !== "public"} onChange={(event) => setFeatured(event.target.checked)} /><span />精选到公开主页</label>
            <label className="toggle-label"><input type="checkbox" checked={allowAsset} onChange={(event) => setAllowAsset(event.target.checked)} /><span />允许生成资产</label>
            <label className="toggle-label"><input type="checkbox" checked={allowPersona} onChange={(event) => setAllowPersona(event.target.checked)} /><span />允许用于赛博人格</label>
          </div>
          {error && <p className="form-error" role="alert">{error}</p>}
          <footer><button className="button button--secondary" type="button" onClick={() => fileInputRef.current?.click()}><Upload size={16} />图片或 PDF</button><button className="button button--primary" type="submit">保存记录 <Check size={16} /></button></footer>
        </form>
      )}
    </div>
  );
}

function CreatorRecordCard({ record, selected, onToggle }: { record: GrowthRecord; selected: boolean; onToggle: () => void }) {
  const { dispatch } = useDemoStore();
  const [notice, setNotice] = useState("");
  const update = (patch: Partial<GrowthRecord>) => dispatch({ type: "UPDATE_RECORD", id: record.id, patch });

  const copyShare = async () => {
    const url = `${window.location.origin}/share/record/${record.id}`;
    try { await navigator.clipboard.writeText(url); setNotice("分享链接已复制"); }
    catch { setNotice(url); }
    window.setTimeout(() => setNotice(""), 2200);
  };

  return (
    <article className={`creator-record${selected ? " is-selected" : ""}`}>
      <div className="timeline-dot" />
      <header>
        <div><span className={`visibility-pill visibility-pill--${record.visibility}`}>{record.visibility === "private" ? <LockKeyhole size={12} /> : record.visibility === "public" ? <Globe2 size={12} /> : <Link2 size={12} />}{visibilityLabel[record.visibility]}</span><time>{formatDate(record.createdAt, true)}</time></div>
        <label className={`select-record${record.allowAsset ? "" : " is-disabled"}`}><input type="checkbox" checked={selected} disabled={!record.allowAsset} onChange={onToggle} /><span><Check size={13} /></span>{selected ? "已选" : "用于生成"}</label>
      </header>
      <h3>{record.title}</h3>
      <p>{record.body}</p>
      {record.attachments.length > 0 && <div className="record-attachments">{record.attachments.map((item) => <span key={item.id}>{item.type.startsWith("image/") ? <Image size={15} /> : <FileText size={15} />}{item.name}<small>{formatFileSize(item.size)}</small></span>)}</div>}
      {record.links.map((item) => <a key={item} href={item} target="_blank" rel="noreferrer" className="record-link"><Link2 size={14} />{item}</a>)}
      <div className="tag-row">{record.tags.map((tag) => <span key={tag}>#{tag}</span>)}</div>
      <div className="record-controls">
        <label><span>可见性</span><select value={record.visibility} onChange={(event) => { const value = event.target.value as Visibility; update({ visibility: value, featured: value === "public" ? record.featured : false }); }}><option value="private">私密</option><option value="unlisted">仅链接</option><option value="public">公开</option></select></label>
        <label className="mini-toggle"><input type="checkbox" checked={record.featured} disabled={record.visibility !== "public"} onChange={(event) => update({ featured: event.target.checked })} /><span />主页精选</label>
        <label className="mini-toggle"><input type="checkbox" checked={record.allowAsset} onChange={(event) => update({ allowAsset: event.target.checked })} /><span />生成资产</label>
        <label className="mini-toggle"><input type="checkbox" checked={record.allowPersona} onChange={(event) => update({ allowPersona: event.target.checked })} /><span />人格资料</label>
        {record.visibility === "unlisted" && <button type="button" className="copy-link-button" onClick={copyShare}><Copy size={14} />复制链接</button>}
      </div>
      {notice && <span className="inline-notice">{notice}</span>}
    </article>
  );
}

function AssetsView({ records, assets, selectedRecords, toggleRecord }: { records: GrowthRecord[]; assets: GeneratedAsset[]; selectedRecords: string[]; toggleRecord: (id: string) => void }) {
  const { state, dispatch } = useDemoStore();
  const [kind, setKind] = useState<AssetKind>("skill");
  const [status, setStatus] = useState<"idle" | "analyzing" | "error">("idle");
  const [error, setError] = useState("");
  const timerRef = useRef<number>();
  const eligible = records.filter((record) => record.allowAsset);
  const selected = eligible.filter((record) => selectedRecords.includes(record.id));

  useEffect(() => {
    return () => {
      if (timerRef.current) window.clearTimeout(timerRef.current);
    };
  }, []);

  const generate = () => {
    if (selected.length === 0) { setError("请先选择至少一条允许生成资产的记录"); return; }
    setError(""); setStatus("analyzing");
    timerRef.current = window.setTimeout(() => {
      const first = selected[0];
      const titlePrefix = kind === "skill" ? "从经历中提炼的" : kind === "article" ? "复盘：" : "可复用的";
      const asset: GeneratedAsset = {
        id: makeId("asset"), authorId: state.currentUserId, kind,
        title: `${titlePrefix}${first.title}`.slice(0, 34),
        summary: selected.map((item) => item.body).join(" ").slice(0, 150),
        task: kind === "skill" ? "遇到类似校园项目时，快速判断下一步" : "回顾并复用这组成长经验",
        output: kind === "skill" ? "分步行动建议与检查清单" : kind === "article" ? "一篇结构化经验文章" : "一份可复制填写的模板",
        boundary: "仅基于已授权记录生成，使用前需要结合自己的具体情境判断。",
        evidence: `${selected.length} 条本人授权成长记录，包含过程判断与复盘。`,
        sourceRecordIds: selected.map((item) => item.id), generationStatus: "draft", visibility: "private",
        updatedAt: new Date().toISOString(), categories: Array.from(new Set(selected.flatMap((item) => item.tags))).slice(0, 4),
        likes: 0, likedByMe: false, bookmarkedByMe: false, helpful: 0, helpfulByMe: false, comments: [],
      };
      dispatch({ type: "ADD_ASSET", asset }); setStatus("idle");
    }, 1050);
  };

  return (
    <section className="asset-workshop page-container">
      <div className="asset-builder">
        <div className="section-title-row"><div><span className="eyebrow">从记录到可复用资产</span><h2>选择材料，生成第一版草稿</h2></div><span className="privacy-note"><LockKeyhole size={14} />只处理已授权内容</span></div>
        <div className="builder-step"><span className="step-number">1</span><div><h3>选择成长记录</h3><p>私密记录也可以参与生成，但不会因此自动公开。</p></div></div>
        <div className="source-picker">
          {eligible.map((record) => <label key={record.id} className={selectedRecords.includes(record.id) ? "is-selected" : ""}><input type="checkbox" checked={selectedRecords.includes(record.id)} onChange={() => toggleRecord(record.id)} /><span className="source-check"><Check size={14} /></span><span><strong>{record.title}</strong><small>{visibilityLabel[record.visibility]} · {formatDate(record.createdAt)}</small></span></label>)}
        </div>
        <div className="builder-step"><span className="step-number">2</span><div><h3>选择输出类型</h3><p>生成结果始终先进入草稿，不会自动发布。</p></div></div>
        <div className="asset-kind-picker">
          {(["skill", "article", "template"] as AssetKind[]).map((item) => <button key={item} type="button" className={kind === item ? "is-active" : ""} onClick={() => setKind(item)}>{item === "skill" ? <Sparkles size={19} /> : item === "article" ? <FileText size={19} /> : <Settings2 size={19} />}<strong>{assetKindLabel[item]}</strong><small>{item === "skill" ? "可被任务匹配调用" : item === "article" ? "讲清经验与判断" : "变成可重复填写的结构"}</small></button>)}
        </div>
        {error && <p className="form-error">{error}</p>}
        <div className="builder-actions">
          {status === "analyzing" && <span><LoaderCircle className="spin" size={17} />正在分析 {selected.length} 条记录，提取共同判断…</span>}
          <button className="button button--primary" type="button" disabled={status === "analyzing"} onClick={generate}>{status === "analyzing" ? "生成中" : `生成${assetKindLabel[kind]}草稿`} <WandSparkles size={16} /></button>
          {status === "analyzing" && <button className="text-button" type="button" onClick={() => { if (timerRef.current) window.clearTimeout(timerRef.current); setStatus("idle"); }}>取消</button>}
        </div>
      </div>
      <div className="asset-drafts">
        <div className="section-title-row"><div><span className="eyebrow">草稿与发布</span><h2>我的经验资产</h2></div><span className="muted-count">{assets.length} 个</span></div>
        {assets.length === 0 ? <div className="empty-state empty-state--large"><WandSparkles size={26} /><h3>还没有资产草稿</h3><p>从左侧选择记录开始。</p></div> : assets.sort((a, b) => +new Date(b.updatedAt) - +new Date(a.updatedAt)).map((asset) => <AssetDraftCard key={asset.id} asset={asset} />)}
      </div>
    </section>
  );
}

function AssetDraftCard({ asset }: { asset: GeneratedAsset }) {
  const { dispatch } = useDemoStore();
  const [editing, setEditing] = useState(asset.generationStatus === "draft");
  const update = (patch: Partial<GeneratedAsset>) => dispatch({ type: "UPDATE_ASSET", id: asset.id, patch });
  return (
    <article className="asset-draft-card">
      <header><span className="asset-kind">{assetKindLabel[asset.kind]}</span><span className={`draft-status draft-status--${asset.generationStatus}`}>{asset.generationStatus === "ready" ? "已发布" : "草稿"}</span></header>
      {editing ? (
        <div className="draft-editor">
          <label>标题<input value={asset.title} onChange={(event) => update({ title: event.target.value })} /></label>
          <label>摘要<textarea value={asset.summary} onChange={(event) => update({ summary: event.target.value })} /></label>
          <label>输出<input value={asset.output} onChange={(event) => update({ output: event.target.value })} /></label>
          <label>使用边界<textarea value={asset.boundary} onChange={(event) => update({ boundary: event.target.value })} /></label>
        </div>
      ) : <><h3>{asset.title}</h3><p>{asset.summary}</p><dl><div><dt>输出</dt><dd>{asset.output}</dd></div><div><dt>边界</dt><dd>{asset.boundary}</dd></div></dl></>}
      <div className="draft-card-footer">
        <label>可见性<select value={asset.visibility} onChange={(event) => update({ visibility: event.target.value as Visibility })}><option value="private">私密</option><option value="unlisted">仅链接</option><option value="public">公开</option></select></label>
        <button className="button button--secondary button--small" type="button" onClick={() => setEditing((value) => !value)}>{editing ? "完成编辑" : "编辑"}</button>
        {asset.generationStatus !== "ready" && <button className="button button--primary button--small" type="button" onClick={() => dispatch({ type: "PUBLISH_ASSET", id: asset.id })}>确认发布</button>}
      </div>
      {asset.visibility === "unlisted" && asset.generationStatus === "ready" && <RouterLink className="text-link" to={`/share/asset/${asset.id}`}><Link2 size={14} />打开仅链接页面</RouterLink>}
    </article>
  );
}

function PersonaStudio({ records, persona }: { records: GrowthRecord[]; persona?: PersonaConfig }) {
  const { state, dispatch } = useDemoStore();
  const eligible = records.filter((record) => record.allowPersona);
  const [selected, setSelected] = useState<string[]>(persona?.sourceRecordIds ?? eligible.slice(0, 2).map((item) => item.id));
  const [analyzing, setAnalyzing] = useState(false);
  const [error, setError] = useState("");
  const [question, setQuestion] = useState("");
  const [messages, setMessages] = useState<{ role: "user" | "persona"; text: string }[]>([]);
  const timerRef = useRef<number>();

  useEffect(() => {
    setSelected((current) => current.filter((id) => eligible.some((record) => record.id === id)));
  }, [eligible.length]);
  useEffect(() => {
    return () => {
      if (timerRef.current) window.clearTimeout(timerRef.current);
    };
  }, []);

  const generate = () => {
    if (selected.length === 0) { setError("至少选择一条授权资料"); return; }
    setError(""); setAnalyzing(true);
    timerRef.current = window.setTimeout(() => {
      dispatch({ type: "UPSERT_PERSONA", persona: {
        authorId: state.currentUserId, sourceRecordIds: selected, generationStatus: "draft",
        name: persona?.name ?? "林夏的项目复盘搭子",
        summary: "基于成长记录中的项目调研、失败复盘和团队协作经验，帮助提问者把模糊问题变成下一步行动。",
        tone: "坦诚、具体、不过度承诺；优先追问真实经历，再给出一个小实验。",
        traits: ["重视证据", "允许不确定", "善于复盘"],
        knowledgeScopes: Array.from(new Set(eligible.filter((record) => selected.includes(record.id)).flatMap((record) => record.tags))).slice(0, 5),
        blockedTopics: ["替本人做价值承诺", "披露未公开原始资料", "医疗、法律和财务决策"],
        publicEntry: false, needsRebuild: false,
      } });
      setAnalyzing(false); setMessages([]);
    }, 1200);
  };

  const currentPersona = persona;
  const update = (patch: Partial<PersonaConfig>) => currentPersona && dispatch({ type: "UPDATE_PERSONA", authorId: currentPersona.authorId, patch });
  const sendTest = (event: FormEvent) => {
    event.preventDefault();
    if (!question.trim() || !currentPersona) return;
    const text = question.trim();
    setMessages((current) => [...current, { role: "user", text }, { role: "persona", text: personaAnswer(text, currentPersona) }]);
    setQuestion("");
  };

  const stage = !currentPersona || currentPersona.needsRebuild ? 1 : currentPersona.generationStatus === "draft" ? 2 : currentPersona.generationStatus === "ready" ? 4 : 1;

  return (
    <section className="persona-studio page-container">
      <div className="persona-main">
        <div className="section-title-row"><div><span className="eyebrow">你决定人格知道什么</span><h2>生成一个有边界的赛博人格</h2></div><span className={`persona-live-status${currentPersona?.publicEntry ? " is-live" : ""}`}>{currentPersona?.publicEntry ? "公开入口已开启" : "公开入口已关闭"}</span></div>
        <div className="persona-steps">
          {["选择资料", "编辑草稿", "确认边界", "测试对话", "开放入口"].map((item, index) => <div key={item} className={stage >= index + 1 ? "is-active" : ""}><span>{stage > index + 1 ? <Check size={13} /> : index + 1}</span><small>{item}</small></div>)}
        </div>

        <div className="persona-section">
          <div className="builder-step"><span className="step-number">1</span><div><h3>选择授权资料</h3><p>撤销记录的人格授权会立即关闭公开入口，重新生成并确认后才能再次开放。</p></div></div>
          <div className="persona-source-grid">
            {eligible.map((record) => <label key={record.id} className={selected.includes(record.id) ? "is-selected" : ""}><input type="checkbox" checked={selected.includes(record.id)} onChange={() => setSelected((current) => current.includes(record.id) ? current.filter((id) => id !== record.id) : [...current, record.id])} /><span className="source-check"><Check size={14} /></span><span><strong>{record.visibility === "private" ? record.title : record.title}</strong><small>{record.visibility === "private" ? "私密资料，仅用于已授权生成" : `${visibilityLabel[record.visibility]}资料`}</small></span></label>)}
          </div>
          {eligible.length === 0 && <div className="empty-state"><LockKeyhole size={21} /><p>先在成长记录中开启“允许用于赛博人格”。</p></div>}
          {error && <p className="form-error">{error}</p>}
          <div className="builder-actions">
            {analyzing && <span><LoaderCircle className="spin" size={17} />正在提取表达习惯、知识范围和禁区…</span>}
            <button className="button button--primary" type="button" disabled={analyzing || eligible.length === 0} onClick={generate}>{currentPersona ? "重新生成草稿" : "生成边界草稿"} <Sparkles size={16} /></button>
            {analyzing && <button className="text-button" type="button" onClick={() => { if (timerRef.current) window.clearTimeout(timerRef.current); setAnalyzing(false); }}>取消</button>}
          </div>
        </div>

        {currentPersona && currentPersona.generationStatus !== "idle" && (
          <div className="persona-section persona-draft">
            <div className="builder-step"><span className="step-number">2</span><div><h3>逐项编辑并确认</h3><p>这是模拟生成的草稿。确认前不会影响公开主页。</p></div></div>
            <div className="persona-form-grid">
              <label>人格名称<input value={currentPersona.name} onChange={(event) => update({ name: event.target.value })} /></label>
              <label className="span-2">简介<textarea value={currentPersona.summary} onChange={(event) => update({ summary: event.target.value })} /></label>
              <label className="span-2">表达习惯<textarea value={currentPersona.tone} onChange={(event) => update({ tone: event.target.value })} /></label>
              <CommaEditor label="个性特征" values={currentPersona.traits} onChange={(traits) => update({ traits })} />
              <CommaEditor label="知识范围" values={currentPersona.knowledgeScopes} onChange={(knowledgeScopes) => update({ knowledgeScopes })} />
              <label className="span-2">禁区（每行一项）<textarea value={currentPersona.blockedTopics.join("\n")} onChange={(event) => update({ blockedTopics: event.target.value.split("\n").map((item) => item.trim()).filter(Boolean) })} /></label>
            </div>
            <div className="persona-safety-note"><ShieldAlert size={18} /><span><strong>隐私保护</strong>私密来源在公开对话中只会显示为“授权私密资料”，不会暴露文件名、原文或具体来源。</span></div>
            {currentPersona.generationStatus === "draft" ? <button className="button button--primary" type="button" onClick={() => update({ generationStatus: "ready", confirmedAt: new Date().toISOString(), needsRebuild: false, publicEntry: false })}>确认知识范围与人格边界 <CheckCircle2 size={16} /></button> : <span className="confirmed-line"><CheckCircle2 size={17} />已于 {currentPersona.confirmedAt ? formatDate(currentPersona.confirmedAt, true) : "今天"} 确认</span>}
          </div>
        )}

        {currentPersona?.generationStatus === "ready" && (
          <div className="persona-section">
            <div className="builder-step"><span className="step-number">3</span><div><h3>测试对话</h3><p>确认回复风格和禁区，再决定是否开放入口。</p></div></div>
            <div className="test-chat">
              <div className="test-chat__messages">
                {messages.length === 0 && <div className="chat-placeholder"><MessageCircle size={22} /><p>可以问：“第一次访谈很失败，我应该先改什么？”</p></div>}
                {messages.map((message, index) => <div key={`${message.role}-${index}`} className={`chat-bubble chat-bubble--${message.role}`}>{message.role === "persona" && <Bot size={15} />}<p>{message.text}</p></div>)}
              </div>
              <form onSubmit={sendTest}><input value={question} onChange={(event) => setQuestion(event.target.value)} placeholder="输入一个测试问题" /><button type="submit"><Send size={16} /></button></form>
            </div>
          </div>
        )}
      </div>

      <aside className="persona-sidebar">
        <div className="persona-preview-card">
          <div className="persona-orb"><Bot size={30} /></div>
          <span className="eyebrow">公开入口预览</span>
          <h3>{currentPersona?.name ?? "你的赛博人格"}</h3>
          <p>{currentPersona?.summary ?? "选择授权资料后，这里会出现人格简介。"}</p>
          <div className="persona-disclosure">这是基于创作者授权资料生成的 AI 人格，不代表本人实时观点。</div>
          <label className={`public-switch${currentPersona?.generationStatus === "ready" && !currentPersona.needsRebuild ? "" : " is-disabled"}`}><span><strong>开放公开对话入口</strong><small>{currentPersona?.generationStatus === "ready" && !currentPersona.needsRebuild ? "仅登录用户可对话" : "完成确认后才能开启"}</small></span><input type="checkbox" disabled={currentPersona?.generationStatus !== "ready" || currentPersona.needsRebuild} checked={currentPersona?.publicEntry ?? false} onChange={(event) => update({ publicEntry: event.target.checked })} /><i /></label>
          {currentPersona?.publicEntry && <RouterLink className="button button--secondary button--block" to={`/u/${state.users.find((user) => user.id === state.currentUserId)?.handle}?chat=1`}>查看公开入口 <ArrowRight size={15} /></RouterLink>}
        </div>
        <div className="sidebar-card">
          <h3>资料使用清单</h3>
          <div className="permission-row"><Check size={15} /><span>只使用逐条授权资料</span></div>
          <div className="permission-row"><Check size={15} /><span>不代表本人实时观点</span></div>
          <div className="permission-row"><Check size={15} /><span>撤销授权会立即关闭入口</span></div>
          <div className="permission-row"><Check size={15} /><span>禁区问题会主动拒绝</span></div>
        </div>
      </aside>
    </section>
  );
}

function CommaEditor({ label, values, onChange }: { label: string; values: string[]; onChange: (values: string[]) => void }) {
  return <label>{label}<textarea value={values.join("、")} onChange={(event) => onChange(event.target.value.split(/[、，,]/).map((item) => item.trim()).filter(Boolean))} /></label>;
}

function inferTags(value: string) {
  const tags: string[] = [];
  if (/访谈|用户|需求/.test(value)) tags.push("用户访谈");
  if (/比赛|竞赛|路演/.test(value)) tags.push("竞赛复盘");
  if (/团队|组队|协作/.test(value)) tags.push("团队协作");
  if (/失败|复盘|改/.test(value)) tags.push("失败复盘");
  return tags.length ? tags : ["成长记录"];
}

function personaAnswer(question: string, persona: PersonaConfig) {
  if (/文件名|原文|私密|来源/.test(question)) return "这部分来自创作者授权的私密资料，我不能展示文件名、原文或具体来源。我们可以只讨论其中被允许复用的方法。";
  if (/医疗|法律|投资|财务/.test(question)) return "这超出了我的知识与授权边界，我不能代替专业判断。建议向具备资质的人求助。";
  return `我会先追问一个事实：这次经历里，哪一个具体时刻让你觉得“没有奏效”？基于${persona.knowledgeScopes.slice(0, 2).join("和") || "已授权经验"}，下一步先做一个小实验：把当时的判断、证据和结果各写一句，再只改动一个变量。`;
}
