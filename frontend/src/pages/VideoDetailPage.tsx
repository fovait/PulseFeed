import { ArrowLeft, BarChart3, Home } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { CommentsDrawer } from "../components/CommentsDrawer";
import { ReelItem } from "../components/ReelItem";
import { ReportDialog } from "../components/ReportDialog";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useEventTracker } from "../hooks/useEventTracker";
import { useToast } from "../hooks/useToast";
import type { Comment, FeedVideo, VideoMetrics } from "../types/api";
import { normalizeVideo, videoAuthor } from "../utils/video";

type ReportTarget = { type: "video" | "comment"; id: number; title?: string };

export function VideoDetailPage() {
  const { id } = useParams<{ id: string }>();
  const videoID = Number(id);
  const navigate = useNavigate();
  const { session, requireAuth } = useAuth();
  const { pushToast } = useToast();
  const track = useEventTracker(session);

  const [video, setVideo] = useState<FeedVideo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [commentsOpen, setCommentsOpen] = useState(false);
  const [reportTarget, setReportTarget] = useState<ReportTarget | null>(null);
  const [authorFollowed, setAuthorFollowed] = useState(false);
  const [metrics, setMetrics] = useState<VideoMetrics | null>(null);
  const [metricsOpen, setMetricsOpen] = useState(false);
  const [metricsLoading, setMetricsLoading] = useState(false);

  const isAuthor = Boolean(video && session?.account_id && videoAuthor(video).id === session.account_id);

  async function openMetrics() {
    if (!video) return;
    setMetricsOpen(true);
    if (metrics) return;
    setMetricsLoading(true);
    try {
      const resp = await pulsefeedApi.getVideoMetrics(video.id);
      setMetrics(resp.metrics);
    } catch (err) {
      pushToast(err instanceof Error ? err.message : "指标加载失败", "error");
    } finally {
      setMetricsLoading(false);
    }
  }

  const refresh = useCallback(async () => {
    if (!videoID) {
      setError("无效的视频 ID");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const raw = await pulsefeedApi.getVideoDetail(videoID);
      const detail = normalizeVideo(raw);
      const author = videoAuthor(detail);
      if (session?.token) {
        const [liked, followed] = await Promise.all([
          pulsefeedApi.isLiked(videoID).catch(() => ({ is_liked: Boolean(detail.is_liked) })),
          author.id
            ? pulsefeedApi.isFollowing(author.id).catch(() => ({ is_followed: false }))
            : Promise.resolve({ is_followed: false }),
        ]);
        detail.is_liked = liked.is_liked;
        detail.is_following = followed.is_followed;
        setAuthorFollowed(followed.is_followed);
      }
      setVideo(detail);
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载视频失败");
    } finally {
      setLoading(false);
    }
  }, [session?.token, videoID]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  async function toggleLike() {
    if (!video) return;
    if (!requireAuth("登录后才能点赞")) return;
    const previous = Boolean(video.is_liked);
    setVideo((cur) => (cur ? { ...cur, is_liked: !previous, likes_count: Math.max(0, Number(cur.likes_count || 0) + (previous ? -1 : 1)) } : cur));
    try {
      if (previous) await pulsefeedApi.unlikeVideo(video.id);
      else await pulsefeedApi.likeVideo(video.id);
    } catch (err) {
      setVideo((cur) => (cur ? { ...cur, is_liked: previous, likes_count: Math.max(0, Number(cur.likes_count || 0) + (previous ? 1 : -1)) } : cur));
      pushToast(err instanceof Error ? err.message : "点赞失败", "error");
    }
  }

  async function toggleFollow(authorID: number) {
    if (!authorID || !requireAuth("登录后才能关注作者")) return;
    const wasFollowing = authorFollowed;
    setAuthorFollowed(!wasFollowing);
    try {
      if (wasFollowing) {
        await pulsefeedApi.unfollow(authorID);
        pushToast("已取消关注");
      } else {
        await pulsefeedApi.follow(authorID);
        pushToast("已关注", "success");
      }
    } catch (err) {
      setAuthorFollowed(wasFollowing);
      pushToast(err instanceof Error ? err.message : "关注失败", "error");
    }
  }

  function openMessage(authorID: number) {
    if (!authorID || !requireAuth("登录后才能发送私信")) return;
    navigate(`/messages?peer_id=${authorID}`);
  }

  async function share() {
    if (!video) return;
    await track(video.id, "share");
    const url = `${window.location.origin}/video/${video.id}`;
    try {
      await navigator.clipboard.writeText(url);
      pushToast("分享链接已复制", "success");
    } catch {
      pushToast("当前浏览器不支持自动复制");
    }
  }

  function handleBack() {
    if (window.history.length > 1) {
      navigate(-1);
    } else {
      navigate("/feed/recommend");
    }
  }

  return (
    <main className="relative h-[100svh] overflow-hidden bg-black">
      <div className="absolute left-3 top-3 z-50 flex items-center gap-2 md:left-28 md:top-4">
        <button
          onClick={handleBack}
          className="flex items-center gap-1.5 rounded-lg bg-black/60 px-3 py-2 text-sm font-semibold text-white backdrop-blur hover:bg-black/80"
          title="返回"
        >
          <ArrowLeft className="h-4 w-4" />
          返回
        </button>
        <button
          onClick={() => navigate("/feed/recommend")}
          className="flex items-center gap-1.5 rounded-lg bg-black/60 px-3 py-2 text-sm font-semibold text-white backdrop-blur hover:bg-black/80"
          title="返回主页"
        >
          <Home className="h-4 w-4" />
          主页
        </button>
        {isAuthor && (
          <button
            onClick={openMetrics}
            className="flex items-center gap-1.5 rounded-lg bg-pulse-cyan/20 px-3 py-2 text-sm font-semibold text-pulse-cyan backdrop-blur hover:bg-pulse-cyan/30"
            title="查看数据"
          >
            <BarChart3 className="h-4 w-4" />
            数据
          </button>
        )}
      </div>

      {metricsOpen && (
        <div className="absolute inset-0 z-40 grid place-items-center bg-black/60 backdrop-blur" onClick={() => setMetricsOpen(false)}>
          <div className="glass-panel w-[min(420px,90vw)] rounded-lg p-5" onClick={(e) => e.stopPropagation()}>
            <div className="mb-4 flex items-center justify-between">
              <h3 className="text-lg font-black">视频数据</h3>
              <button className="text-xs text-white/52 hover:text-white" onClick={() => setMetricsOpen(false)}>关闭</button>
            </div>
            {metricsLoading ? (
              <p className="text-sm text-white/52">加载中...</p>
            ) : !metrics ? (
              <p className="text-sm text-white/52">暂无数据</p>
            ) : (
              <div className="grid grid-cols-2 gap-3 text-center">
                <MetricBox label="曝光" value={metrics.impression_count} />
                <MetricBox label="播放" value={metrics.view_count} />
                <MetricBox label="完播" value={metrics.play_complete_count} />
                <MetricBox label="分享" value={metrics.share_count} />
              </div>
            )}
          </div>
        </div>
      )}

      <div className="h-full overflow-hidden">
        {loading ? (
          <div className="grid h-full place-items-center text-sm text-white/58">加载中...</div>
        ) : error || !video ? (
          <div className="grid h-full place-items-center px-8 text-center">
            <div>
              <p className="text-xl font-black">{error || "视频不存在"}</p>
              <button className="primary-button mt-5" onClick={() => navigate(-1)}>
                返回
              </button>
            </div>
          </div>
        ) : (
          <ReelItem
            key={video.id}
            video={video}
            onLike={toggleLike}
            onComments={() => setCommentsOpen(true)}
            onShare={share}
            onReport={() => setReportTarget({ type: "video", id: video.id, title: video.title })}
            onFollow={(authorID) => toggleFollow(authorID)}
            onMessage={(authorID) => openMessage(authorID)}
            following={authorFollowed}
            onVisible={() => track(video.id, "impression")}
            onPlayStart={() => track(video.id, "view")}
            onComplete={() => track(video.id, "play_complete")}
          />
        )}
      </div>

      <CommentsDrawer
        video={commentsOpen ? video : null}
        onClose={() => setCommentsOpen(false)}
        onCountChange={(_, count) =>
          setVideo((cur) => (cur ? { ...cur, comments_count: Math.max(0, count) } : cur))
        }
        onReportComment={(comment: Comment) =>
          setReportTarget({ type: "comment", id: comment.id, title: comment.content })
        }
      />
      <ReportDialog target={reportTarget} onClose={() => setReportTarget(null)} />
    </main>
  );
}

function MetricBox({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-lg bg-white/[0.06] p-3">
      <p className="text-2xl font-black">{value ?? 0}</p>
      <p className="mt-1 text-xs text-white/52">{label}</p>
    </div>
  );
}
