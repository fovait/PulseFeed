import { CheckCircle2, ImagePlus, Link2, PlayCircle, Video } from "lucide-react";
import { useState, type ChangeEvent, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { pulsefeedApi } from "../api/pulsefeed";
import { useAuth } from "../hooks/useAuth";
import { useToast } from "../hooks/useToast";
import { uploadVideoInChunks, type UploadProgress } from "../utils/upload";
import { captureVideoFrame } from "../utils/videoFrame";

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
    Boolean(title.trim()) && Boolean(playUrl.trim()) && !uploadingVideo && !uploadingCover && !submitting;

  async function ensureCover(): Promise<string> {
    if (coverUrl.trim()) return coverUrl.trim();
    if (!playUrl.trim()) throw new Error("请先上传视频");
    setUploadingCover(true);
    try {
      const blob = await captureVideoFrame(playUrl.trim());
      const formData = new FormData();
      formData.append("file", blob, "cover.jpg");
      const response = await pulsefeedApi.uploadCoverFile(formData);
      const next = response.cover_url || response.url;
      setCoverUrl(next);
      pushToast("已自动生成封面（首帧）", "success");
      return next;
    } finally {
      setUploadingCover(false);
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!requireAuth("登录后才能发布视频")) return;
    if (!title.trim() || !playUrl.trim()) {
      pushToast("请先填写标题并上传视频", "error");
      return;
    }
    setSubmitting(true);
    try {
      let finalCover = coverUrl.trim();
      if (!finalCover) {
        try {
          finalCover = await ensureCover();
        } catch (err) {
          pushToast(err instanceof Error ? err.message : "自动封面生成失败", "error");
          setSubmitting(false);
          return;
        }
      }
      await pulsefeedApi.publishVideo({
        title: title.trim(),
        description: description.trim(),
        play_url: playUrl.trim(),
        cover_url: finalCover,
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
    <main className="min-h-[100svh] bg-pulse-black px-4 pb-28 pt-5 md:pl-28 md:pr-8 md:pt-8">
      <div className="mx-auto w-full max-w-[1180px]">
        <header className="mb-5 flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
          <div>
            <h1 className="text-2xl font-black md:text-3xl">发布视频</h1>
            <p className="mt-1 text-sm text-white/52">上传视频、封面、标题和描述，发布参数由后端生成。</p>
          </div>
          <button className="primary-button hidden min-w-32 disabled:opacity-60 md:block" form="publish-form" disabled={!canSubmit}>
            {submitting ? "发布中..." : "发布"}
          </button>
        </header>

        <form id="publish-form" className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_390px]" onSubmit={submit}>
          <section className="glass-panel rounded-lg p-4 md:p-5">
            <div className="grid gap-4">
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="ghost-button flex min-h-14 cursor-pointer items-center justify-center gap-2 text-center">
                  <Video className="h-4 w-4 text-pulse-cyan" />
                  {uploadingVideo ? "上传中..." : "选择视频"}
                  <input className="sr-only" type="file" accept="video/mp4" onChange={uploadVideoFile} disabled={uploadingVideo} />
                </label>
                <label className="ghost-button flex min-h-14 cursor-pointer items-center justify-center gap-2 text-center">
                  <ImagePlus className="h-4 w-4 text-pulse-cyan" />
                  {uploadingCover ? "上传中..." : "选择封面 (可选)"}
                  <input className="sr-only" type="file" accept=".jpg,.jpeg,.png,.webp,image/*" onChange={uploadCoverFile} disabled={uploadingCover} />
                </label>
              </div>
              {!coverUrl && playUrl ? (
                <p className="text-xs text-white/52">
                  未上传封面时，会自动抓取视频第一帧作为封面。也可点
                  <button type="button" className="ml-1 text-pulse-cyan underline" onClick={() => ensureCover().catch((err) => pushToast(err instanceof Error ? err.message : "抓取失败", "error"))} disabled={uploadingCover}>
                    立即生成
                  </button>
                  预览。
                </p>
              ) : null}

              {uploadProgress ? (
                <UploadProgressMeter progress={uploadProgress} />
              ) : null}

              <label className="block">
                <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">标题</span>
                <input className="control-field" value={title} onChange={(event) => setTitle(event.target.value)} required maxLength={120} />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-bold uppercase tracking-[0.08em] text-white/52">描述</span>
                <textarea
                  className="control-field min-h-36 resize-none"
                  value={description}
                  onChange={(event) => setDescription(event.target.value)}
                  maxLength={1000}
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

              <button className="primary-button w-full disabled:opacity-60 md:hidden" disabled={!canSubmit}>
                {submitting ? "发布中..." : "发布"}
              </button>
            </div>
          </section>

          <aside className="space-y-4">
            <MediaPreview playUrl={playUrl} coverUrl={coverUrl} title={title} />
            <section className="glass-panel rounded-lg p-4">
              <h2 className="font-black">上传状态</h2>
              <div className="mt-3 grid gap-2">
                <UploadStatus label="视频" uploading={uploadingVideo} url={playUrl} />
                <UploadStatus label="封面 (可选)" uploading={uploadingCover} url={coverUrl} />
              </div>
            </section>
          </aside>
        </form>
      </div>
    </main>
  );
}

function MediaPreview({ playUrl, coverUrl, title }: { playUrl: string; coverUrl: string; title: string }) {
  return (
    <section className="glass-panel rounded-lg p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="font-black">预览</h2>
        <span className="text-xs font-bold text-white/42">9:16</span>
      </div>
      <div className="mx-auto aspect-[9/16] w-full max-w-[320px] overflow-hidden rounded-lg bg-black">
        {playUrl ? (
          <video className="h-full w-full object-contain" src={playUrl} poster={coverUrl} controls playsInline preload="metadata" />
        ) : coverUrl ? (
          <img className="h-full w-full object-cover" src={coverUrl} alt={title || "封面预览"} />
        ) : (
          <div className="grid h-full place-items-center bg-white/[0.04] px-8 text-center">
            <div>
              <PlayCircle className="mx-auto h-12 w-12 text-white/32" />
              <p className="mt-3 text-sm font-bold text-white/58">上传视频后预览</p>
            </div>
          </div>
        )}
      </div>
      <p className="mt-3 truncate text-sm font-bold text-white/82">{title.trim() || "未命名视频"}</p>
    </section>
  );
}

function UploadProgressMeter({ progress }: { progress: UploadProgress }) {
  const percent = progress.total ? Math.round((progress.completed / progress.total) * 100) : 0;

  return (
    <div className="rounded-lg bg-white/[0.06] p-3 text-sm text-white/72">
      <div className="mb-2 flex justify-between text-xs font-bold uppercase tracking-[0.08em] text-white/52">
        <span>{progress.phase === "hashing" ? "计算校验" : progress.phase === "uploading" ? "上传分片" : "上传完成"}</span>
        <span>{progress.completed}/{progress.total}</span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-white/10">
        <div className="h-full bg-pulse-cyan" style={{ width: `${percent}%` }} />
      </div>
    </div>
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
