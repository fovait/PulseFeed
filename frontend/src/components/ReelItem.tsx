import { MessageCircle, Pause, Play, Send, UserPlus, Volume2, VolumeX } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { ActionRail } from "./ActionRail";
import type { FeedVideo } from "../types/api";
import { renderWithTags } from "../utils/tags";
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
  const [paused, setPaused] = useState(false);
  const [active, setActive] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const author = videoAuthor(video);

  function togglePause() {
    const player = videoRef.current;
    if (!player) return;
    if (player.paused) {
      player.play().catch(() => {});
    } else {
      player.pause();
    }
  }

  function toggleMute() {
    const player = videoRef.current;
    const next = !muted;
    if (player) {
      player.muted = next;
    }
    setMuted(next);
  }

  function seek(nextTime: number) {
    const player = videoRef.current;
    if (!player || !Number.isFinite(nextTime)) return;
    player.currentTime = nextTime;
    setCurrentTime(nextTime);
  }

  useEffect(() => {
    const root = rootRef.current;
    const player = videoRef.current;
    if (!root || !player || !video.play_url) return undefined;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && entry.intersectionRatio > 0.66) {
          setActive(true);
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
          setActive(false);
          player.pause();
        }
      },
      { threshold: [0, 0.4, 0.66, 0.9] },
    );

    observer.observe(root);
    return () => observer.disconnect();
  }, [onVisible, video.play_url]);

  useEffect(() => {
    setCurrentTime(0);
    setDuration(0);
    setPaused(false);
    setPlaybackBlocked(false);
  }, [video.play_url]);

  useEffect(() => {
    if (!active || !video.play_url) return undefined;

    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.matches("input, textarea, select, [contenteditable='true']")) return;
      if (event.code === "Space") {
        event.preventDefault();
        togglePause();
      }
      if (event.key.toLowerCase() === "m") {
        toggleMute();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [active, muted, video.play_url]);

  return (
    <article ref={rootRef} className="relative min-h-[100svh] snap-start overflow-hidden bg-black">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(34,211,238,0.14),transparent_34%),linear-gradient(180deg,#050505_0%,#000_55%,#050505_100%)]" />

      <div className="relative z-10 flex min-h-[100svh] items-center justify-center px-3 pb-20 pt-16 md:pb-8 md:pl-28 md:pr-8 md:pt-16">
        <div className="flex w-full max-w-[1180px] items-end justify-center gap-4 md:gap-5">
          <div className="relative h-[calc(100svh-10rem)] min-h-[360px] w-full max-w-[430px] overflow-hidden rounded-lg bg-black shadow-2xl md:aspect-[9/16] md:h-[calc(100svh-7rem)] md:max-h-[860px] md:min-h-[560px] md:w-auto md:max-w-[520px]">
            {video.play_url ? (
              <>
                <video
                  ref={videoRef}
                  className="absolute inset-0 h-full w-full bg-black object-contain"
                  src={video.play_url}
                  poster={video.cover_url}
                  loop={false}
                  playsInline
                  muted={muted}
                  preload="metadata"
                  onPlay={() => { setPaused(false); onPlayStart(); }}
                  onPause={() => setPaused(true)}
                  onEnded={() => { setPaused(true); onComplete(); }}
                  onLoadedMetadata={(event) => setDuration(event.currentTarget.duration || 0)}
                  onTimeUpdate={(event) => setCurrentTime(event.currentTarget.currentTime || 0)}
                  onClick={togglePause}
                />
                {paused && (
                  <div className="pointer-events-none absolute inset-0 grid place-items-center">
                    <div className="rounded-full bg-black/50 p-4">
                      <Play className="h-10 w-10 fill-white text-white" />
                    </div>
                  </div>
                )}
                <div className="absolute right-3 top-3 z-30 flex gap-2">
                  <button
                    type="button"
                    className="grid h-10 w-10 place-items-center rounded-full bg-black/50 text-white shadow-lg backdrop-blur-md transition hover:bg-white/14"
                    onClick={toggleMute}
                    aria-label={muted ? "取消静音" : "静音"}
                  >
                    {muted ? <VolumeX className="h-5 w-5" /> : <Volume2 className="h-5 w-5" />}
                  </button>
                  <button
                    type="button"
                    className="grid h-10 w-10 place-items-center rounded-full bg-black/50 text-white shadow-lg backdrop-blur-md transition hover:bg-white/14"
                    onClick={togglePause}
                    aria-label={paused ? "播放" : "暂停"}
                  >
                    {paused ? <Play className="h-5 w-5 fill-white" /> : <Pause className="h-5 w-5 fill-white" />}
                  </button>
                </div>
                <div className="absolute inset-x-0 bottom-0 z-30 px-3 pb-2">
                  <input
                    className="h-1 w-full cursor-pointer accent-pulse-cyan"
                    type="range"
                    min={0}
                    max={duration || 0}
                    step={0.1}
                    value={duration ? Math.min(currentTime, duration) : 0}
                    onChange={(event) => seek(Number(event.target.value))}
                    disabled={!duration}
                    aria-label="视频进度"
                  />
                </div>
              </>
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
      {author.id ? (
        <Link to={`/user/${author.id}`} className="text-sm font-bold text-white/86 hover:text-pulse-cyan">
          @{author.username}
        </Link>
      ) : (
        <p className="text-sm font-bold text-white/86">@{author.username}</p>
      )}
      <h2 className={["mt-1 line-clamp-2 font-black leading-tight", compact ? "text-2xl" : "text-3xl"].join(" ")}>
        {video.title ? renderWithTags(video.title) : `视频 #${video.id}`}
      </h2>
      {video.description ? (
        <p className={["mt-2 text-sm leading-6 text-white/78", compact ? "line-clamp-2" : "line-clamp-4"].join(" ")}>
          {renderWithTags(video.description)}
        </p>
      ) : null}
      <div className="mt-3 flex flex-wrap items-center gap-2 text-xs font-semibold text-white/52">
        <span>{formatRelativeTime(video.create_time || video.created_at)}</span>
        {video.recommendation?.source ? (
          <span className="rounded-lg bg-pulse-cyan/15 px-2 py-0.5 text-[0.65rem] text-pulse-cyan">
            {recommendationSourceLabel(video.recommendation.source)}
          </span>
        ) : null}
        {video.recommendation?.reasons?.slice(0, 3).map((reason, i) => (
          <span key={`${reason}-${i}`} className="rounded-lg bg-white/[0.06] px-2 py-0.5 text-[0.65rem] text-white/70">
            {reason}
          </span>
        ))}
      </div>
    </div>
  );
}

function recommendationSourceLabel(source: string): string {
  switch (source) {
    case "latest": return "最新";
    case "popularity": return "热门";
    case "following": return "关注作者";
    case "tag_interest": return "兴趣标签";
    case "likes": return "高赞";
    case "mixed": return "综合推荐";
    default: return source;
  }
}
