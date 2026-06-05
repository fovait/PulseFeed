import { Bell, CheckCheck, Heart, MessageCircle, UserPlus } from "lucide-react";
import { Link } from "react-router-dom";
import { useAuth } from "../hooks/useAuth";
import { useNotifications } from "../hooks/useNotifications";
import { requestUnreadRefresh } from "../hooks/useUnreadSummary";
import type { Notification } from "../types/api";
import { formatRelativeTime } from "../utils/time";
import { useEffect } from "react";

function notificationIcon(type: string) {
  if (type.includes("like")) return <Heart className="h-4 w-4 text-pulse-red" />;
  if (type.includes("comment")) return <MessageCircle className="h-4 w-4 text-pulse-cyan" />;
  if (type.includes("follow") || type.includes("social")) return <UserPlus className="h-4 w-4 text-pulse-cyan" />;
  return <Bell className="h-4 w-4 text-white/70" />;
}

function notificationLink(n: Notification): string | null {
  if (n.type.includes("follow") || n.type.includes("social")) {
    return `/user/${n.sender_id}`;
  }
  if (n.type.includes("like") || n.type.includes("comment")) {
    if (n.target_id) return `/video/${n.target_id}`;
  }
  return null;
}

export function NotificationsPage() {
  const { session, openAuth } = useAuth();
  const { notifications, unreadCount, refresh, markRead } = useNotifications(session);

  useEffect(() => {
    if (session?.token) {
      refresh().catch(() => undefined);
    }
  }, [refresh, session?.token]);

  async function handleMarkAll() {
    await markRead();
    requestUnreadRefresh();
  }

  async function handleClick(n: Notification) {
    if (!n.is_read) {
      await markRead(n.id);
      requestUnreadRefresh();
    }
  }

  if (!session?.token) {
    return (
      <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
        <div className="mx-auto w-full max-w-[820px]">
          <h1 className="text-2xl font-black md:text-3xl">通知</h1>
          <section className="glass-panel mt-6 rounded-lg p-6">
            <Bell className="h-12 w-12 text-white/40" />
            <p className="mt-4 text-sm text-white/58">登录后才能查看通知</p>
            <button className="primary-button mt-4" onClick={() => openAuth("登录后查看通知")}>
              登录 / 注册
            </button>
          </section>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto w-full max-w-[820px]">
        <header className="mb-5 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-black md:text-3xl">通知</h1>
            <p className="mt-1 text-sm text-white/52">{unreadCount > 0 ? `${unreadCount} 条未读` : "已全部读完"}</p>
          </div>
          {unreadCount > 0 && (
            <button className="ghost-button flex items-center gap-2" onClick={handleMarkAll}>
              <CheckCheck className="h-4 w-4" />
              全部已读
            </button>
          )}
        </header>

        {notifications.length === 0 ? (
          <div className="glass-panel rounded-lg p-6 text-center text-sm text-white/52">还没有通知</div>
        ) : (
          <ul className="space-y-2">
            {notifications.map((n) => {
              const link = notificationLink(n);
              const body = (
                <div className={["glass-panel flex items-start gap-3 rounded-lg p-3 transition hover:bg-white/[0.04]", n.is_read ? "opacity-70" : ""].join(" ")}>
                  <div className="grid h-9 w-9 place-items-center rounded-lg bg-white/[0.06]">
                    {notificationIcon(n.type)}
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm">{n.content || n.type}</p>
                    <p className="mt-1 text-xs text-white/42">{formatRelativeTime(n.created_at)}</p>
                  </div>
                  {!n.is_read && <span className="mt-2 h-2 w-2 rounded-full bg-pulse-red" />}
                </div>
              );
              return (
                <li key={n.id}>
                  {link ? (
                    <Link to={link} onClick={() => handleClick(n)}>
                      {body}
                    </Link>
                  ) : (
                    <button className="block w-full text-left" onClick={() => handleClick(n)}>
                      {body}
                    </button>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </main>
  );
}
