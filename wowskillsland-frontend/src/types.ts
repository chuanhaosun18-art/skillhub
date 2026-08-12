export type Visibility = "private" | "unlisted" | "public";

export type GenerationStatus =
  | "idle"
  | "analyzing"
  | "draft"
  | "ready"
  | "error";

export type AssetKind = "skill" | "article" | "template";

export interface UserProfile {
  id: string;
  handle: string;
  name: string;
  school: string;
  major: string;
  bio: string;
  initials: string;
  accent: string;
  followers: number;
  following: number;
  verified?: boolean;
}

export interface AttachmentMeta {
  id: string;
  name: string;
  type: string;
  size: number;
  sessionPreviewUrl?: string;
}

export interface CommentItem {
  id: string;
  authorId: string;
  text: string;
  createdAt: string;
}

export interface GrowthRecord {
  id: string;
  authorId: string;
  title: string;
  body: string;
  createdAt: string;
  attachments: AttachmentMeta[];
  links: string[];
  tags: string[];
  visibility: Visibility;
  featured: boolean;
  allowAsset: boolean;
  allowPersona: boolean;
  likes: number;
  likedByMe: boolean;
  bookmarkedByMe: boolean;
  comments: CommentItem[];
}

export interface GeneratedAsset {
  id: string;
  authorId: string;
  kind: AssetKind;
  title: string;
  summary: string;
  task: string;
  output: string;
  boundary: string;
  evidence: string;
  sourceRecordIds: string[];
  generationStatus: GenerationStatus;
  visibility: Visibility;
  publishedAt?: string;
  updatedAt: string;
  categories: string[];
  likes: number;
  likedByMe: boolean;
  bookmarkedByMe: boolean;
  helpful: number;
  helpfulByMe: boolean;
  comments: CommentItem[];
  trustReportId?: string;
}

export interface PersonaConfig {
  authorId: string;
  sourceRecordIds: string[];
  generationStatus: GenerationStatus;
  name: string;
  summary: string;
  tone: string;
  traits: string[];
  knowledgeScopes: string[];
  blockedTopics: string[];
  publicEntry: boolean;
  needsRebuild: boolean;
  confirmedAt?: string;
}

export interface TrustDimensions {
  validity: number;
  reliability: number;
  clarity: number;
  traceability: number;
  acceptance: number;
}

export interface TrustReport {
  id: string;
  assetId?: string;
  title: string;
  authorId: string;
  status: "valid" | "watch" | "limited";
  score: number;
  dimensions: TrustDimensions;
  evidence: string[];
  risks: string[];
  boundary: string;
  permissions: string;
  maintenance: string;
  failureCases: string[];
  createdAt: string;
}

export interface NotificationItem {
  id: string;
  message: string;
  createdAt: string;
  read: boolean;
  href?: string;
}

export interface SocialState {
  followingUserIds: string[];
  notifications: NotificationItem[];
}

export interface DemoState {
  version: 1;
  currentUserId: string;
  isLoggedIn: boolean;
  users: UserProfile[];
  records: GrowthRecord[];
  assets: GeneratedAsset[];
  personas: PersonaConfig[];
  reports: TrustReport[];
  social: SocialState;
}
