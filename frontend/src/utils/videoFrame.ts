/**
 * 抓取视频指定时间点的画面，返回 PNG/JPEG Blob。
 * 默认抓第一帧（0.1s，0 容易抓到全黑）。
 */
export async function captureVideoFrame(
  videoUrl: string,
  options: { time?: number; mimeType?: string; quality?: number } = {},
): Promise<Blob> {
  const { time = 0.1, mimeType = "image/jpeg", quality = 0.92 } = options;

  return new Promise<Blob>((resolve, reject) => {
    const video = document.createElement("video");
    video.crossOrigin = "anonymous";
    video.preload = "auto";
    video.muted = true;
    video.playsInline = true;
    video.src = videoUrl;

    const cleanup = () => {
      video.removeAttribute("src");
      video.load();
    };

    video.addEventListener(
      "loadedmetadata",
      () => {
        const seekTo = Math.min(time, Math.max(0, video.duration - 0.05));
        video.currentTime = isFinite(seekTo) ? seekTo : 0;
      },
      { once: true },
    );

    video.addEventListener(
      "seeked",
      () => {
        try {
          const canvas = document.createElement("canvas");
          canvas.width = video.videoWidth || 720;
          canvas.height = video.videoHeight || 1280;
          const ctx = canvas.getContext("2d");
          if (!ctx) {
            cleanup();
            reject(new Error("canvas 2d 不可用"));
            return;
          }
          ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
          canvas.toBlob(
            (blob) => {
              cleanup();
              if (!blob) {
                reject(new Error("抓取首帧失败"));
                return;
              }
              resolve(blob);
            },
            mimeType,
            quality,
          );
        } catch (err) {
          cleanup();
          reject(err instanceof Error ? err : new Error(String(err)));
        }
      },
      { once: true },
    );

    video.addEventListener(
      "error",
      () => {
        cleanup();
        reject(new Error("视频加载失败，无法抓取首帧"));
      },
      { once: true },
    );
  });
}
