import { useState, type FormEvent } from "react";
import { X } from "lucide-react";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";

export function AuthModal() {
  const { authOpen, authMode, authReason, closeAuth, login, register, setAuthMode } = useAuth();
  const { pushToast } = useToast();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);

  if (!authOpen) return null;

  const isRegister = authMode === "register";

  async function submit(event: FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    try {
      if (isRegister) {
        await register(username.trim(), password);
      } else {
        await login(username.trim(), password);
      }
      setPassword("");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "认证失败", "error");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-[70] flex items-end justify-center bg-black/60 px-4 pb-5 backdrop-blur-sm">
      <div className="glass-panel w-full max-w-[398px] rounded-lg p-4">
        <div className="mb-4 flex items-start justify-between gap-4">
          <div>
            <h2 className="text-xl font-black">{isRegister ? "创建账号" : "登录 PulseFeed"}</h2>
            <p className="mt-1 text-sm text-white/62">{authReason}</p>
          </div>
          <button type="button" className="rounded-lg p-2 text-white/70 hover:bg-white/10" onClick={closeAuth}>
            <X className="h-5 w-5" />
          </button>
        </div>

        <form className="space-y-3" onSubmit={submit}>
          <label className="block text-xs font-bold uppercase tracking-[0.08em] text-white/52">用户名</label>
          <input
            className="control-field"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            autoComplete="username"
            required
          />
          <label className="block text-xs font-bold uppercase tracking-[0.08em] text-white/52">密码</label>
          <input
            className="control-field"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete={isRegister ? "new-password" : "current-password"}
            required
          />
          <button className="primary-button w-full disabled:opacity-60" disabled={submitting}>
            {submitting ? "提交中..." : isRegister ? "注册并登录" : "登录"}
          </button>
        </form>

        <button
          type="button"
          className="mt-4 w-full text-center text-sm font-semibold text-pulse-cyan"
          onClick={() => setAuthMode(isRegister ? "login" : "register")}
        >
          {isRegister ? "已有账号，去登录" : "没有账号，注册一个"}
        </button>
      </div>
    </div>
  );
}
