import { Flag, Heart, MessageCircle, Share2 } from "lucide-react";
import type { ReactNode } from "react";

export function ActionRail({
  liked,
  likesCount,
  commentsCount,
  onLike,
  onComments,
  onShare,
  onReport,
  className = "absolute bottom-28 right-3 z-20 flex flex-col items-center gap-4",
}: {
  liked: boolean;
  likesCount: number;
  commentsCount: number;
  onLike: () => void;
  onComments: () => void;
  onShare: () => void;
  onReport: () => void;
  className?: string;
}) {
  return (
    <div className={className}>
      <ActionButton label={String(likesCount)} active={liked} onClick={onLike}>
        <Heart className={["h-7 w-7", liked ? "fill-pulse-red text-pulse-red" : ""].join(" ")} />
      </ActionButton>
      <ActionButton label={String(commentsCount)} onClick={onComments}>
        <MessageCircle className="h-7 w-7" />
      </ActionButton>
      <ActionButton label="分享" onClick={onShare}>
        <Share2 className="h-7 w-7" />
      </ActionButton>
      <ActionButton label="举报" onClick={onReport}>
        <Flag className="h-7 w-7" />
      </ActionButton>
    </div>
  );
}

function ActionButton({
  label,
  active = false,
  onClick,
  children,
}: {
  label: string;
  active?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button type="button" className="flex flex-col items-center gap-1 text-white" onClick={onClick}>
      <span
        className={[
          "grid h-12 w-12 place-items-center rounded-full bg-black/42 shadow-lg backdrop-blur-md transition",
          active ? "ring-2 ring-pulse-red/60" : "hover:bg-white/12",
        ].join(" ")}
      >
        {children}
      </span>
      <span className="max-w-14 truncate text-center text-[0.68rem] font-bold text-white/86">{label}</span>
    </button>
  );
}
