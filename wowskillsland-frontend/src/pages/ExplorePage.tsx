import { Avatar } from "@/components/Avatar";
import { CommentThread } from "@/components/CommentThread";
import { SocialActions } from "@/components/SocialActions";
import { useDemoStore } from "@/store/DemoStore";
import type { GeneratedAsset, GrowthRecord, PersonaConfig } from "@/types";
import { assetKindLabel, formatDate, getUser, recommendationReason, taskMatchScore } from "@/utils";
import {
  ArrowRight,
  BookOpen,
  Bot,
  Check,
  FileCheck2,
  Search,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { useMemo, useState } from "react";
import { Link, useOutletContext, useSearchParams } from "react-router-dom";

type View = "skills" | "records" | "personas";

export default function ExplorePage() {
  const { state } = useDemoStore();
  const { openLogin } = useOutletContext<{ openLogin: () => void }>();
  const [searchParams] = useSearchParams();
  const [query, setQuery] = useState(searchParams.get("q") ?? "");
  const [view, setView] = useState<View>("skills");
  const [category, setCategory] = useState("全部");
  const [trustedOnly, setTrustedOnly] = useState(false);

  const publicAssets = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return state.assets
      .filter((asset) => asset.visibility === "public" && asset.generationStatus === "ready")
      .filter((asset) => category === "全部" || asset.categories.includes(category))
      .filter((asset) => !trustedOnly || Boolean(asset.trustReportId))
      .filter((asset) => !normalized || [asset.title, asset.summary, asset.task, ...asset.categories].join(" ").toLowerCase().includes(normalized))
      .sort((a, b) => taskMatchScore(b, query, state.reports) - taskMatchScore(a, query, state.reports));
  }, [category, query, state.assets, state.reports, trustedOnly]);

  const publicRecords = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return state.records
      .filter((record) => record.visibility === "public")
      .filter((record) => category === "全部" || record.tags.includes(category))
      .filter((record) => !normalized || [record.title, record.body, ...record.tags].join(" ").toLowerCase().includes(normalized))
      .sort((a, b) => +new Date(b.createdAt) - +new Date(a.createdAt));
  }, [category, query, state.records]);

  const publicPersonas = state.personas.filter((persona) => persona.publicEntry && persona.generationStatus === "ready");
  const categories = ["全部", "竞赛", "访谈", "产品", "团队协作", "路演", "复盘"];

  return (
    <div className="product-page explore-page">
      <section className="page-heading page-container">
        <div>
          <span className="eyebrow"><Sparkles size={14} />校园经验正在流动</span>
          <h1>从一个真实任务开始探索</h1>
          <p>找到有边界、有证据、也敢于保留失败案例的经验资产。</p>
        </div>
        <div className="heading-stats">
          <div><strong>186</strong><span>公开资产</span></div>
          <div><strong>92</strong><span>可信报告</span></div>
          <div><strong>38</strong><span>开放人格</span></div>
        </div>
      </section>

      <section className="explore-toolbar page-container">
        <div className="search-box">
          <Search size={18} />
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索任务、经验或创作者" aria-label="搜索探索内容" />
          {query && <button type="button" onClick={() => setQuery("")}>清除</button>}
        </div>
        <div className="view-tabs" role="tablist" aria-label="内容类型">
          <button className={view === "skills" ? "is-active" : ""} type="button" onClick={() => setView("skills")} role="tab" aria-selected={view === "skills"}><FileCheck2 size={16} />Skill <span>{publicAssets.length}</span></button>
          <button className={view === "records" ? "is-active" : ""} type="button" onClick={() => setView("records")} role="tab" aria-selected={view === "records"}><BookOpen size={16} />经验动态 <span>{publicRecords.length}</span></button>
          <button className={view === "personas" ? "is-active" : ""} type="button" onClick={() => setView("personas")} role="tab" aria-selected={view === "personas"}><Bot size={16} />赛博人格 <span>{publicPersonas.length}</span></button>
        </div>
        <div className="filter-row">
          <div className="category-scroll">
            {categories.map((item) => <button key={item} className={category === item ? "is-active" : ""} type="button" onClick={() => setCategory(item)}>{item}</button>)}
          </div>
          {view === "skills" && (
            <label className="check-filter"><input type="checkbox" checked={trustedOnly} onChange={(event) => setTrustedOnly(event.target.checked)} /><span><Check size={13} /></span>仅看已评测</label>
          )}
        </div>
      </section>

      <section className="explore-content explore-content--rooms page-container">
        <header className="explore-results-heading">
          <div>
            <h2>{query ? "按你的任务找到这些房间" : "今天可以进入的经验房间"}</h2>
            <p>{query ? `结果优先考虑“${query.slice(0, 32)}${query.length > 32 ? "…" : ""}”与证据、边界和可信评测的匹配。` : "每张卡片是一份可使用的经验资产，也保留它的作者、证据和边界。"}</p>
          </div>
          <Link to="/trust">可信评测如何工作 <ArrowRight size={15} /></Link>
        </header>

        <div className="feed-column room-grid">
          {view === "skills" && (publicAssets.length ? publicAssets.map((asset) => <SkillCard key={asset.id} asset={asset} query={query} onNeedLogin={openLogin} />) : <EmptyResult />)}
          {view === "records" && (publicRecords.length ? publicRecords.map((record) => <ExperienceCard key={record.id} record={record} onNeedLogin={openLogin} />) : <EmptyResult />)}
          {view === "personas" && (publicPersonas.length ? publicPersonas.map((persona) => <PersonaCard key={persona.authorId} persona={persona} />) : <EmptyResult />)}
        </div>
      </section>
    </div>
  );
}

