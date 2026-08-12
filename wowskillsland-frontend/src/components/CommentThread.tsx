import { Avatar } from "@/components/Avatar";
import { makeId, useDemoStore } from "@/store/DemoStore";
import type { CommentItem } from "@/types";
import { formatDate, getUser } from "@/utils";
import { Send } from "lucide-react";
import { type FormEvent, useState } from "react";

export function CommentThread({
  comments,
  target,
  onNeedLogin,
}: {
  comments: CommentItem[];
  target: { type: "asset" | "record"; id: string };
  onNeedLogin?: () => void;
}) {
  const { state, dispatch } = useDemoStore();
  const [text, setText] = useState("");
  const currentUser = getUser(state.users, state.currentUserId)!;

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!state.isLoggedIn) {
      onNeedLogin?.();
      return;
    }
    if (!text.trim()) return;
    dispatch({
      type: "ADD_COMMENT",
      target,
      comment: {
        id: makeId("comment"),
        authorId: state.currentUserId,
        text: text.trim(),
        createdAt: new Date().toISOString(),
      },
    });
    setText("");
  };

  return (
    <div className="comment-thread">
      {comments.map((comment) => {
        const author = getUser(state.users, comment.authorId) ?? currentUser;
        return (
          <div className="comment" key={comment.id}>
            <Avatar user={author} size="sm" />
            <div><strong>{author.name}</strong><p>{comment.text}</p><small>{formatDate(comment.createdAt)}</small></div>
          </div>
        );
      })}
      <form className="comment-form" onSubmit={submit}>
        {state.isLoggedIn && <Avatar user={currentUser} size="sm" />}
        <input value={text} onChange={(event) => setText(event.target.value)} placeholder={state.isLoggedIn ? "写下具体的反馈…" : "登录后参与讨论"} aria-label="评论内容" />
        <button type="submit" aria-label="发送评论"><Send size={16} /></button>
      </form>
    </div>
  );
}
