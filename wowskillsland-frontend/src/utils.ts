import type { AssetKind, GeneratedAsset, TrustReport, UserProfile, Visibility } from "@/types";

export const visibilityLabel: Record<Visibility, string> = {
  private: "私密",
  unlisted: "仅链接",
  public: "公开",
};

export const assetKindLabel: Record<AssetKind, string> = {
  skill: "Skill",
  article: "经验文章",
  template: "模板",
};

export function formatDate(value: string, withYear = false) {
  return new Intl.DateTimeFormat("zh-CN", {
    ...(withYear ? { year: "numeric" } : {}),
    month: "short",
    day: "numeric",
  }).format(new Date(value));
}

export function formatFileSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${Math.round(size / 1024)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

export function getUser(users: UserProfile[], id: string) {
  return users.find((user) => user.id === id);
}

export function taskMatchScore(asset: GeneratedAsset, task: string, reports: TrustReport[]) {
  const normalized = task.trim().toLowerCase();
  const terms = normalized
    .replace(/[，。！？、,.!?]/g, " ")
    .split(/\s+/)
    .filter((term) => term.length >= 2);
  const haystack = [asset.title, asset.summary, asset.task, ...asset.categories].join(" ").toLowerCase();
  const keywordScore = terms.reduce((score, term) => score + (haystack.includes(term) ? 14 : 0), 0);
  const report = reports.find((item) => item.assetId === asset.id);
  const trustScore = report ? report.score * 0.55 : 0;
  const recency = asset.publishedAt ? Math.max(0, 10 - (Date.now() - new Date(asset.publishedAt).getTime()) / 86400000) : 0;
  return keywordScore + trustScore + recency;
}

export function recommendationReason(asset: GeneratedAsset, task: string, reports: TrustReport[]) {
  const report = reports.find((item) => item.assetId === asset.id);
  const taskHit = asset.categories.find((category) => task.includes(category));
  if (report && taskHit) return `与“${taskHit}”任务相符，且可信评测 ${report.score} 分`;
  if (report) return `已完成可信评测，${report.dimensions.clarity >= 85 ? "边界与输出清晰" : "证据可追溯"}`;
  return "与你正在浏览的校园项目场景相关";
}
