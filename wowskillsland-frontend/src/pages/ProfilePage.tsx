import { Avatar } from "@/components/Avatar";
import { CommentThread } from "@/components/CommentThread";
import { SocialActions } from "@/components/SocialActions";
import { useDemoStore } from "@/store/DemoStore";
import type { GeneratedAsset, GrowthRecord, PersonaConfig } from "@/types";
import { assetKindLabel, formatDate, getUser } from "@/utils";
import {
  ArrowLeft,
  ArrowRight,
  Bot,
  Check,
  FileText,
  Globe2,
  Heart,
  LockKeyhole,
  MessageCircle,
  Send,
  ShieldCheck,
  Sparkles,
  UserPlus,
  X,
} from "lucide-react";
import { type FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useOutletContext, useParams, useSearchParams } from "react-router-dom";

type ProfileTab = "featured" | "assets" | "records";

export default function ProfilePage() {
  const { state, dispatch } = useDemoStore();
  const { openLogin } = useOutletContext<{ openLogin: () => void }>();
  const { handle } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const [tab, setTab] = useState<ProfileTab>("featured");
  const user = state.users.find((item) => item.handle === handle);
  const chatRequested = searchParams.get("chat") === "1";

  if (!user) {
    return <div className="product-page page-container not-found"><span>404</span><h1>没有找到这位创作者</h1><p>主页可能尚未公开，或链接已经发生变化。</p><Link className="button button--primary" to="/explore">返回探索</Link></div>;
  }

  const records = state.records.filter((record) => record.authorId === user.id && record.visibility === "public");
  const featured = records.filter((record) => record.featured);
  const assets = state.assets.filter((asset) => asset.authorId === user.id && asset.visibility === "public" && asset.generationStatus === "ready");
  const persona = state.personas.find((item) => item.authorId === user.id && item.publicEntry && item.generationStatus === "ready");
  const isMe = user.id === state.currentUserId;
  const isFollowing = state.social.followingUserIds.includes(user.id);

  const openChat = () => {
    if (!state.isLoggedIn) { openLogin(); return; }
    setSearchParams((current) => { current.set("chat", "1"); return current; });
  };

  return (
    <div className="product-page profile-page">
      <section className="profile-cover"><div className="profile-cover__scan" /></section>
      <section className="profile-header page-container">
        <Avatar user={user} size="xl" />
        <div className="profile-header__main">
          <div><span className="eyebrow">@{user.handle}</span><h1>{user.name}{user.verified && <ShieldCheck size={20} />}</h1><p>{user.school} · {user.major}</p></div>
          <div className="profile-buttons">
            {!isMe && <button className={`button ${isFollowing ? "button--secondary" : "button--primary"}`} type="button" onClick={() => state.isLoggedIn ? dispatch({ type: "TOGGLE_FOLLOW", userId: user.id }) : openLogin()}>{isFollowing ? <><Check size={15} />已关注</> : <><UserPlus size={15} />关注</>}</button>}
            {persona && <button className="button button--dark" type="button" onClick={openChat}><Bot size={16} />和赛博人格聊聊</button>}
          </div>
        </div>
        <p className="profile-bio">{user.bio}</p>
        <div className="profile-counts"><span><strong>{user.followers}</strong> 关注者</span><span><strong>{user.following}</strong> 正在关注</span><span><strong>{assets.length}</strong> 公开资产</span><span><strong>{records.length}</strong> 成长记录</span></div>
      </section>

      <div className="profile-tabs page-container">
        <button className={tab === "featured" ? "is-active" : ""} type="button" onClick={() => setTab("featured")}>主页精选</button>
        <button className={tab === "assets" ? "is-active" : ""} type="button" onClick={() => setTab("assets")}>公开资产 <span>{assets.length}</span></button>
        <button className={tab === "records" ? "is-active" : ""} type="button" onClick={() => setTab("records")}>成长记录 <span>{records.length}</span></button>
      </div>

      <section className="profile-content page-container">
        <div className="profile-feed">
          {tab === "featured" && <>
            <div className="section-title-row"><div><span className="eyebrow">由创作者亲自选择</span><h2>主页精选</h2></div></div>
            {featured.map((record) => <PublicRecord key={record.id} record={record} onNeedLogin={openLogin} />)}
            {assets.slice(0, 2).map((asset) => <PublicAsset key={asset.id} asset={asset} onNeedLogin={openLogin} />)}
            {featured.length === 0 && assets.length === 0 && <EmptyProfile />}
          </>}
          {tab === "assets" && <>{assets.map((asset) => <PublicAsset key={asset.id} asset={asset} onNeedLogin={openLogin} />)}{assets.length === 0 && <EmptyProfile />}</>}
          {tab === "records" && <>{records.map((record) => <PublicRecord key={record.id} record={record} onNeedLogin={openLogin} />)}{records.length === 0 && <EmptyProfile />}</>}
        </div>
        <aside className="profile-sidebar">
          {persona ? <div className="profile-persona-card"><div className="persona-orb"><Bot size={27} /></div><span className="eyebrow">赛博人格已开放</span><h3>{persona.name}</h3><p>{persona.summary}</p><div className="persona-scope">{persona.knowledgeScopes.map((item) => <span key={item}>{item}</span>)}</div><div className="persona-disclosure">这是基于创作者授权资料生成的 AI 人格，不代表本人实时观点。</div><button className="button button--primary button--block" type="button" onClick={openChat}>开始对话 <ArrowRight size={15} /></button></div> : <div className="sidebar-card"><Bot size={22} /><h3>赛博人格未开放</h3><p>创作者尚未开放公开对话入口。</p></div>}
          <div className="sidebar-card"><h3>主页公开原则</h3><div className="permission-row"><Globe2 size={15} /><span>只展示明确设为公开的内容</span></div><div className="permission-row"><LockKeyhole size={15} /><span>私密与仅链接内容不会出现</span></div></div>
        </aside>
      </section>

      {chatRequested && persona && state.isLoggedIn && <PersonaChat persona={persona} creatorName={user.name} onClose={() => setSearchParams((current) => { current.delete("chat"); return current; })} />}
    </div>
  );
}