function SkillCard({ asset, query, onNeedLogin }: { asset: GeneratedAsset; query: string; onNeedLogin: () => void }) {
  const { state, dispatch } = useDemoStore();
  const [commentsOpen, setCommentsOpen] = useState(false);
  const author = getUser(state.users, asset.authorId)!;
  const report = state.reports.find((item) => item.assetId === asset.id);
  const isFollowing = state.social.followingUserIds.includes(author.id);

  return (
    <article className="content-card skill-card room-card">
      <div className="room-card__topline">
        <span className="asset-kind">{assetKindLabel[asset.kind]}</span>
        {report ? <Link className="room-score" to="/trust"><ShieldCheck size={14} />{report.score}</Link> : <span className="room-score room-score--pending">待评测</span>}
      </div>
      <h2>{asset.title}</h2>
      <p className="skill-summary">{asset.summary}</p>
      <div className="room-card__reason"><Sparkles size={14} />{recommendationReason(asset, query, state.reports)}</div>
      <dl className="room-card__facts">
        <div><dt>适用任务</dt><dd>{asset.task}</dd></div>
        <div><dt>交付结果</dt><dd>{asset.output}</dd></div>
      </dl>
      <footer className="room-card__footer">
        <Link className="room-author" to={`/u/${author.handle}`}>
          <Avatar user={author} size="sm" />
          <span><strong>{author.name}{author.verified && <ShieldCheck size={12} />}</strong><small>{author.school}</small></span>
        </Link>
        <div className="room-card__actions">
          {author.id !== state.currentUserId && <button type="button" className={`follow-button${isFollowing ? " is-following" : ""}`} onClick={() => state.isLoggedIn ? dispatch({ type: "TOGGLE_FOLLOW", userId: author.id }) : onNeedLogin()}>{isFollowing ? "已关注" : "关注"}</button>}
          <button className="room-enter" type="button" aria-label={`使用 ${asset.title}`}><ArrowRight size={16} /></button>
        </div>
      </footer>
      <div className="room-card__social"><SocialActions item={asset} targetType="asset" compact onComment={() => setCommentsOpen((value) => !value)} onNeedLogin={onNeedLogin} /></div>
      {commentsOpen && <div className="room-card__comments"><CommentThread comments={asset.comments} target={{ type: "asset", id: asset.id }} onNeedLogin={onNeedLogin} /></div>}
    </article>
  );
}

function ExperienceCard({ record, onNeedLogin }: { record: GrowthRecord; onNeedLogin: () => void }) {
  const { state, dispatch } = useDemoStore();
  const [commentsOpen, setCommentsOpen] = useState(false);
  const author = getUser(state.users, record.authorId)!;
  const isFollowing = state.social.followingUserIds.includes(author.id);

  return (
    <article className="content-card experience-card room-card room-card--record">
      <header className="content-card__author">
        <Link to={`/u/${author.handle}`}><Avatar user={author} /><span><strong>{author.name}</strong><small>{author.school} · {formatDate(record.createdAt)}</small></span></Link>
        {author.id !== state.currentUserId && <button type="button" className={`follow-button${isFollowing ? " is-following" : ""}`} onClick={() => state.isLoggedIn ? dispatch({ type: "TOGGLE_FOLLOW", userId: author.id }) : onNeedLogin()}>{isFollowing ? "已关注" : "关注"}</button>}
      </header>
      <div className="room-card__topline"><span className="asset-kind">经验动态</span><time>{formatDate(record.createdAt)}</time></div>
      <h2>{record.title}</h2>
      <p className="record-body">{record.body}</p>
      <div className="tag-row">{record.tags.map((tag) => <span key={tag}>#{tag}</span>)}</div>
      <footer className="content-card__footer"><SocialActions item={record} targetType="record" compact onComment={() => setCommentsOpen((value) => !value)} onNeedLogin={onNeedLogin} /></footer>
      {commentsOpen && <CommentThread comments={record.comments} target={{ type: "record", id: record.id }} onNeedLogin={onNeedLogin} />}
    </article>
  );
}

function PersonaCard({ persona }: { persona: PersonaConfig }) {
  const { state } = useDemoStore();
  const author = getUser(state.users, persona.authorId)!;
  return (
    <article className="content-card persona-card room-card room-card--persona">
      <div className="persona-card__top"><Avatar user={author} size="lg" /><div><span className="asset-kind"><Bot size={13} />赛博人格</span><h2>{persona.name}</h2><p>由 {author.name} 授权维护</p></div></div>
      <p className="persona-summary">{persona.summary}</p>
      <div className="persona-scope"><strong>可以聊</strong>{persona.knowledgeScopes.map((item) => <span key={item}>{item}</span>)}</div>
      <div className="persona-disclosure">这是基于创作者授权资料生成的 AI 人格，不代表本人实时观点。</div>
      <Link className="button button--primary button--block" to={`/u/${author.handle}?chat=1`}>进入对话 <ArrowRight size={16} /></Link>
    </article>
  );
}

function EmptyResult() {
  return <div className="empty-state empty-state--large"><Search size={28} /><h2>暂时没有匹配内容</h2><p>试着缩短关键词，或切换一个内容视图。</p></div>;
}
