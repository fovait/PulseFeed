import { ImagePlus, LogOut, Plus, Save, UserRound } from "lucide-react";
import { useEffect, useMemo, useState, type ChangeEvent, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { API_BASE_URL } from "../api/client";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";
import type { ProfileResponse } from "../types/api";

export function ProfilePage() {
  const { session, logout, openAuth, updateSession } = useAuth();
  const { pushToast } = useToast();
  const [profile, setProfile] = useState<ProfileResponse | null>(null);
  const [username, setUsername] = useState("");
  const [bio, setBio] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");
  const [savingProfile, setSavingProfile] = useState(false);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [changingPassword, setChangingPassword] = useState(false);
  const savedAvatarUrl = useMemo(() => profile?.account.avatar_url || "", [profile]);
  const previewAvatarUrl = avatarUrl.trim() || savedAvatarUrl;

  useEffect(() => {
    if (!session?.account_id) {
      setProfile(null);
      return;
    }
    pulsefeedApi.getProfile(session.account_id).then(setProfile).catch(() => setProfile(null));
  }, [session?.account_id]);

  useEffect(() => {
    setUsername(session?.username || "");
  }, [session?.username]);

  useEffect(() => {
    setBio(profile?.account.bio || "");
    setAvatarUrl(profile?.account.avatar_url || "");
  }, [profile]);

  async function refreshProfile() {
    if (!session?.account_id) return;
    const next = await pulsefeedApi.getProfile(session.account_id);
    setProfile(next);
  }

  async function saveProfile(event: FormEvent) {
    event.preventDefault();
    if (!session?.token) return;
    const nextUsername = username.trim();
    if (!nextUsername) {
      pushToast("昵称不能为空", "error");
      return;
    }
    setSavingProfile(true);
    try {
      await pulsefeedApi.updateProfile({
        avatar_url: avatarUrl.trim(),
        bio: bio.trim(),
      });
      if (nextUsername !== session.username) {
        const response = await pulsefeedApi.rename(nextUsername);
        updateSession((current) => ({
          ...current,
          token: response.token || current.token,
          username: nextUsername,
        }));
      }
      await refreshProfile();
      pushToast("资料已保存", "success");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "资料保存失败", "error");
    } finally {
      setSavingProfile(false);
    }
  }

  async function uploadAvatar(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file || !session?.token) return;
    setUploadingAvatar(true);
    try {
      const formData = new FormData();
      formData.append("file", file);
      const response = await pulsefeedApi.uploadAvatar(formData);
      setAvatarUrl(response.avatar_url);
      await refreshProfile();
      pushToast("头像已更新", "success");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "头像上传失败", "error");
    } finally {
      setUploadingAvatar(false);
    }
  }

  async function submitPassword(event: FormEvent) {
    event.preventDefault();
    if (!session?.username) return;
    if (!oldPassword || !newPassword) {
      pushToast("请输入旧密码和新密码", "error");
      return;
    }
    setChangingPassword(true);
    try {
      await pulsefeedApi.changePassword(session.username, oldPassword, newPassword);
      setOldPassword("");
      setNewPassword("");
      logout();
      openAuth("密码已修改，请重新登录");
      pushToast("密码已修改", "success");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "密码修改失败", "error");
    } finally {
      setChangingPassword(false);
    }
  }

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
                  <div className="grid h-16 w-16 overflow-hidden rounded-lg bg-white/10 md:h-20 md:w-20">
                    {previewAvatarUrl ? (
                      <img className="h-full w-full object-cover" src={previewAvatarUrl} alt={session.username} />
                    ) : (
                      <UserRound className="m-auto h-8 w-8 text-pulse-cyan md:h-10 md:w-10" />
                    )}
                  </div>
                  <div>
                    <h2 className="text-2xl font-black md:text-4xl">@{session.username}</h2>
                    <p className="mt-1 text-sm text-white/52">{profile?.account.bio || `account_id #${session.account_id}`}</p>
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

              <form className="mt-6 grid gap-4" onSubmit={saveProfile}>
                <label className="block">
                  <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">昵称</span>
                  <input className="control-field" value={username} onChange={(event) => setUsername(event.target.value)} maxLength={32} />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">简介</span>
                  <textarea
                    className="control-field min-h-24 resize-none"
                    value={bio}
                    onChange={(event) => setBio(event.target.value)}
                    maxLength={512}
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">头像 URL</span>
                  <input className="control-field" value={avatarUrl} onChange={(event) => setAvatarUrl(event.target.value)} />
                </label>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <label className="ghost-button flex cursor-pointer items-center justify-center gap-2">
                    <ImagePlus className="h-4 w-4 text-pulse-cyan" />
                    {uploadingAvatar ? "上传中..." : "上传头像"}
                    <input className="sr-only" type="file" accept=".jpg,.jpeg,.png,.webp,image/*" onChange={uploadAvatar} disabled={uploadingAvatar} />
                  </label>
                  <button className="primary-button flex items-center justify-center gap-2 disabled:opacity-60" disabled={savingProfile}>
                    <Save className="h-4 w-4" />
                    {savingProfile ? "保存中..." : "保存资料"}
                  </button>
                </div>
              </form>
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
                <h2 className="font-black">修改密码</h2>
                <form className="mt-4 grid gap-3" onSubmit={submitPassword}>
                  <input
                    className="control-field"
                    type="password"
                    value={oldPassword}
                    onChange={(event) => setOldPassword(event.target.value)}
                    placeholder="旧密码"
                  />
                  <input
                    className="control-field"
                    type="password"
                    value={newPassword}
                    onChange={(event) => setNewPassword(event.target.value)}
                    placeholder="新密码"
                  />
                  <button className="ghost-button disabled:opacity-60" disabled={changingPassword || !oldPassword || !newPassword}>
                    {changingPassword ? "修改中..." : "修改密码"}
                  </button>
                </form>
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
