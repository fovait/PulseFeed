import { ArrowLeft, CheckCircle2, EyeOff, RefreshCw, ShieldAlert, ThumbsDown, ThumbsUp } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";
import type { ContentReport, ModerationStatus } from "../types/api";
import { formatRelativeTime } from "../utils/time";

const STATUS_TABS: { value: ModerationStatus | "all"; label: string }[] = [
  { value: "pending", label: "待审核" },
  { value: "approved", label: "已通过" },
  { value: "rejected", label: "已驳回" },
  { value: "hidden", label: "已隐藏" },
  { value: "all", label: "全部" },
];

const STATUS_TEXT: Record<ModerationStatus, string> = {
  pending: "待审核",
  approved: "已通过",
  rejected: "已驳回",
  hidden: "已隐藏",
};

const STATUS_COLOR: Record<ModerationStatus, string> = {
  pending: "bg-yellow-500/20 text-yellow-200",
  approved: "bg-emerald-500/20 text-emerald-200",
  rejected: "bg-rose-500/20 text-rose-200",
  hidden: "bg-zinc-500/20 text-zinc-200",
};

export function AdminModerationPage() {
  const navigate = useNavigate();
  const { session, openAuth } = useAuth();
  const { pushToast } = useToast();

  const [authChecked, setAuthChecked] = useState(false);
  const [isAdmin, setIsAdmin] = useState(false);
  const [filter, setFilter] = useState<ModerationStatus | "all">("pending");
  const [reports, setReports] = useState<ContentReport[]>([]);
  const [loading, setLoading] = useState(false);
  const [actingID, setActingID] = useState<number | null>(null);
  const [noteByID, setNoteByID] = useState<Record<number, string>>({});

  useEffect(() => {
    if (!session?.token) {
      setAuthChecked(true);
      setIsAdmin(false);
      return;
    }
    pulsefeedApi
      .isModerationAdmin()
      .then((resp) => {
        setIsAdmin(resp.is_admin);
        setAuthChecked(true);
      })
      .catch(() => {
        setIsAdmin(false);
        setAuthChecked(true);
      });
  }, [session?.token]);

  const load = useCallback(async () => {
    if (!isAdmin) return;
    setLoading(true);
    try {
      const resp = await pulsefeedApi.listModerationReports(filter === "all" ? undefined : filter);
      setReports(resp.reports || []);
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "加载失败", "error");
    } finally {
      setLoading(false);
    }
  }, [filter, isAdmin, pushToast]);

  useEffect(() => {
    load();
  }, [load]);

  async function decide(report: ContentReport, status: ModerationStatus) {
    setActingID(report.id);
    try {
      await pulsefeedApi.reviewModerationReport(report.id, status, noteByID[report.id] || "");
      pushToast(`已${STATUS_TEXT[status]}`, "success");
      // 待审核 tab 下：处理完直接移出列表
      if (filter === "pending") {
        setReports((items) => items.filter((r) => r.id !== report.id));
      } else {
        await load();
      }
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "操作失败", "error");
    } finally {
      setActingID(null);
    }
  }

  function targetLink(report: ContentReport): string {
    if (report.target_type === "video") return `/video/${report.target_id}`;
    return "#";
  }

  if (!session?.token) {
    return (
      <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
        <div className="mx-auto w-full max-w-[820px]">
          <h1 className="text-2xl font-black md:text-3xl">审核后台</h1>
          <section className="glass-panel mt-6 rounded-lg p-6">
            <ShieldAlert className="h-12 w-12 text-white/40" />
            <p className="mt-4 text-sm text-white/58">登录后才能访问</p>
            <button className="primary-button mt-4" onClick={() => openAuth("登录后查看审核后台")}>
              登录 / 注册
            </button>
          </section>
        </div>
      </main>
    );
  }

  if (authChecked && !isAdmin) {
    return (
      <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
        <div className="mx-auto w-full max-w-[820px]">
          <button onClick={() => navigate("/feed/recommend")} className="mb-4 flex items-center gap-1 text-sm text-white/60 hover:text-white">
            <ArrowLeft className="h-4 w-4" /> 返回主页
          </button>
          <h1 className="text-2xl font-black md:text-3xl">审核后台</h1>
          <section className="glass-panel mt-6 rounded-lg p-6 text-sm text-white/60">
            你不在管理员白名单内。请联系运维把账号 ID #{session.account_id} 加入 <code>MODERATION_ADMIN_IDS</code>。
          </section>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto w-full max-w-[920px]">
        <button onClick={() => navigate("/feed/recommend")} className="mb-4 flex items-center gap-1 text-sm text-white/60 hover:text-white">
          <ArrowLeft className="h-4 w-4" /> 返回主页
        </button>
        <header className="mb-5 flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-black md:text-3xl">审核后台</h1>
            <p className="mt-1 text-sm text-white/52">处理用户举报，{STATUS_TEXT[filter as ModerationStatus] || "全部"} {reports.length} 条</p>
          </div>
          <button onClick={() => load()} disabled={loading} className="ghost-button flex items-center gap-1">
            <RefreshCw className={["h-4 w-4", loading ? "animate-spin" : ""].join(" ")} />
            刷新
          </button>
        </header>

        <div className="mb-4 flex flex-wrap gap-1 border-b border-white/10">
          {STATUS_TABS.map((tab) => (
            <button
              key={tab.value}
              onClick={() => setFilter(tab.value)}
              className={[
                "px-3 py-2 text-sm font-bold transition",
                filter === tab.value ? "border-b-2 border-pulse-cyan text-white" : "text-white/52 hover:text-white",
              ].join(" ")}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {loading && reports.length === 0 ? (
          <p className="text-sm text-white/52">加载中...</p>
        ) : reports.length === 0 ? (
          <div className="glass-panel rounded-lg p-6 text-center text-sm text-white/52">没有匹配的举报</div>
        ) : (
          <ul className="space-y-3">
            {reports.map((report) => {
              const isPending = report.status === "pending";
              const target = targetLink(report);
              return (
                <li key={report.id} className="glass-panel rounded-lg p-4">
                  <div className="mb-3 flex flex-wrap items-center gap-2 text-xs">
                    <span className={["rounded-lg px-2 py-0.5 font-bold", STATUS_COLOR[report.status]].join(" ")}>
                      {STATUS_TEXT[report.status]}
                    </span>
                    <span className="text-white/52">#{report.id}</span>
                    <span className="text-white/52">举报者 #{report.reporter_id}</span>
                    {report.reviewer_id ? <span className="text-white/52">审核者 #{report.reviewer_id}</span> : null}
                    <span className="text-white/42">{formatRelativeTime(report.created_at)}</span>
                  </div>

                  <div className="mb-3 flex items-center gap-3">
                    {target !== "#" ? (
                      <Link to={target} className="text-sm font-bold text-pulse-cyan hover:underline">
                        {report.target_type === "video" ? `视频 #${report.target_id}` : `评论 #${report.target_id}`}
                      </Link>
                    ) : (
                      <span className="text-sm font-bold text-white/72">
                        {report.target_type === "video" ? `视频 #${report.target_id}` : `评论 #${report.target_id}`}
                      </span>
                    )}
                  </div>

                  <p className="mb-3 rounded-lg bg-white/[0.06] p-3 text-sm">{report.reason}</p>

                  {report.review_note ? (
                    <p className="mb-3 text-xs text-white/52">
                      审核备注: <span className="text-white/72">{report.review_note}</span>
                    </p>
                  ) : null}

                  {isPending ? (
                    <div className="space-y-2">
                      <input
                        className="control-field"
                        placeholder="审核备注（可选）"
                        value={noteByID[report.id] || ""}
                        onChange={(e) => setNoteByID((prev) => ({ ...prev, [report.id]: e.target.value }))}
                        maxLength={255}
                      />
                      <div className="flex flex-wrap gap-2">
                        <button
                          disabled={actingID === report.id}
                          onClick={() => decide(report, "approved")}
                          className="primary-button flex items-center gap-1.5 disabled:opacity-60"
                        >
                          <ThumbsUp className="h-4 w-4" /> 通过
                        </button>
                        <button
                          disabled={actingID === report.id}
                          onClick={() => decide(report, "rejected")}
                          className="ghost-button flex items-center gap-1.5 disabled:opacity-60"
                        >
                          <ThumbsDown className="h-4 w-4" /> 驳回
                        </button>
                        <button
                          disabled={actingID === report.id}
                          onClick={() => decide(report, "hidden")}
                          className="ghost-button flex items-center gap-1.5 border-rose-500/40 text-rose-200 disabled:opacity-60"
                        >
                          <EyeOff className="h-4 w-4" /> 隐藏内容
                        </button>
                      </div>
                    </div>
                  ) : (
                    <p className="text-xs text-white/42 flex items-center gap-1">
                      <CheckCircle2 className="h-3 w-3" />
                      已于 {formatRelativeTime(report.reviewed_at)} 处理
                    </p>
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
