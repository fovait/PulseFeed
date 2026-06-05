import { API_BASE_URL } from "../api/client";
import type { FeedVideo } from "../types/api";

export function videoAuthor(video: FeedVideo) {
  return {
    id: video.author?.id || video.author_id || 0,
    username: video.author?.username || video.username || "unknown",
  };
}

export function resolveMediaUrl(url?: string) {
  if (!url) return "";
  if (/^https?:\/\//i.test(url) || url.startsWith("blob:") || url.startsWith("data:")) {
    return url;
  }
  return `${API_BASE_URL}${url.startsWith("/") ? url : `/${url}`}`;
}

export function normalizeVideo(video: FeedVideo): FeedVideo {
  return {
    ...video,
    likes_count: Number(video.likes_count || 0),
    comments_count: Number(video.comments_count || 0),
    is_liked: Boolean(video.is_liked),
    play_url: resolveMediaUrl(video.play_url),
    cover_url: resolveMediaUrl(video.cover_url),
  };
}
