import { useDemoStore } from "@/store/DemoStore";
import type { GeneratedAsset, GrowthRecord } from "@/types";
import { Bookmark, Heart, MessageCircle, ThumbsUp } from "lucide-react";

type Props = {
  item: GeneratedAsset | GrowthRecord;
  targetType: "asset" | "record";
  onComment?: () => void;
  showHelpful?: boolean;
  compact?: boolean;
  onNeedLogin?: () => void;
};

export function SocialActions({ item, targetType, onComment, showHelpful, compact, onNeedLogin }: Props) {
  const { state, dispatch } = useDemoStore();
  const act = (callback: () => void) => {
    if (!state.isLoggedIn) {
      onNeedLogin?.();
      return;
    }
    callback();
  };
  const asset = targetType === "asset" ? (item as GeneratedAsset) : null;

  return (
    <div className={`social-actions${compact ? " social-actions--compact" : ""}`}>
      <button
        type="button"
        className={item.likedByMe ? "is-active" : ""}
        aria-pressed={item.likedByMe}
        onClick={() => act(() => dispatch({ type: "TOGGLE_LIKE", target: { type: targetType, id: item.id } }))}
      >
        <Heart size={17} fill={item.likedByMe ? "currentColor" : "none"} />
        <span>{item.likes}</span>
      </button>
      <button type="button" onClick={() => act(() => onComment?.())}>
        <MessageCircle size={17} />
        <span>{item.comments.length}</span>
      </button>
      <button
        type="button"
        className={item.bookmarkedByMe ? "is-active" : ""}
        aria-pressed={item.bookmarkedByMe}
        onClick={() => act(() => dispatch({ type: "TOGGLE_BOOKMARK", target: { type: targetType, id: item.id } }))}
      >
        <Bookmark size={17} fill={item.bookmarkedByMe ? "currentColor" : "none"} />
        <span>{compact ? "" : "收藏"}</span>
      </button>
      {showHelpful && asset && (
        <button
          type="button"
          className={asset.helpfulByMe ? "is-active" : ""}
          aria-pressed={asset.helpfulByMe}
          onClick={() => act(() => dispatch({ type: "TOGGLE_HELPFUL", id: asset.id }))}
        >
          <ThumbsUp size={17} fill={asset.helpfulByMe ? "currentColor" : "none"} />
          <span>{asset.helpful} 人觉得有帮助</span>
        </button>
      )}
    </div>
  );
}
