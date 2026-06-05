import { useState, type FormEvent } from "react";
import { Flag, X } from "lucide-react";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";

export type ReportTarget = {
  type: "video" | "comment";
  id: number;
  title?: string;
};

export function ReportDialog({
  target,
  onClose,
}: {
  target: ReportTarget | null;
  onClose: () => void;
}) {
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const { requireAuth } = useAuth();
  const { pushToast } = useToast();

  if (!target) return null;

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!target) return;
    if (!requireAuth("登录后才能举报内容")) return;
    setSubmitting(true);
    try {
      await pulsefeedApi.report(target.type, target.id, reason.trim());
      pushToast("举报已提交", "success");
      setReason("");
      onClose();
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "举报失败", "error");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-[65] flex items-end justify-center bg-black/60 px-4 pb-5 backdrop-blur-sm">
      <form className="glass-panel w-full max-w-[398px] rounded-lg p-4" onSubmit={submit}>
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Flag className="h-5 w-5 text-pulse-red" />
            <h2 className="text-lg font-black">举报内容</h2>
          </div>
          <button type="button" className="rounded-lg p-2 text-white/70 hover:bg-white/10" onClick={onClose}>
            <X className="h-5 w-5" />
          </button>
        </div>
        <p className="mb-3 text-sm text-white/62">
          {target.title || `${target.type} #${target.id}`}
        </p>
        <textarea
          className="control-field min-h-28 resize-none"
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          maxLength={255}
          placeholder="请输入举报原因"
          required
        />
        <button className="primary-button mt-3 w-full disabled:opacity-60" disabled={submitting || !reason.trim()}>
          {submitting ? "提交中..." : "提交举报"}
        </button>
      </form>
    </div>
  );
}
