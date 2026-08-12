import { seedState } from "@/data/seed";
import type {
  CommentItem,
  DemoState,
  GeneratedAsset,
  GrowthRecord,
  PersonaConfig,
  TrustReport,
} from "@/types";
import {
  createContext,
  type Dispatch,
  type ReactNode,
  useContext,
  useEffect,
  useMemo,
  useReducer,
} from "react";

const STORAGE_KEY = "wowskillsland-demo-v1";
const LEGACY_STORAGE_KEYS = ["skillx-demo-v1"];

type ContentTarget = { type: "asset" | "record"; id: string };

export type DemoAction =
  | { type: "SET_LOGIN"; value: boolean }
  | { type: "ADD_RECORD"; record: GrowthRecord }
  | { type: "UPDATE_RECORD"; id: string; patch: Partial<GrowthRecord> }
  | { type: "ADD_ASSET"; asset: GeneratedAsset }
  | { type: "UPDATE_ASSET"; id: string; patch: Partial<GeneratedAsset> }
  | { type: "PUBLISH_ASSET"; id: string }
  | { type: "TOGGLE_LIKE"; target: ContentTarget }
  | { type: "TOGGLE_BOOKMARK"; target: ContentTarget }
  | { type: "TOGGLE_HELPFUL"; id: string }
  | { type: "ADD_COMMENT"; target: ContentTarget; comment: CommentItem }
  | { type: "TOGGLE_FOLLOW"; userId: string }
  | { type: "MARK_NOTIFICATIONS_READ" }
  | { type: "UPSERT_PERSONA"; persona: PersonaConfig }
  | { type: "UPDATE_PERSONA"; authorId: string; patch: Partial<PersonaConfig> }
  | { type: "ADD_REPORT"; report: TrustReport }
  | { type: "RESET_DEMO" };

type DemoStoreValue = {
  state: DemoState;
  dispatch: Dispatch<DemoAction>;
};

const DemoStoreContext = createContext<DemoStoreValue | null>(null);

export function makeId(prefix: string) {
  const suffix =
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? crypto.randomUUID().slice(0, 8)
      : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `${prefix}-${suffix}`;
}

function addNotice(state: DemoState, message: string, href?: string): DemoState {
  return {
    ...state,
    social: {
      ...state.social,
      notifications: [
        {
          id: makeId("notice"),
          message,
          href,
          createdAt: new Date().toISOString(),
          read: false,
        },
        ...state.social.notifications,
      ].slice(0, 30),
    },
  };
}

function updateTarget(
  state: DemoState,
  target: ContentTarget,
  updater: (item: GrowthRecord | GeneratedAsset) => GrowthRecord | GeneratedAsset,
) {
  if (target.type === "record") {
    return {
      ...state,
      records: state.records.map((item) =>
        item.id === target.id ? (updater(item) as GrowthRecord) : item,
      ),
    };
  }
  return {
    ...state,
    assets: state.assets.map((item) =>
      item.id === target.id ? (updater(item) as GeneratedAsset) : item,
    ),
  };
}

