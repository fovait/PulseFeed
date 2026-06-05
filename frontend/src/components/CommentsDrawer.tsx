import { Flag, SendHorizonal, Trash2, X } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";
import type { Comment, FeedVideo } from "../types/api";
import { formatRelativeTime } from "../utils/time";

export function CommentsDrawer({
  video,
  onClose,
  onCountChange,
  onReportComment,
}: {
  video: FeedVideo | null;
  onClose: () => void;
  onCountChange: (videoID: number, count: number) => void;
  onReportComment: (comment: Comment) => void;
}) {
  const [comments, setComments] = useState<Comment[]>([]);
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const { session, requireAuth } = useAuth();
  const { pushToast } = useToast();

  useEffect(() => {
    if (!video) return undefined;
    let cancelled = false;
    setLoading(true);
    pulsefeedApi
      .listComments(video.id)
      .then((items) => {
        if (!cancelled) {
          const next = items || [];
          setComments(next);
          onCountChange(video.id, next.length);
        }
      })
      .catch((error) => {
        if (!cancelled) pushToast(error instanceof Error ? error.message : "评论加载失败", "error");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [onCountChange, pushToast, video?.id]);

  if (!video) return null;

  async function refresh() {
    if (!video) return;
    const items = await pulsefeedApi.listComments(video.id);
    const next = items || [];
    setComments(next);
    onCountChange(video.id, next.length);
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!requireAuth("登录后才能发表评论")) return;
    if (!video) return;
    const text = content.trim();
    if (!text) return;
    setSubmitting(true);
    try {
      await pulsefeedApi.publishComment(video.id, text);
      setContent("");
      await refresh();
      pushToast("评论已发布", "success");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "评论发布失败", "error");
    } finally {
      setSubmitting(false);
    }
  }

  async function remove(commentID: number) {
    if (!requireAuth("登录后才能删除评论")) return;
    if (!video) return;
    try {
      await pulsefeedApi.deleteComment(commentID);
      setComments((items) => {
        const next = items.filter((item) => item.id !== commentID);
        onCountChange(video.id, next.length);
        return next;
      });
      pushToast("评论已删除", "success");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "删除失败", "error");
    }
  }

  return (
    <aside className="fixed inset-x-0 bottom-0 z-[60] mx-auto max-h-[76svh] w-full max-w-[430px] overflow-hidden rounded-t-lg border border-white/12 bg-zinc-950/90 shadow-glow backdrop-blur-2xl">
      <div className="flex items-center justify-between border-b border-white/10 px-4 py-3">
        <div>
          <h2 className="text-base font-black">评论</h2>
          <p className="max-w-[300px] truncate text-xs text-white/52">{video.title}</p>
        </div>
        <button type="button" className="rounded-lg p-2 text-white/70 hover:bg-white/10" onClick={onClose}>
          <X className="h-5 w-5" />
        </button>
      </div>

      <div className="max-h-[46svh] space-y-3 overflow-y-auto px-4 py-4">
        {loading ? <p className="text-sm text-white/58">正在加载评论...</p> : null}
        {!loading && comments.length === 0 ? <p className="text-sm text-white/58">还没有评论</p> : null}
        {comments.map((comment) => (
          <div key={comment.id} className="rounded-lg bg-white/[0.06] p-3">
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-sm font-black">@{comment.username || `用户 ${comment.author_id}`}</p>
                <p className="mt-1 text-sm leading-6 text-white/82">{comment.content}</p>
                <p className="mt-1 text-xs text-white/42">{formatRelativeTime(comment.created_at)}</p>
              </div>
              <div className="flex shrink-0 gap-1">
                <button
                  type="button"
                  className="rounded-lg p-2 text-white/52 hover:bg-white/10 hover:text-white"
                  onClick={() => onReportComment(comment)}
                >
                  <Flag className="h-4 w-4" />
                </button>
                {session?.account_id === comment.author_id ? (
                  <button
                    type="button"
                    className="rounded-lg p-2 text-white/52 hover:bg-white/10 hover:text-pulse-red"
                    onClick={() => remove(comment.id)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                ) : null}
              </div>
            </div>
          </div>
        ))}
      </div>

      <form className="border-t border-white/10 p-3" onSubmit={submit}>
        <div className="flex items-end gap-2">
          <textarea
            className="control-field min-h-12 resize-none text-sm"
            value={content}
            onChange={(event) => setContent(event.target.value)}
            maxLength={500}
            placeholder={session?.token ? "写下你的评论" : "登录后发表评论"}
          />
          <button
            className="grid h-12 w-12 shrink-0 place-items-center rounded-lg bg-pulse-cyan text-black disabled:opacity-45"
            disabled={submitting || !content.trim()}
          >
            <SendHorizonal className="h-5 w-5" />
          </button>
        </div>
      </form>
    </aside>
  );
}
