import { RefreshCw, Search } from "lucide-react";
import { Link } from "react-router-dom";
import type { FeedMode } from "../types/api";

const labels: Record<FeedMode, string> = {
  recommend: "推荐",
  latest: "最新",
  following: "关注",
  popularity: "热榜",
  likes: "点赞榜",
};

export function TopStatusBar({
  mode,
  onRefresh,
  loading,
}: {
  mode: FeedMode;
  onRefresh: () => void;
  loading: boolean;
}) {
  return (
    <header className="pointer-events-none fixed inset-x-0 top-0 z-30 px-4 pt-[max(1rem,env(safe-area-inset-top))] md:left-24 md:right-6 md:px-0">
      <div className="mx-auto flex w-full max-w-[1120px] items-center justify-between">
        <div>
          <h1 className="text-lg font-black leading-none">PulseFeed</h1>
          <p className="mt-1 text-xs font-semibold text-white/58">{labels[mode]}视频流</p>
        </div>
        <div className="flex items-center gap-2">
          <Link
            to="/search"
            className="pointer-events-auto rounded-lg bg-black/42 p-2 text-white backdrop-blur-md hover:bg-white/10"
            title="搜索"
          >
            <Search className="h-5 w-5" />
          </Link>
          <button
            type="button"
            className="pointer-events-auto rounded-lg bg-black/42 p-2 text-white backdrop-blur-md hover:bg-white/10"
            onClick={onRefresh}
          >
            <RefreshCw className={["h-5 w-5", loading ? "animate-spin" : ""].join(" ")} />
          </button>
        </div>
      </div>
    </header>
  );
}
