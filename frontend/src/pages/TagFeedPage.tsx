import { ArrowLeft, Hash, Search } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { pulsefeedApi } from "../api/pulsefeed";
import { useToast } from "../hooks/useToast";
import type { FeedVideo } from "../types/api";

const PAGE_SIZE = 20;

export function TagFeedPage() {
  const { name } = useParams<{ name: string }>();
  const tagName = decodeURIComponent(name || "");
  const navigate = useNavigate();
  const { pushToast } = useToast();

  const [videos, setVideos] = useState<FeedVideo[]>([]);
  const [beforeTime, setBeforeTime] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [searchInput, setSearchInput] = useState(tagName);

  useEffect(() => {
    setSearchInput(tagName);
  }, [tagName]);

  function submitSearch(event: FormEvent) {
    event.preventDefault();
    const next = searchInput.trim().replace(/^#+/, "");
    if (!next) return;
    if (next === tagName) return;
    navigate(`/tag/${encodeURIComponent(next)}`);
  }

  const fetchPage = useCallback(
    async (append: boolean) => {
      if (!tagName) return;
      setLoading(true);
      try {
        const resp = await pulsefeedApi.listByTag(tagName, PAGE_SIZE, append ? beforeTime : 0);
        const list = resp.video_list || [];
        setVideos((items) => (append ? [...items, ...list] : list));
        setBeforeTime(resp.next_before_time || 0);
        setHasMore(Boolean(resp.has_more));
      } catch (error) {
        pushToast(error instanceof Error ? error.message : "标签流加载失败", "error");
      } finally {
        setLoading(false);
      }
    },
    [beforeTime, pushToast, tagName],
  );

  useEffect(() => {
    setVideos([]);
    setBeforeTime(0);
    setHasMore(false);
    fetchPage(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tagName]);

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto w-full max-w-[1120px]">
        <button onClick={() => navigate("/feed/recommend")} className="mb-4 flex items-center gap-1 text-sm text-white/60 hover:text-white">
          <ArrowLeft className="h-4 w-4" /> 返回主页
        </button>
        <header className="mb-5 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="grid h-12 w-12 place-items-center rounded-lg bg-pulse-cyan/15">
              <Hash className="h-6 w-6 text-pulse-cyan" />
            </div>
            <div>
              <h1 className="text-2xl font-black md:text-3xl">{tagName ? `#${tagName}` : "标签搜索"}</h1>
              <p className="mt-1 text-sm text-white/52">
                {tagName ? `该标签下共 ${videos.length}${hasMore ? "+" : ""} 个视频` : "输入标签名查看相关视频"}
              </p>
            </div>
          </div>
          <form onSubmit={submitSearch} className="flex w-full items-center gap-2 md:w-72">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-white/52" />
              <input
                className="control-field pl-9"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="搜索标签（例如 Go）"
              />
            </div>
            <button type="submit" className="primary-button">搜索</button>
          </form>
        </header>

        {!tagName ? (
          <div className="glass-panel rounded-lg p-6 text-center text-sm text-white/52">
            在上方输入标签名后回车，或者点击视频里的 <span className="text-pulse-cyan">#标签</span> 进入。
          </div>
        ) : loading && videos.length === 0 ? (
          <p className="text-sm text-white/52">加载中...</p>
        ) : videos.length === 0 ? (
          <div className="glass-panel rounded-lg p-6 text-center text-sm text-white/52">该标签下还没有视频</div>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
              {videos.map((v) => (
                <Link key={v.id} to={`/video/${v.id}`} className="group block overflow-hidden rounded-lg bg-white/[0.06]">
                  <div className="aspect-[3/4] bg-black">
                    {v.cover_url ? <img src={v.cover_url} alt={v.title} className="h-full w-full object-cover transition group-hover:scale-105" /> : null}
                  </div>
                  <div className="p-2">
                    <p className="line-clamp-2 text-xs font-semibold">{v.title}</p>
                    <p className="mt-1 text-[0.65rem] text-white/52">@{v.username || v.author?.username || "-"} · ♥ {v.likes_count}</p>
                  </div>
                </Link>
              ))}
            </div>
            {hasMore && (
              <button className="ghost-button mx-auto mt-6 block" onClick={() => fetchPage(true)} disabled={loading}>
                {loading ? "加载中..." : "加载更多"}
              </button>
            )}
          </>
        )}
      </div>
    </main>
  );
}
