import type { UserProfile } from "@/types";
import type { CSSProperties } from "react";

export function Avatar({ user, size = "md" }: { user: UserProfile; size?: "sm" | "md" | "lg" | "xl" }) {
  return (
    <span
      className={`avatar avatar--${size}`}
      style={{ "--avatar-accent": user.accent } as CSSProperties}
      aria-label={user.name}
      title={user.name}
    >
      {user.initials}
    </span>
  );
}
