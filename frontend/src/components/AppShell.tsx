import { Outlet, useLocation } from "react-router-dom";
import { BottomNav } from "./BottomNav";
import { AuthModal } from "./AuthModal";

export function AppShell() {
  const location = useLocation();
  const usesWideShell =
    location.pathname.startsWith("/feed") ||
    location.pathname.startsWith("/messages") ||
    location.pathname.startsWith("/profile");

  return (
    <div className="min-h-screen bg-pulse-black text-white">
      <div
        className={
          usesWideShell
            ? "min-h-screen w-full overflow-hidden bg-black"
            : "mx-auto min-h-screen w-full max-w-[430px] overflow-hidden border-x border-white/10 bg-black shadow-glow"
        }
      >
        <Outlet />
        <BottomNav wide={usesWideShell} />
        <AuthModal />
      </div>
    </div>
  );
}
