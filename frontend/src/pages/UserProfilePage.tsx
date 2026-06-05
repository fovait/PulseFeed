import { ArrowLeft, MessageCircle, UserPlus, UserRound, UserX } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";
import type { FeedVideo, ProfileResponse } from "../types/api";

export function UserProfilePage() {
  const { id } = useParams<{ id: string }>();
  const userID = Number(id);
  const navigate = useNavigate();
  const { session, requireAuth } = useAuth();
  const { pushToast } = useToast();

  const [profile, setProfile] = useState<ProfileResponse | null>(null);
  const [videos, setVideos] = useState<FeedVideo[]>([]);
  const [loading, setLoading] = useState(true);
  const [isFollowed, setIsFollowed] = useState(false);
  const [following, setFollowing] = useState(false);

  const isSelf = session?.account_id === userID;

  const refresh = useCallback(async () => {
    if (!userID) return;
    setLoading(true);
    try {
      const [p, vs] = await Promise.all([
        pulsefeedApi.getProfile(userID),
        pulsefeedApi.listVideosByAuthor(userID).catch(() => [] as FeedVideo[]),
      ]);
      setProfile(p);
      setVideos(vs || []);
      if (session?.token && !isSelf) {
        try {
          const r = await pulsefeedApi.isFollowing(userID);
          setIsFollowed(r.is_followed);
        } catch {
          setIsFollowed(false);
        }
      } else {
        setIsFollowed(false);
      }
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "加载失败", "error");
    } finally {
      setLoading(false);
    }
  }, [isSelf, pushToast, session?.token, userID]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  async function toggleFollow() {
    if (!requireAuth("登录后才能关注")) return;
    if (isSelf || !userID) return;
    setFollowing(true);
    try {
      if (isFollowed) {
        await pulsefeedApi.unfollow(userID);
        setIsFollowed(false);
        pushToast("已取消关注");
      } else {
        await pulsefeedApi.follow(userID);
        setIsFollowed(true);
        pushToast("已关注", "success");
      }
      await refresh();
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "操作失败", "error");
    } finally {
      setFollowing(false);
    }
  }

  if (!userID) {
    return (
      <main className="min-h-[100svh] bg-pulse-black p-6 text-white/60">
        无效的用户 ID
      </main>
    );
  }

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto w-full max-w-[1120px]">
        <button
          onClick={() => navigate(-1)}
          className="mb-4 flex items-center gap-1 text-sm text-white/60 hover:text-white"
        >
          <ArrowLeft className="h-4 w-4" /> 返回
        </button>

        <section className="glass-panel rounded-lg p-4 md:p-6">
          <div className="flex flex-col gap-5 md:flex-row md:items-center md:justify-between">
            <div className="flex items-center gap-4">
              <div className="grid h-16 w-16 overflow-hidden rounded-lg bg-white/10 md:h-20 md:w-20">
                {profile?.account.avatar_url ? (
                  <img className="h-full w-full object-cover" src={profile.account.avatar_url} alt={profile.account.username} />
                ) : (
                  <UserRound className="m-auto h-8 w-8 text-pulse-cyan md:h-10 md:w-10" />
                )}
              </div>
              <div>
                <h1 className="text-2xl font-black md:text-4xl">@{profile?.account.username || (loading ? "…" : "")}</h1>
                <p className="mt-1 text-sm text-white/52">{profile?.account.bio || `account_id #${userID}`}</p>
              </div>
            </div>

            {!isSelf && (
              <div className="flex gap-2">
                <button
                  onClick={toggleFollow}
                  disabled={following}
                  className={isFollowed ? "ghost-button flex items-center gap-2" : "primary-button flex items-center gap-2"}
                >
                  {isFollowed ? <UserX className="h-4 w-4" /> : <UserPlus className="h-4 w-4" />}
                  {following ? "处理中..." : isFollowed ? "已关注" : "关注"}
                </button>
                <Link to={`/messages?peer_id=${userID}`} className="ghost-button flex items-center gap-2">
                  <MessageCircle className="h-4 w-4" />
                  私信
                </Link>
              </div>
            )}
          </div>

          <div className="mt-6 grid grid-cols-4 gap-3 text-center">
            <Stat label="视频" value={profile?.video_count} />
            <Stat label="获赞" value={profile?.total_likes} />
            <Link to={`/user/${userID}/followers`} className="block rounded-lg bg-white/[0.06] px-2 py-4 transition hover:bg-white/[0.1]">
              <p className="text-2xl font-black md:text-3xl">{profile?.follower_count ?? "-"}</p>
              <p className="mt-1 text-xs text-white/42">粉丝</p>
            </Link>
            <Link to={`/user/${userID}/following`} className="block rounded-lg bg-white/[0.06] px-2 py-4 transition hover:bg-white/[0.1]">
              <p className="text-2xl font-black md:text-3xl">{profile?.vlogger_count ?? "-"}</p>
              <p className="mt-1 text-xs text-white/42">关注</p>
            </Link>
          </div>
        </section>

        <section className="mt-6">
          <h2 className="mb-3 text-lg font-black">作品 ({videos.length})</h2>
          {loading ? (
            <p className="text-sm text-white/52">加载中...</p>
          ) : videos.length === 0 ? (
            <p className="text-sm text-white/52">还没有发布过视频</p>
          ) : (
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
              {videos.map((v) => (
                <Link
                  key={v.id}
                  to={`/video/${v.id}`}
                  className="group block overflow-hidden rounded-lg bg-white/[0.06]"
                >
                  <div className="aspect-[3/4] bg-black">
                    {v.cover_url ? (
                      <img src={v.cover_url} alt={v.title} className="h-full w-full object-cover transition group-hover:scale-105" />
                    ) : null}
                  </div>
                  <div className="p-2">
                    <p className="line-clamp-2 text-xs font-semibold">{v.title}</p>
                    <p className="mt-1 text-[0.65rem] text-white/52">♥ {v.likes_count} · 💬 {v.comments_count}</p>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </section>
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
