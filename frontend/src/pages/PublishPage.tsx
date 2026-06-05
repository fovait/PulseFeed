import { CheckCircle2, Link2, UploadCloud } from "lucide-react";
import { useState, type ChangeEvent, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";
import { uploadVideoInChunks, type UploadProgress } from "../utils/upload";

export function PublishPage() {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [playUrl, setPlayUrl] = useState("");
  const [coverUrl, setCoverUrl] = useState("");
  const [advancedUrlMode, setAdvancedUrlMode] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [uploadingVideo, setUploadingVideo] = useState(false);
  const [uploadingCover, setUploadingCover] = useState(false);
  const [uploadProgress, setUploadProgress] = useState<UploadProgress | null>(null);
  const { requireAuth } = useAuth();
  const { pushToast } = useToast();
  const navigate = useNavigate();
  const canSubmit =
    Boolean(title.trim()) && Boolean(playUrl.trim()) && Boolean(coverUrl.trim()) && !uploadingVideo && !uploadingCover && !submitting;

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!requireAuth("登录后才能发布视频")) return;
    if (!title.trim() || !playUrl.trim() || !coverUrl.trim()) {
      pushToast("请先填写标题并上传视频和封面", "error");
      return;
    }
    setSubmitting(true);
    try {
      await pulsefeedApi.publishVideo({
        title: title.trim(),
        description: description.trim(),
        play_url: playUrl.trim(),
        cover_url: coverUrl.trim(),
      });
      pushToast("视频已发布", "success");
      navigate("/feed/latest");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "发布失败", "error");
    } finally {
      setSubmitting(false);
    }
  }

  async function uploadVideoFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (!requireAuth("登录后才能上传视频")) return;
    if (!file.name.toLowerCase().endsWith(".mp4")) {
      pushToast("后端当前只允许 .mp4 视频", "error");
      return;
    }
    setUploadingVideo(true);
    setUploadProgress(null);
    try {
      if (file.size > 20 * 1024 * 1024) {
        const url = await uploadVideoInChunks(file, setUploadProgress);
        setPlayUrl(url);
      } else {
        const formData = new FormData();
        formData.append("file", file);
        const response = await pulsefeedApi.uploadVideoFile(formData);
        setPlayUrl(response.play_url || response.url);
      }
      pushToast("视频上传完成", "success");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "视频上传失败", "error");
    } finally {
      setUploadingVideo(false);
    }
  }

  async function uploadCoverFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (!requireAuth("登录后才能上传封面")) return;
    setUploadingCover(true);
    try {
      const formData = new FormData();
      formData.append("file", file);
      const response = await pulsefeedApi.uploadCoverFile(formData);
      setCoverUrl(response.cover_url || response.url);
      pushToast("封面上传完成", "success");
    } catch (error) {
      pushToast(error instanceof Error ? error.message : "封面上传失败", "error");
    } finally {
      setUploadingCover(false);
    }
  }

  return (
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5">
      <header className="mb-5">
        <h1 className="text-2xl font-black">发布视频</h1>
        <p className="mt-1 text-sm text-white/52">上传视频和封面后发布。</p>
      </header>

      <form className="glass-panel space-y-4 rounded-lg p-4" onSubmit={submit}>
        <div className="grid grid-cols-2 gap-2">
          <label className="ghost-button flex cursor-pointer items-center justify-center gap-2 text-center">
            <UploadCloud className="h-4 w-4 text-pulse-cyan" />
            {uploadingVideo ? "上传中..." : "选择视频"}
            <input className="sr-only" type="file" accept="video/mp4" onChange={uploadVideoFile} disabled={uploadingVideo} />
          </label>
          <label className="ghost-button flex cursor-pointer items-center justify-center gap-2 text-center">
            <UploadCloud className="h-4 w-4 text-pulse-cyan" />
            {uploadingCover ? "上传中..." : "选择封面"}
            <input className="sr-only" type="file" accept=".jpg,.jpeg,.png,.webp,image/*" onChange={uploadCoverFile} disabled={uploadingCover} />
          </label>
        </div>
        <div className="grid gap-2">
          <UploadStatus label="视频" uploading={uploadingVideo} url={playUrl} />
          <UploadStatus label="封面" uploading={uploadingCover} url={coverUrl} />
        </div>
        {uploadProgress ? (
          <div className="rounded-lg bg-white/[0.06] p-3 text-sm text-white/72">
            <div className="mb-2 flex justify-between text-xs font-bold uppercase tracking-[0.08em] text-white/52">
              <span>{uploadProgress.phase === "hashing" ? "计算校验" : uploadProgress.phase === "uploading" ? "上传分片" : "上传完成"}</span>
              <span>{uploadProgress.completed}/{uploadProgress.total}</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-white/10">
              <div
                className="h-full bg-pulse-cyan"
                style={{ width: `${Math.round((uploadProgress.completed / uploadProgress.total) * 100)}%` }}
              />
            </div>
          </div>
        ) : null}
        <label className="block">
          <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">标题</span>
          <input className="control-field" value={title} onChange={(event) => setTitle(event.target.value)} required />
        </label>
        <label className="block">
          <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">描述</span>
          <textarea
            className="control-field min-h-24 resize-none"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
          />
        </label>
        <section className="overflow-hidden rounded-lg border border-white/10 bg-white/[0.04]">
          <button
            className="flex w-full items-center justify-between px-3 py-2.5 text-left text-sm font-bold text-white/78"
            type="button"
            onClick={() => setAdvancedUrlMode((enabled) => !enabled)}
          >
            <span className="flex items-center gap-2">
              <Link2 className="h-4 w-4 text-pulse-cyan" />
              URL 发布
            </span>
            <span className="text-xs text-white/42">{advancedUrlMode ? "收起" : "高级"}</span>
          </button>
          {advancedUrlMode ? (
            <div className="space-y-3 border-t border-white/10 p-3">
              <label className="block">
                <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">play_url</span>
                <input className="control-field" value={playUrl} onChange={(event) => setPlayUrl(event.target.value)} />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">cover_url</span>
                <input className="control-field" value={coverUrl} onChange={(event) => setCoverUrl(event.target.value)} />
              </label>
            </div>
          ) : null}
        </section>
        <button className="primary-button w-full disabled:opacity-60" disabled={!canSubmit}>
          {submitting ? "发布中..." : "发布"}
        </button>
      </form>
    </main>
  );
}

function UploadStatus({ label, uploading, url }: { label: string; uploading: boolean; url: string }) {
  return (
    <div className="flex min-h-12 items-center justify-between gap-3 rounded-lg bg-white/[0.06] px-3 py-2 text-sm">
      <div className="min-w-0">
        <div className="font-bold text-white/86">{label}</div>
        <div className="truncate text-xs text-white/42">{url ? compactUrl(url) : uploading ? "上传中..." : "待上传"}</div>
      </div>
      {url ? (
        <span className="inline-flex shrink-0 items-center gap-1 text-xs font-bold text-pulse-cyan">
          <CheckCircle2 className="h-4 w-4" />
          已生成
        </span>
      ) : null}
    </div>
  );
}

function compactUrl(url: string) {
  if (url.length <= 56) return url;
  return `${url.slice(0, 26)}...${url.slice(-22)}`;
}