function PublicRecord({ record, onNeedLogin }: { record: GrowthRecord; onNeedLogin: () => void }) {
  const { state } = useDemoStore();
  const [comments, setComments] = useState(false);
  const author = getUser(state.users, record.authorId)!;
  return <article className="content-card public-record"><header className="content-card__author"><span><Avatar user={author} /><span><strong>{author.name}</strong><small>{formatDate(record.createdAt, true)}</small></span></span></header><h2>{record.title}</h2><p>{record.body}</p><div className="tag-row">{record.tags.map((tag) => <span key={tag}>#{tag}</span>)}</div><footer className="content-card__footer"><SocialActions item={record} targetType="record" onComment={() => setComments((value) => !value)} onNeedLogin={onNeedLogin} /></footer>{comments && <CommentThread comments={record.comments} target={{ type: "record", id: record.id }} onNeedLogin={onNeedLogin} />}</article>;
}

function PublicAsset({ asset, onNeedLogin }: { asset: GeneratedAsset; onNeedLogin: () => void }) {
  const { state } = useDemoStore();
  const [comments, setComments] = useState(false);
  const report = state.reports.find((item) => item.assetId === asset.id);
  return <article className="content-card public-asset"><header><span className="asset-kind">{assetKindLabel[asset.kind]}</span>{report && <span className="trust-chip"><ShieldCheck size={14} />可信分 {report.score}</span>}</header><h2>{asset.title}</h2><p>{asset.summary}</p><dl className="skill-specs"><div><dt>交付输出</dt><dd>{asset.output}</dd></div><div><dt>使用边界</dt><dd>{asset.boundary}</dd></div></dl><footer className="content-card__footer"><SocialActions item={asset} targetType="asset" showHelpful onComment={() => setComments((value) => !value)} onNeedLogin={onNeedLogin} /></footer>{comments && <CommentThread comments={asset.comments} target={{ type: "asset", id: asset.id }} onNeedLogin={onNeedLogin} />}</article>;
}

function PersonaChat({ persona, creatorName, onClose }: { persona: PersonaConfig; creatorName: string; onClose: () => void }) {
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<{ role: "user" | "persona"; text: string }[]>([{ role: "persona", text: `你好，我是基于${creatorName}授权资料生成的 AI 人格。你可以问我关于${persona.knowledgeScopes.slice(0, 3).join("、")}的问题。` }]);
  const privateCount = usePrivateSourceCount(persona);
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!input.trim()) return;
    const question = input.trim();
    setMessages((current) => [...current, { role: "user", text: question }, { role: "persona", text: publicPersonaAnswer(question, persona) }]);
    setInput("");
  };
  return <div className="persona-chat-layer"><aside className="persona-chat-panel"><header><button className="icon-button" type="button" onClick={onClose}><ArrowLeft size={19} /></button><div><span className="eyebrow">AI 人格对话</span><h2>{persona.name}</h2></div><button className="icon-button" type="button" onClick={onClose}><X size={19} /></button></header><div className="chat-disclosure"><ShieldCheck size={16} /><span>这是基于创作者授权资料生成的 AI 人格，不代表本人实时观点。</span></div><div className="chat-source-summary"><Sparkles size={15} />知识范围：{persona.knowledgeScopes.join("、")}{privateCount > 0 && ` · ${privateCount} 项授权私密资料`}</div><div className="public-chat-messages">{messages.map((message, index) => <div className={`chat-bubble chat-bubble--${message.role}`} key={index}>{message.role === "persona" && <Bot size={15} />}<p>{message.text}</p></div>)}</div><form onSubmit={submit}><input value={input} onChange={(event) => setInput(event.target.value)} placeholder="问一个具体问题…" autoFocus /><button type="submit"><Send size={17} /></button></form><small className="chat-footer-note">AI 可能出错。重要决定请向本人或专业人士核实。</small></aside></div>;
}

function usePrivateSourceCount(persona: PersonaConfig) {
  const { state } = useDemoStore();
  return useMemo(() => state.records.filter((record) => persona.sourceRecordIds.includes(record.id) && record.visibility === "private").length, [persona.sourceRecordIds, state.records]);
}

function publicPersonaAnswer(question: string, persona: PersonaConfig) {
  if (/文件|原文|私密|附件|来源|截图/.test(question)) return "我不能展示授权资料的文件名、原文、附件或具体来源。可以告诉你的是：相关经验支持先把事实、判断和结果分开记录，再决定下一步。";
  if (/你本人|现在认为|替我承诺|保证/.test(question)) return "我不代表创作者本人，也不能替本人表达实时观点或作出承诺。我只能基于已授权资料中的稳定经验回答。";
  if (/医疗|法律|投资|财务/.test(question)) return "这个问题属于我的明确禁区，我不能提供相关决策建议。请咨询具备资质的专业人士。";
  return `基于我被授权的${persona.knowledgeScopes.slice(0, 2).join("与") || "成长经验"}，我建议先把问题缩小到一个最近发生的具体时刻：你当时看到了什么证据，做了什么判断，结果是什么？然后只设计一个今天能验证的动作。`;
}

function EmptyProfile() {
  return <div className="empty-state empty-state--large"><FileText size={25} /><h3>这里还没有公开内容</h3><p>创作者可以在自己的中心选择要展示的记录和资产。</p></div>;
}
