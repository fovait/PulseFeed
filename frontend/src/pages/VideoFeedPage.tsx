import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { CommentsDrawer } from "../components/CommentsDrawer";
import { ReelItem } from "../components/ReelItem";
import { ReportDialog, type ReportTarget } from "../components/ReportDialog";
import { TopStatusBar } from "../components/TopStatusBar";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useEventTracker } from "../hooks/useEventTracker";
import { useToast } from "../hooks/useToast";
import type {
  Comment,
  FeedMode,
  FeedVideo,
  LikesCursor,
  PopularityCursor,
  RankedVideo,
  TimeFeedResponse,
} from "../types/api";
import { normalizeVideo, videoAuthor } from "../utils/video";

const modes: FeedMode[] = ["recommend", "latest", "following", "popularity", "likes"];
const PAGE_SIZE = 8;

type FeedCursorState = {
  beforeTime: number;
  popularityCursor?: PopularityCursor;
  likesCursor?: LikesCursor;
  recommendCursor: string;
};

const initialCursor: FeedCursorState = {
  beforeTime: 0,
  recommendCursor: "",
};

export function VideoFeedPage() {
  const params = useParams();
  const [searchParams] = useSearchParams();
  const mode = modes.includes(params.mode as FeedMode) ? (params.mode as FeedMode) : "recommend";
  const sharedVideoID = Number(searchParams.get("video_id") || 0);
  const [videos, setVideos] = useState<FeedVideo[]>([]);
  const [cursor, setCursor] = useState<FeedCursorState>(initialCursor);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [followedAuthors, setFollowedAuthors] = useState<Set<number>>(() => new Set());
  const [selectedVideo, setSelectedVideo] = useState<FeedVideo | null>(null);
  const [reportTarget, setReportTarget] = useState<ReportTarget | null>(null);
  const loadingRef = useRef(false);
  const cursorRef = useRef<FeedCursorState>(initialCursor);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const { session, requireAuth } = useAuth();
  const { pushToast } = useToast();
  const navigate = useNavigate();
  const track = useEventTracker(session);

  const label = useMemo(() => {
    if (mode === "recommend") return "推荐";
    if (mode === "latest") return "最新";
    if (mode === "following") return "关注";
    if (mode === "likes") return "点赞榜";
    return "热榜";
  }, [mode]);

  const updateVideo = useCallback((videoID: number, updater: (video: FeedVideo) => FeedVideo) => {
    setVideos((items) => items.map((item) => (item.id === videoID ? updater(item) : item)));
    setSelectedVideo((item) => (item?.id === videoID ? updater(item) : item));
  }, []);

  const hydrateInteractiveState = useCallback(
    async (items: FeedVideo[]) => {
      if (!session?.token || items.length === 0) return items;
      return Promise.all(
        items.map(async (video) => {
          const author = videoAuthor(video);
          const [liked, followed] = await Promise.all([
            pulsefeedApi.isLiked(video.id).catch(() => ({ is_liked: Boolean(video.is_liked) })),
            author.id ? pulsefeedApi.isFollowing(author.id).catch(() => ({ is_followed: Boolean(video.is_following) })) : Promise.resolve({ is_followed: false }),
          ]);
          return {
            ...video,
            is_liked: liked.is_liked,
            is_following: followed.is_followed,
          };
        }),
      );
    },
    [session?.token],
  );

  const hydrateRecommendations = useCallback(
    async (ranked: RankedVideo[]) => {
      if (ranked.length === 0) return [];
      const byID = new Map(ranked.map((item) => [item.video_id, item]));
      const details = await pulsefeedApi.listVideoDetails(ranked.map((item) => item.video_id));
      const detailByID = new Map((details.videos || []).map((video) => [video.id, video]));
      const videos = ranked.map((item) => {
        const detail = detailByID.get(item.video_id);
        if (!detail) {
          return normalizeVideo({
            id: item.video_id,
            title: `推荐视频 #${item.video_id}`,
            description: "这条推荐视频暂时缺少完整详情。",
            play_url: "",
            cover_url: "",
            likes_count: 0,
            comments_count: 0,
            recommendation: item,
          });
        }
        return normalizeVideo({
          ...detail,
          recommendation: byID.get(detail.id) || item,
        });
      });
      return hydrateInteractiveState(videos);
    },
    [hydrateInteractiveState],
  );

  const prioritizeSharedVideo = useCallback(
    async (items: FeedVideo[]) => {
      if (!sharedVideoID) return items;
      const existing = items.find((item) => item.id === sharedVideoID);
      if (existing) {
        return [existing, ...items.filter((item) => item.id !== sharedVideoID)];
      }
      try {
        const detail = normalizeVideo(await pulsefeedApi.getVideoDetail(sharedVideoID));
        const [hydrated] = await hydrateInteractiveState([detail]);
        return [hydrated, ...items.filter((item) => item.id !== sharedVideoID)];
      } catch {
        pushToast("分享视频加载失败", "error");
        return items;
      }
    },
    [hydrateInteractiveState, pushToast, sharedVideoID],
  );

  const fetchFeed = useCallback(
    async (append: boolean) => {
      if (loadingRef.current) return;
      if (mode === "following" && !requireAuth("登录后才能查看关注流")) return;

      loadingRef.current = true;
      setLoading(true);
      try {
        let nextVideos: FeedVideo[] = [];
        const activeCursor = append ? cursorRef.current : initialCursor;
        let nextCursor = activeCursor;
        let more = false;

        if (mode === "recommend") {
          const response = await pulsefeedApi.recommend(PAGE_SIZE, append ? activeCursor.recommendCursor : "");
          nextVideos = await hydrateRecommendations(response.videos || []);
          nextCursor = { ...initialCursor, recommendCursor: response.next_cursor || "" };
          more = Boolean(response.has_more);
        } else if (mode === "latest") {
          const response: TimeFeedResponse = await pulsefeedApi.listLatest(PAGE_SIZE, append ? activeCursor.beforeTime : 0);
          nextVideos = await hydrateInteractiveState((response.video_list || []).map(normalizeVideo));
          nextCursor = { ...initialCursor, beforeTime: response.next_before_time || 0 };
          more = Boolean(response.has_more);
        } else if (mode === "following") {
          const response = await pulsefeedApi.listFollowing(PAGE_SIZE, append ? activeCursor.beforeTime : 0);
          nextVideos = await hydrateInteractiveState((response.video_list || []).map(normalizeVideo));
          nextCursor = { ...initialCursor, beforeTime: response.next_before_time || 0 };
          more = Boolean(response.has_more);
        } else if (mode === "likes") {
          const response = await pulsefeedApi.listLikes(PAGE_SIZE, append ? activeCursor.likesCursor : undefined);
          nextVideos = await hydrateInteractiveState((response.video_list || []).map(normalizeVideo));
          nextCursor = { ...initialCursor, likesCursor: response.next_cursor };
          more = Boolean(response.has_more);
        } else {
          const response = await pulsefeedApi.listPopularity(PAGE_SIZE, append ? activeCursor.popularityCursor : undefined);
          nextVideos = await hydrateInteractiveState((response.video_list || []).map(normalizeVideo));
          nextCursor = { ...initialCursor, popularityCursor: response.next_cursor };
          more = Boolean(response.has_more);
        }

        const visibleVideos = append ? nextVideos : await prioritizeSharedVideo(nextVideos);

        setVideos((items) => (append ? [...items, ...visibleVideos] : visibleVideos));
        setFollowedAuthors((items) => {
          const next = append ? new Set(items) : new Set<number>();
          for (const video of visibleVideos) {
            const author = videoAuthor(video);
            if (author.id && video.is_following) {
              next.add(author.id);
            }
          }
          return next;
        });
        cursorRef.current = nextCursor;
        setCursor(nextCursor);
        setHasMore(more);
      } catch (error) {
        pushToast(error instanceof Error ? error.message : `${label}流加载失败`, "error");
      } finally {
        loadingRef.current = false;
        setLoading(false);
      }
    },
    [hydrateInteractiveState, hydrateRecommendations, label, mode, prioritizeSharedVideo, pushToast, requireAuth],
  );

  useEffect(() => {
    setVideos([]);
    setCursor(initialCursor);
    cursorRef.current = initialCursor;
    setHasMore(false);
    scrollRef.current?.scrollTo({ top: 0 });
    fetchFeed(false);
  }, [fetchFeed, mode, session?.token, sharedVideoID]);

  async function toggleLike(video: FeedVideo) {
    if (!requireAuth("登录后才能点赞")) return;
    const previous = Boolean(video.is_liked);
    updateVideo(video.id, (item) => ({
      ...item,
      is_liked: !previous,
      likes_count: Math.max(0, Number(item.likes_count || 0) + (previous ? -1 : 1)),
    }));
    try {
      if (previous) {
        await pulsefeedApi.unlikeVideo(video.id);
      } else {
        await pulsefeedApi.likeVideo(video.id);
      }
    } catch (error) {
      updateVideo(video.id, (item) => ({
        ...item,
        is_liked: previous,
        likes_count: Math.max(0, Number(item.likes_count || 0) + (previous ? 1 : -1)),
      }));
      pushToast(error instanceof Error ? error.message : "点赞失败", "error");
    }
  }

  async function toggleFollowAuthor(authorID: number) {
    if (!authorID || !requireAuth("登录后才能关注作者")) return;
    const wasFollowing = followedAuthors.has(authorID);
    setFollowedAuthors((items) => {
      const next = new Set(items);
      if (wasFollowing) {
        next.delete(authorID);
      } else {
        next.add(authorID);
      }
      return next;
    });
    try {
      if (wasFollowing) {
        await pulsefeedApi.unfollow(authorID);
        pushToast("已取消关注", "success");
      } else {
        await pulsefeedApi.follow(authorID);
        pushToast("已关注", "success");
      }
    } catch (error) {
      setFollowedAuthors((items) => {
        const next = new Set(items);
        if (wasFollowing) {
          next.add(authorID);
        } else {
          next.delete(authorID);
        }
        return next;
      });
      pushToast(error instanceof Error ? error.message : "关注失败", "error");
    }
  }

  function openMessage(authorID: number) {
    if (!authorID || !requireAuth("登录后才能发送私信")) return;
    navigate(`/messages?peer_id=${authorID}`);
  }

  function openComments(video: FeedVideo) {
    setSelectedVideo(video);
  }

  const updateCommentCount = useCallback(
    (videoID: number, count: number) => {
      updateVideo(videoID, (item) => ({ ...item, comments_count: Math.max(0, count) }));
    },
    [updateVideo],
  );

  async function share(video: FeedVideo) {
    await track(video.id, "share");
    const url = `${window.location.origin}/feed/${mode}?video_id=${video.id}`;
    try {
      await navigator.clipboard.writeText(url);
      pushToast("分享链接已复制", "success");
    } catch {
      pushToast("当前浏览器不支持自动复制");
    }
  }

  function handleScroll() {
    const element = scrollRef.current;
    if (!element || !hasMore || loadingRef.current) return;
    const remaining = element.scrollHeight - element.scrollTop - element.clientHeight;
    if (remaining < element.clientHeight * 1.2) {
      fetchFeed(true);
    }
  }

  return (
    <main className="relative h-[100svh] overflow-hidden bg-black">
      <TopStatusBar mode={mode} loading={loading} onRefresh={() => fetchFeed(false)} />
      <div ref={scrollRef} className="h-full snap-y snap-mandatory overflow-y-auto scrollbar-hide" onScroll={handleScroll}>
        {videos.map((video) => {
          const author = videoAuthor(video);
          return (
            <ReelItem
              key={`${mode}-${video.id}-${video.recommendation?.score || ""}`}
              video={video}
              onLike={() => toggleLike(video)}
              onComments={() => openComments(video)}
              onShare={() => share(video)}
              onReport={() => setReportTarget({ type: "video", id: video.id, title: video.title })}
              onFollow={() => toggleFollowAuthor(author.id)}
              onMessage={() => openMessage(author.id)}
              following={followedAuthors.has(author.id)}
              onVisible={() => track(video.id, "impression")}
              onPlayStart={() => track(video.id, "view")}
              onComplete={() => track(video.id, "play_complete")}
            />
          );
        })}

        {!loading && videos.length === 0 ? (
          <section className="grid min-h-[100svh] snap-start place-items-center px-8 text-center">
            <div>
              <p className="text-2xl font-black">{label}流暂无数据</p>
              <p className="mt-3 text-sm leading-6 text-white/58">确认后端已启动，并且数据库已有已发布视频。</p>
              <button className="primary-button mt-5" onClick={() => fetchFeed(false)}>
                重新加载
              </button>
            </div>
          </section>
        ) : null}

        {loading ? (
          <section className="grid min-h-[42svh] snap-start place-items-center pb-28 text-sm font-semibold text-white/58">
            加载中...
          </section>
        ) : null}

        {!hasMore && videos.length > 0 ? (
          <section className="grid min-h-[28svh] snap-start place-items-center pb-28 text-xs font-semibold text-white/42">
            已经到底了
          </section>
        ) : null}
      </div>

      <CommentsDrawer
        video={selectedVideo}
        onClose={() => setSelectedVideo(null)}
        onCountChange={updateCommentCount}
        onReportComment={(comment: Comment) =>
          setReportTarget({ type: "comment", id: comment.id, title: comment.content })
        }
      />
      <ReportDialog target={reportTarget} onClose={() => setReportTarget(null)} />
    </main>
  );
}
