import { MessageCircle, Send, UserPlus } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { ActionRail } from "./ActionRail";
import type { FeedVideo } from "../types/api";
import { formatRelativeTime } from "../utils/time";
import { videoAuthor } from "../utils/video";

export function ReelItem({
  video,
  onLike,
  onComments,
  onShare,
  onReport,
  onFollow,
  onMessage,
  following,
  onVisible,
  onPlayStart,
  onComplete,
}: {
  video: FeedVideo;
  onLike: () => void;
  onComments: () => void;
  onShare: () => void;
  onReport: () => void;
  onFollow: (authorID: number) => void;
  onMessage: (authorID: number) => void;
  following: boolean;
  onVisible: () => void;
  onPlayStart: () => void;
  onComplete: () => void;
}) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const rootRef = useRef<HTMLElement | null>(null);
  const [muted, setMuted] = useState(false);
  const [playbackBlocked, setPlaybackBlocked] = useState(false);
  const author = videoAuthor(video);

  useEffect(() => {
    const root = rootRef.current;
    const player = videoRef.current;
    if (!root || !player || !video.play_url) return undefined;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && entry.intersectionRatio > 0.66) {
          onVisible();
          const attempt = player.play();
          if (attempt) {
            attempt
              .then(() => {
                setPlaybackBlocked(false);
              })
              .catch(() => {
                player.muted = true;
                setMuted(true);
                player.play().catch(() => setPlaybackBlocked(true));
              });
          }
        } else {
          player.pause();
        }
      },
      { threshold: [0, 0.4, 0.66, 0.9] },
    );

    observer.observe(root);
    return () => observer.disconnect();
  }, [onVisible, video.play_url]);

  return (
    <article ref={rootRef} className="relative min-h-[100svh] snap-start overflow-hidden bg-black">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(34,211,238,0.14),transparent_34%),linear-gradient(180deg,#050505_0%,#000_55%,#050505_100%)]" />

      <div className="relative z-10 flex min-h-[100svh] items-center justify-center px-3 pb-20 pt-16 md:pb-8 md:pl-28 md:pr-8 md:pt-16">
        <div className="flex w-full max-w-[1180px] items-end justify-center gap-4 md:gap-5">
          <div className="relative h-[calc(100svh-10rem)] min-h-[360px] w-full max-w-[430px] overflow-hidden rounded-lg bg-black shadow-2xl md:aspect-[9/16] md:h-[calc(100svh-7rem)] md:max-h-[860px] md:min-h-[560px] md:w-auto md:max-w-[520px]">
            {video.play_url ? (
              <video
                ref={videoRef}
                className="absolute inset-0 h-full w-full bg-black object-contain"
                src={video.play_url}
                poster={video.cover_url}
                loop={false}
                playsInline
                muted={muted}
                preload="metadata"
                onPlay={onPlayStart}
                onEnded={onComplete}
              />
            ) : (
              <div className="absolute inset-0 grid place-items-center bg-gradient-to-b from-zinc-900 via-black to-zinc-950">
                <div className="px-10 text-center">
                  <div className="mx-auto mb-4 grid h-20 w-20 place-items-center rounded-lg border border-white/12 bg-white/8 text-2xl font-black">
                    #{video.id}
                  </div>
                  <p className="text-sm text-white/58">推荐接口暂未返回视频详情，已展示推荐信息。</p>
                </div>
              </div>
            )}
            <div className="pointer-events-none absolute inset-x-0 bottom-0 h-56 bg-gradient-to-t from-black/88 via-black/38 to-transparent md:hidden" />
            <div className="absolute inset-x-0 bottom-0 z-20 px-4 pb-5 pr-20 md:hidden">
              <VideoInfo
                video={video}
                author={author}
                following={following}
                onFollow={onFollow}
                onMessage={onMessage}
                onComments={onComments}
                compact
              />
            </div>
          </div>

          <ActionRail
            className="absolute bottom-28 right-3 z-20 flex flex-col items-center gap-4 md:static md:mb-5 md:shrink-0 md:gap-5"
            liked={Boolean(video.is_liked)}
            likesCount={Number(video.likes_count || 0)}
            commentsCount={Number(video.comments_count || 0)}
            onLike={onLike}
            onComments={onComments}
            onShare={onShare}
            onReport={onReport}
          />

          <aside className="hidden w-[min(34vw,420px)] max-w-[420px] pb-6 md:block">
            <VideoInfo
              video={video}
              author={author}
              following={following}
              onFollow={onFollow}
              onMessage={onMessage}
              onComments={onComments}
            />
          </aside>
        </div>
      </div>

      {playbackBlocked ? (
        <button
          type="button"
          className="absolute left-1/2 top-1/2 z-30 -translate-x-1/2 -translate-y-1/2 rounded-lg bg-white px-4 py-2 text-sm font-black text-black"
          onClick={() => videoRef.current?.play()}
        >
          点击播放
        </button>
      ) : null}
    </article>
  );
}

function VideoInfo({
  video,
  author,
  following,
  onFollow,
  onMessage,
  onComments,
  compact = false,
}: {
  video: FeedVideo;
  author: ReturnType<typeof videoAuthor>;
  following: boolean;
  onFollow: (authorID: number) => void;
  onMessage: (authorID: number) => void;
  onComments: () => void;
  compact?: boolean;
}) {
  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <button
          type="button"
          className="ghost-button flex items-center gap-1.5 py-2"
          onClick={() => onFollow(author.id)}
          disabled={!author.id}
        >
          <UserPlus className="h-4 w-4 text-pulse-cyan" />
          {following ? "已关注" : "关注"}
        </button>
        <button
          type="button"
          className="ghost-button flex items-center gap-1.5 py-2"
          onClick={() => onMessage(author.id)}
          disabled={!author.id}
        >
          <Send className="h-4 w-4 text-pulse-cyan" />
          私信
        </button>
        <button type="button" className="ghost-button flex items-center gap-1.5 py-2" onClick={onComments}>
          <MessageCircle className="h-4 w-4 text-pulse-cyan" />
          评论
        </button>
      </div>
      <p className="text-sm font-bold text-white/86">@{author.username}</p>
      <h2 className={["mt-1 line-clamp-2 font-black leading-tight", compact ? "text-2xl" : "text-3xl"].join(" ")}>
        {video.title || `视频 #${video.id}`}
      </h2>
      {video.description ? (
        <p className={["mt-2 text-sm leading-6 text-white/78", compact ? "line-clamp-2" : "line-clamp-4"].join(" ")}>
          {video.description}
        </p>
      ) : null}
      <div className="mt-3 flex flex-wrap items-center gap-2 text-xs font-semibold text-white/52">
        <span>{formatRelativeTime(video.create_time || video.created_at)}</span>
        {video.recommendation ? (
          <>
            <span>score {video.recommendation.score.toFixed(2)}</span>
            <span>{video.recommendation.source}</span>
          </>
        ) : null}
      </div>
      {video.recommendation?.reasons?.length ? (
        <p className="mt-2 line-clamp-2 text-xs text-cyan-100/72">
          {video.recommendation.reasons.join(" · ")}
        </p>
      ) : null}
    </div>
  );
}
