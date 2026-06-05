import { Bell, Flame, Heart, Home, MessageCircle, Radio, UserRound, UsersRound } from "lucide-react";
import { NavLink } from "react-router-dom";
import type { ComponentType } from "react";

type NavItem = {
  to: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
};

const items: NavItem[] = [
  { to: "/feed/recommend", label: "推荐", icon: Home },
  { to: "/feed/latest", label: "最新", icon: Radio },
  { to: "/feed/following", label: "关注", icon: UsersRound },
  { to: "/feed/popularity", label: "热榜", icon: Flame },
  { to: "/feed/likes", label: "点赞榜", icon: Heart },
  { to: "/messages", label: "消息", icon: MessageCircle },
  { to: "/notifications", label: "通知", icon: Bell },
  { to: "/profile", label: "我的", icon: UserRound },
];

export function BottomNav({
  wide = false,
  messageUnread = 0,
  notificationUnread = 0,
}: {
  wide?: boolean;
  messageUnread?: number;
  notificationUnread?: number;
}) {
  const navClassName = wide
    ? "fixed inset-x-0 bottom-0 z-40 w-full border-t border-white/10 bg-black/82 px-2 pb-[max(0.55rem,env(safe-area-inset-bottom))] pt-2 backdrop-blur-xl md:inset-x-auto md:inset-y-0 md:left-0 md:flex md:w-24 md:flex-col md:border-r md:border-t-0 md:px-3 md:py-5"
    : "fixed inset-x-0 bottom-0 z-40 mx-auto w-full max-w-[430px] border-t border-white/10 bg-black/78 px-2 pb-[max(0.55rem,env(safe-area-inset-bottom))] pt-2 backdrop-blur-xl";
  const listClassName = wide ? "grid grid-cols-8 gap-1 md:flex md:flex-1 md:flex-col md:gap-2" : "grid grid-cols-8 gap-1";

  return (
    <nav className={navClassName}>
      {wide ? <div className="mb-5 hidden px-2 text-lg font-black md:block">Pulse</div> : null}
      <div className={listClassName}>
        {items.map(({ to, label, icon: Icon }) => {
          const unread = label === "消息" ? messageUnread : label === "通知" ? notificationUnread : 0;
          return (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                [
                  "flex flex-col items-center gap-1 rounded-lg px-1 py-1.5 text-[0.68rem] font-semibold transition md:px-2 md:py-3",
                  isActive ? "text-white" : "text-white/52",
                ].join(" ")
              }
            >
              {({ isActive }) => (
                <>
                  <span className="relative">
                    <Icon className={["h-5 w-5", isActive ? "text-pulse-cyan" : ""].join(" ")} />
                    {unread > 0 ? (
                      <span className="absolute -right-2.5 -top-2 rounded-lg bg-pulse-red px-1.5 py-0.5 text-[0.62rem] font-black leading-none text-white">
                        {unread > 99 ? "99+" : unread}
                      </span>
                    ) : null}
                  </span>
                  <span className="md:text-[0.72rem]">{label}</span>
                </>
              )}
            </NavLink>
          );
        })}
      </div>
    </nav>
  );
}
