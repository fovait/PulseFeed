import { ArrowLeft, UserRound } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { pulsefeedApi } from "../api/pulsefeed";
import { useToast } from "../hooks/useToast";
import type { Account } from "../types/api";

type Props = {
  mode: "followers" | "following";
};

export function FollowListPage({ mode }: Props) {
  const { id } = useParams<{ id: string }>();
  const userID = Number(id);
  const navigate = useNavigate();
  const { pushToast } = useToast();
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [count, setCount] = useState(0);
  const [loading, setLoading] = useState(true);

  const title = mode === "followers" ? "粉丝" : "关注";

  const load = useCallback(async () => {
    if (!userID) return;
    setLoading(true);
    try {
      if (mode === "followers") {
        const r = await pulsefeedApi.listSocialFollowers(userID);
        setAccounts(r.followers || []);
        setCount(r.follower_count || 0);
      } else {
        const r = await pulsefeedApi.listSocialFollowing(userID);
        setAccounts(r.vloggers || []);
        setCount(r.vlogger_count || 0);
      }
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "加载失败", "error");
    } finally {
      setLoading(false);
    }
  }, [mode, pushToast, userID]);

  useEffect(() => {
    load();
  }, [load]);

  if (!userID) {
    return <main className="min-h-[100svh] bg-pulse-black p-6 text-white/60">无效的用户 ID</main>;
  }

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto w-full max-w-[720px]">
        <button onClick={() => navigate(-1)} className="mb-4 flex items-center gap-1 text-sm text-white/60 hover:text-white">
          <ArrowLeft className="h-4 w-4" /> 返回
        </button>
        <header className="mb-5">
          <h1 className="text-2xl font-black md:text-3xl">{title}</h1>
          <p className="mt-1 text-sm text-white/52">共 {count} 人</p>
        </header>

        {loading ? (
          <p className="text-sm text-white/52">加载中...</p>
        ) : accounts.length === 0 ? (
          <div className="glass-panel rounded-lg p-6 text-center text-sm text-white/52">
            {mode === "followers" ? "还没有粉丝" : "还没有关注任何人"}
          </div>
        ) : (
          <ul className="space-y-2">
            {accounts.map((a) => (
              <li key={a.id}>
                <Link
                  to={`/user/${a.id}`}
                  className="glass-panel flex items-center gap-3 rounded-lg p-3 transition hover:bg-white/[0.04]"
                >
                  <div className="grid h-12 w-12 overflow-hidden rounded-lg bg-white/10">
                    {a.avatar_url ? (
                      <img src={a.avatar_url} alt={a.username} className="h-full w-full object-cover" />
                    ) : (
                      <UserRound className="m-auto h-6 w-6 text-pulse-cyan" />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-bold">@{a.username}</p>
                    {a.bio ? <p className="truncate text-xs text-white/52">{a.bio}</p> : null}
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </main>
  );
}
