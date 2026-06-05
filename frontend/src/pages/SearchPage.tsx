import { ArrowLeft, Hash, Search, UserRound } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError } from "../api/client";
import { pulsefeedApi } from "../api/pulsefeed";
import { useToast } from "../hooks/useToast";
import type { Account } from "../types/api";

type Mode = "tag" | "user";

export function SearchPage() {
  const navigate = useNavigate();
  const { pushToast } = useToast();
  const [mode, setMode] = useState<Mode>("tag");
  const [tagQuery, setTagQuery] = useState("");
  const [userQuery, setUserQuery] = useState("");
  const [userResult, setUserResult] = useState<Account | null>(null);
  const [userSearching, setUserSearching] = useState(false);
  const [userTriedQuery, setUserTriedQuery] = useState("");

  function submitTag(event: FormEvent) {
    event.preventDefault();
    const next = tagQuery.trim().replace(/^#+/, "");
    if (!next) return;
    navigate(`/tag/${encodeURIComponent(next)}`);
  }

  async function submitUser(event: FormEvent) {
    event.preventDefault();
    const q = userQuery.trim().replace(/^@+/, "");
    if (!q) return;
    setUserSearching(true);
    setUserTriedQuery(q);
    try {
      const account = await pulsefeedApi.findAccountByUsername(q);
      setUserResult(account);
    } catch (err) {
      setUserResult(null);
      if (err instanceof ApiError && err.status === 404) {
        // 不弹 toast，下面会显示「未找到」
      } else {
        pushToast(err instanceof Error ? err.message : "搜索失败", "error");
      }
    } finally {
      setUserSearching(false);
    }
  }

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto w-full max-w-[720px]">
        <button
          onClick={() => navigate("/feed/recommend")}
          className="mb-4 flex items-center gap-1 text-sm text-white/60 hover:text-white"
        >
          <ArrowLeft className="h-4 w-4" /> 返回主页
        </button>
        <header className="mb-5">
          <h1 className="text-2xl font-black md:text-3xl">搜索</h1>
          <p className="mt-1 text-sm text-white/52">支持按标签或用户名查找</p>
        </header>

        <div className="mb-4 flex gap-1 border-b border-white/10">
          <button
            onClick={() => setMode("tag")}
            className={[
              "flex items-center gap-1.5 px-3 py-2 text-sm font-bold transition",
              mode === "tag" ? "border-b-2 border-pulse-cyan text-white" : "text-white/52 hover:text-white",
            ].join(" ")}
          >
            <Hash className="h-4 w-4" /> 标签
          </button>
          <button
            onClick={() => setMode("user")}
            className={[
              "flex items-center gap-1.5 px-3 py-2 text-sm font-bold transition",
              mode === "user" ? "border-b-2 border-pulse-cyan text-white" : "text-white/52 hover:text-white",
            ].join(" ")}
          >
            <UserRound className="h-4 w-4" /> 用户
          </button>
        </div>

        {mode === "tag" ? (
          <form onSubmit={submitTag} className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-white/52" />
              <input
                className="control-field pl-9"
                value={tagQuery}
                onChange={(e) => setTagQuery(e.target.value)}
                placeholder="输入标签名（例如 Go）"
                autoFocus
              />
            </div>
            <button type="submit" className="primary-button">
              搜索
            </button>
          </form>
        ) : (
          <>
            <form onSubmit={submitUser} className="flex items-center gap-2">
              <div className="relative flex-1">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-white/52" />
                <input
                  className="control-field pl-9"
                  value={userQuery}
                  onChange={(e) => setUserQuery(e.target.value)}
                  placeholder="输入完整用户名"
                  autoFocus
                />
              </div>
              <button type="submit" className="primary-button" disabled={userSearching}>
                {userSearching ? "查询中..." : "搜索"}
              </button>
            </form>
            <p className="mt-2 text-xs text-white/42">注意：用户名为精确匹配</p>

            <div className="mt-4">
              {userResult ? (
                <Link
                  to={`/user/${userResult.id}`}
                  className="glass-panel flex items-center gap-3 rounded-lg p-3 transition hover:bg-white/[0.04]"
                >
                  <div className="grid h-12 w-12 overflow-hidden rounded-lg bg-white/10">
                    {userResult.avatar_url ? (
                      <img src={userResult.avatar_url} alt={userResult.username} className="h-full w-full object-cover" />
                    ) : (
                      <UserRound className="m-auto h-6 w-6 text-pulse-cyan" />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-bold">@{userResult.username}</p>
                    {userResult.bio ? <p className="truncate text-xs text-white/52">{userResult.bio}</p> : null}
                  </div>
                </Link>
              ) : userTriedQuery && !userSearching ? (
                <div className="glass-panel rounded-lg p-6 text-center text-sm text-white/52">
                  未找到用户 @{userTriedQuery}
                </div>
              ) : null}
            </div>
          </>
        )}
      </div>
    </main>
  );
}