function reducer(state: DemoState, action: DemoAction): DemoState {
  switch (action.type) {
    case "SET_LOGIN":
      return addNotice(
        { ...state, isLoggedIn: action.value },
        action.value ? "已登录 WowSkillsLand 演示账号" : "已退出登录，公开内容仍可浏览",
      );
    case "ADD_RECORD":
      return addNotice(
        { ...state, records: [action.record, ...state.records] },
        `成长记录“${action.record.title}”已保存`,
        "/creator",
      );
    case "UPDATE_RECORD": {
      const previous = state.records.find((item) => item.id === action.id);
      let next: DemoState = {
        ...state,
        records: state.records.map((item) =>
          item.id === action.id ? { ...item, ...action.patch } : item,
        ),
      };

      if (previous?.allowPersona && action.patch.allowPersona === false) {
        next = {
          ...next,
          personas: next.personas.map((persona) => {
            if (!persona.sourceRecordIds.includes(action.id)) return persona;
            return {
              ...persona,
              sourceRecordIds: persona.sourceRecordIds.filter((id) => id !== action.id),
              generationStatus: "idle",
              publicEntry: false,
              needsRebuild: true,
              confirmedAt: undefined,
            };
          }),
        };
        return addNotice(
          next,
          "已撤销一条人格资料授权，公开对话入口已自动关闭",
          "/creator",
        );
      }
      return next;
    }
    case "ADD_ASSET":
      return addNotice(
        { ...state, assets: [action.asset, ...state.assets] },
        `已生成${kindLabel(action.asset.kind)}草稿`,
        "/creator",
      );
    case "UPDATE_ASSET":
      return {
        ...state,
        assets: state.assets.map((item) =>
          item.id === action.id ? { ...item, ...action.patch, updatedAt: new Date().toISOString() } : item,
        ),
      };
    case "PUBLISH_ASSET": {
      const asset = state.assets.find((item) => item.id === action.id);
      if (!asset) return state;
      return addNotice(
        {
          ...state,
          assets: state.assets.map((item) =>
            item.id === action.id
              ? {
                  ...item,
                  generationStatus: "ready",
                  publishedAt: new Date().toISOString(),
                  updatedAt: new Date().toISOString(),
                }
              : item,
          ),
        },
        `“${asset.title}”已发布，并同步到可见页面`,
        asset.visibility === "public" ? "/explore" : "/creator",
      );
    }
    case "TOGGLE_LIKE": {
      const item =
        action.target.type === "record"
          ? state.records.find((record) => record.id === action.target.id)
          : state.assets.find((asset) => asset.id === action.target.id);
      if (!item) return state;
      const wasLiked = item.likedByMe;
      const next = updateTarget(state, action.target, (target) => ({
        ...target,
        likedByMe: !target.likedByMe,
        likes: Math.max(0, target.likes + (target.likedByMe ? -1 : 1)),
      }));
      return wasLiked ? next : addNotice(next, "已点赞，互动状态已在页面间同步");
    }
    case "TOGGLE_BOOKMARK": {
      const next = updateTarget(state, action.target, (target) => ({
        ...target,
        bookmarkedByMe: !target.bookmarkedByMe,
      }));
      return addNotice(next, "收藏状态已更新", "/explore");
    }
    case "TOGGLE_HELPFUL": {
      const asset = state.assets.find((item) => item.id === action.id);
      if (!asset) return state;
      const next = {
        ...state,
        assets: state.assets.map((item) =>
          item.id === action.id
            ? {
                ...item,
                helpfulByMe: !item.helpfulByMe,
                helpful: Math.max(0, item.helpful + (item.helpfulByMe ? -1 : 1)),
              }
            : item,
        ),
      };
      return asset.helpfulByMe ? next : addNotice(next, "感谢反馈，这会帮助改进任务匹配");
    }
    case "ADD_COMMENT": {
      const next = updateTarget(state, action.target, (target) => ({
        ...target,
        comments: [...target.comments, action.comment],
      }));
      return addNotice(next, "评论已发布", "/explore");
    }
    case "TOGGLE_FOLLOW": {
      if (action.userId === state.currentUserId) return state;
      const isFollowing = state.social.followingUserIds.includes(action.userId);
      const person = state.users.find((user) => user.id === action.userId);
      const next = {
        ...state,
        users: state.users.map((user) =>
          user.id === action.userId
            ? { ...user, followers: Math.max(0, user.followers + (isFollowing ? -1 : 1)) }
            : user,
        ),
        social: {
          ...state.social,
          followingUserIds: isFollowing
            ? state.social.followingUserIds.filter((id) => id !== action.userId)
            : [...state.social.followingUserIds, action.userId],
        },
      };
      return addNotice(next, `${isFollowing ? "已取消关注" : "已关注"}${person ? ` ${person.name}` : ""}`);
    }
    case "MARK_NOTIFICATIONS_READ":
      return {
        ...state,
        social: {
          ...state.social,
          notifications: state.social.notifications.map((item) => ({ ...item, read: true })),
        },
      };
    case "UPSERT_PERSONA": {
      const exists = state.personas.some((item) => item.authorId === action.persona.authorId);
      const personas = exists
        ? state.personas.map((item) =>
            item.authorId === action.persona.authorId ? action.persona : item,
          )
        : [...state.personas, action.persona];
      return addNotice({ ...state, personas }, "赛博人格草稿已生成，请逐项确认", "/creator");
    }
    case "UPDATE_PERSONA": {
      const next = {
        ...state,
        personas: state.personas.map((item) =>
          item.authorId === action.authorId ? { ...item, ...action.patch } : item,
        ),
      };
      if (action.patch.publicEntry === true) {
        return addNotice(next, "赛博人格公开入口已开启", `/u/${state.users.find((u) => u.id === action.authorId)?.handle ?? "linxia"}`);
      }
      if (action.patch.publicEntry === false) {
        return addNotice(next, "赛博人格公开入口已关闭");
      }
      return next;
    }
    case "ADD_REPORT":
      return addNotice(
        { ...state, reports: [action.report, ...state.reports] },
        `“${action.report.title}”已完成模拟评测`,
        "/trust",
      );
    case "RESET_DEMO":
      return seedState;
    default:
      return state;
  }
}

function kindLabel(kind: GeneratedAsset["kind"]) {
  return kind === "skill" ? "Skill" : kind === "article" ? "经验文章" : "模板";
}

function loadInitialState(): DemoState {
  if (typeof window === "undefined") return seedState;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
      ?? LEGACY_STORAGE_KEYS.map((key) => window.localStorage.getItem(key)).find(Boolean);
    if (!raw) return seedState;
    const parsed = JSON.parse(raw) as DemoState;
    if (parsed.version !== 1) return seedState;
    return parsed;
  } catch {
    return seedState;
  }
}

function stateForStorage(state: DemoState): DemoState {
  return {
    ...state,
    records: state.records.map((record) => ({
      ...record,
      attachments: record.attachments.map(({ sessionPreviewUrl: _preview, ...metadata }) => metadata),
    })),
  };
}

export function DemoStoreProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, undefined, loadInitialState);

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(stateForStorage(state)));
    } catch {
      // The prototype remains usable when localStorage is disabled or full.
    }
  }, [state]);

  const value = useMemo(() => ({ state, dispatch }), [state]);
  return <DemoStoreContext.Provider value={value}>{children}</DemoStoreContext.Provider>;
}

export function useDemoStore() {
  const value = useContext(DemoStoreContext);
  if (!value) throw new Error("useDemoStore must be used inside DemoStoreProvider");
  return value;
}
