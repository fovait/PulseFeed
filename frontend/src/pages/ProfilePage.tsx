import { LogOut, Plus, UserRound } from "lucide-react";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { API_BASE_URL } from "../api/client";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import type { ProfileResponse } from "../types/api";

export function ProfilePage() {
  const { session, logout, openAuth } = useAuth();
  const [profile, setProfile] = useState<ProfileResponse | null>(null);

  useEffect(() => {
    if (!session?.account_id) {
      setProfile(null);
      return;
    }
    pulsefeedApi.getProfile(session.account_id).then(setProfile).catch(() => setProfile(null));
  }, [session?.account_id]);

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto w-full max-w-[1120px]">
        <header className="mb-5">
          <h1 className="text-2xl font-black md:text-3xl">我的</h1>
          <p className="mt-1 text-sm text-white/52">账号和发布</p>
        </header>

        {session?.token ? (
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
            <section className="glass-panel rounded-lg p-4 md:p-6">
              <div className="flex flex-col gap-5 md:flex-row md:items-center md:justify-between">
                <div className="flex items-center gap-4">
                  <div className="grid h-16 w-16 place-items-center rounded-lg bg-white/10 md:h-20 md:w-20">
                    <UserRound className="h-8 w-8 text-pulse-cyan md:h-10 md:w-10" />
                  </div>
                  <div>
                    <h2 className="text-2xl font-black md:text-4xl">@{session.username}</h2>
                    <p className="mt-1 text-sm text-white/52">account_id #{session.account_id}</p>
                  </div>
                </div>
                <div className="flex gap-2 md:hidden">
                  <Link className="primary-button flex items-center justify-center gap-2" to="/publish">
                    <Plus className="h-4 w-4" />
                    发布
                  </Link>
                  <button className="ghost-button flex items-center justify-center gap-2" onClick={logout}>
                    <LogOut className="h-4 w-4" />
                    退出
                  </button>
                </div>
              </div>

              <div className="mt-6 grid grid-cols-2 gap-3 text-center md:grid-cols-4">
                <Stat label="视频" value={profile?.video_count} />
                <Stat label="获赞" value={profile?.total_likes} />
                <Stat label="粉丝" value={profile?.follower_count} />
                <Stat label="关注" value={profile?.vlogger_count} />
              </div>
            </section>

            <aside className="space-y-4">
              <section className="glass-panel rounded-lg p-4">
                <h2 className="font-black">快捷操作</h2>
                <div className="mt-4 grid gap-2">
                  <Link className="primary-button flex items-center justify-center gap-2" to="/publish">
                    <Plus className="h-4 w-4" />
                    发布视频
                  </Link>
                  <button className="ghost-button flex items-center justify-center gap-2" onClick={logout}>
                    <LogOut className="h-4 w-4" />
                    退出登录
                  </button>
                </div>
              </section>

              <section className="glass-panel rounded-lg p-4">
                <h2 className="font-black">API</h2>
                <p className="mt-2 break-all text-sm text-white/58">{API_BASE_URL}</p>
              </section>
            </aside>
          </div>
        ) : (
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
            <section className="glass-panel rounded-lg p-6 md:min-h-[360px]">
              <div className="flex h-full flex-col justify-center">
                <UserRound className="h-14 w-14 text-white/42" />
                <h2 className="mt-5 text-3xl font-black">登录 PulseFeed</h2>
                <p className="mt-3 max-w-xl text-sm leading-6 text-white/58">
                  登录后可以点赞、评论、关注、私信、发布和接收通知。
                </p>
                <button className="primary-button mt-6 w-fit" onClick={() => openAuth("登录后进入我的页面")}>
                  登录 / 注册
                </button>
              </div>
            </section>

            <section className="glass-panel rounded-lg p-4">
              <h2 className="font-black">API</h2>
              <p className="mt-2 break-all text-sm text-white/58">{API_BASE_URL}</p>
            </section>
          </div>
        )}
      </div>
    </main>
  );
}

function Stat({ label, value }: { label: string; value?: number }) {
  return (
    <div className="rounded-lg bg-white/[0.06] px-2 py-4">
      <p className="text-2xl font-black md:text-3xl">{value ?? "-"}</p>
      <p className="mt-1 text-xs text-white/42">{label}</p>
    </div>
  );
}
