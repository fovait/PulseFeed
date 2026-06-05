import { Outlet, useLocation } from "react-router-dom";
import { BottomNav } from "./BottomNav";
import { AuthModal } from "./AuthModal";
import { useAuth } from "../hooks/useAuth";
import { useUnreadSummary } from "../hooks/useUnreadSummary";

export function AppShell() {
  const location = useLocation();
  const { session } = useAuth();
  const unread = useUnreadSummary(session);
  const usesWideShell =
    location.pathname.startsWith("/feed") ||
    location.pathname.startsWith("/messages") ||
    location.pathname.startsWith("/profile") ||
    location.pathname.startsWith("/publish");

  return (
    <div className="min-h-screen bg-pulse-black text-white">
      <div
        className={
          usesWideShell
            ? "min-h-screen w-full bg-black"
            : "mx-auto min-h-screen w-full max-w-[430px] overflow-hidden border-x border-white/10 bg-black shadow-glow"
        }
      >
        <Outlet />
        <BottomNav wide={usesWideShell} messageUnread={unread.totalUnread} />
        <AuthModal />
      </div>
    </div>
  );
}
