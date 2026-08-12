import { useDemoStore } from "@/store/DemoStore";
import { assetKindLabel, formatDate, getUser } from "@/utils";
import { ArrowLeft, FileText, Link2, LockKeyhole } from "lucide-react";
import { Link, useParams } from "react-router-dom";

export default function SharePage() {
  const { state } = useDemoStore();
  const { kind, id } = useParams();
  const record = kind === "record" ? state.records.find((item) => item.id === id) : undefined;
  const asset = kind === "asset" ? state.assets.find((item) => item.id === id) : undefined;
  const item = record ?? asset;
  if (!item || item.visibility === "private") return <div className="product-page page-container not-found"><LockKeyhole size={30} /><h1>这个内容不可访问</h1><p>它可能是私密内容，或分享链接已经失效。</p><Link className="button button--primary" to="/">返回首页</Link></div>;
  const author = getUser(state.users, item.authorId)!;
  return <div className="product-page share-page page-container"><Link className="back-link" to="/"><ArrowLeft size={15} />返回 WowSkillsLand</Link><article className="share-card"><span className="eyebrow"><Link2 size={14} />仅通过链接访问</span><h1>{item.title}</h1><p className="share-meta">{author.name} · {formatDate(record?.createdAt ?? asset!.updatedAt, true)}</p>{record ? <><p className="share-body">{record.body}</p>{record.attachments.length > 0 && <div className="record-attachments">{record.attachments.map((file) => <span key={file.id}><FileText size={15} />{file.name}</span>)}</div>}</> : <><span className="asset-kind">{assetKindLabel[asset!.kind]}</span><p className="share-body">{asset!.summary}</p><dl className="skill-specs"><div><dt>输出</dt><dd>{asset!.output}</dd></div><div><dt>边界</dt><dd>{asset!.boundary}</dd></div></dl></>}<div className="share-notice"><LockKeyhole size={16} />此内容不会出现在搜索、动态流或公开主页中。请谨慎转发链接。</div></article></div>;
}
