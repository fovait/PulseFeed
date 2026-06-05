import { apiClient } from "./client";
import type {
  Account,
  Comment,
  EventType,
  FeedVideo,
  LikesCursor,
  LikesFeedResponse,
  Message,
  MessageConversationListResponse,
  MessageListResponse,
  NotificationListResponse,
  PopularityCursor,
  PopularityFeedResponse,
  ProfileResponse,
  RecommendResponse,
  ReportResponse,
  Session,
  TimeFeedResponse,
  ChunkStatusResponse,
  CompleteChunkUploadResponse,
  InitChunkUploadResponse,
  UploadURLResponse,
  VideoDetailsResponse,
} from "../types/api";

export const pulsefeedApi = {
  register(username: string, password: string) {
    return apiClient.request<{ message: string }>("/account/register", {
      auth: false,
      body: { username, password },
    });
  },

  login(username: string, password: string) {
    return apiClient.request<Session>("/account/login", {
      auth: false,
      body: { username, password },
    });
  },

  getProfile(accountID: number) {
    return apiClient.request<ProfileResponse>("/account/getProfile", {
      auth: false,
      body: { account_id: accountID },
    });
  },

  findAccountByID(id: number) {
    return apiClient.request<Account>("/account/findByID", {
      auth: false,
      body: { id },
    });
  },

  listLatest(limit: number, beforeTime = 0) {
    return apiClient.request<TimeFeedResponse>("/feed/listLatest", {
      body: { limit, before_time: beforeTime },
    });
  },

  listFollowing(limit: number, beforeTime = 0) {
    return apiClient.request<TimeFeedResponse>("/feed/listByFollowing", {
      body: { limit, before_time: beforeTime },
    });
  },

  listPopularity(limit: number, cursor?: PopularityCursor) {
    return apiClient.request<PopularityFeedResponse>("/feed/listByPopularity", {
      body: { limit, cursor },
    });
  },

  listLikes(limit: number, cursor?: LikesCursor) {
    return apiClient.request<LikesFeedResponse>("/feed/listLikesCount", {
      body: { limit, cursor },
    });
  },

  recommend(limit: number, cursor = "", debug = true) {
    return apiClient.request<RecommendResponse>("/feed/recommend", {
      body: { limit, cursor, debug },
    });
  },

  getVideoDetail(id: number) {
    return apiClient.request<FeedVideo>("/video/getDetail", {
      auth: false,
      body: { id },
    });
  },

  listVideoDetails(ids: number[]) {
    return apiClient.request<VideoDetailsResponse>("/video/listDetails", {
      auth: false,
      body: { ids },
    });
  },

  uploadVideoFile(formData: FormData) {
    return apiClient.upload<UploadURLResponse>("/video/uploadVideo", formData);
  },

  uploadCoverFile(formData: FormData) {
    return apiClient.upload<UploadURLResponse>("/video/uploadCover", formData);
  },

  initChunkUpload(payload: {
    filename: string;
    file_size: number;
    chunk_size: number;
    total_chunks: number;
    file_hash: string;
  }) {
    return apiClient.request<InitChunkUploadResponse>("/video/chunk/init", {
      body: payload,
    });
  },

  uploadChunk(formData: FormData) {
    return apiClient.upload<{ chunk_index: number }>("/video/chunk/upload", formData);
  },

  chunkStatus(uploadID: string) {
    return apiClient.request<ChunkStatusResponse>("/video/chunk/status", {
      body: { upload_id: uploadID },
    });
  },

  completeChunkUpload(uploadID: string) {
    return apiClient.request<CompleteChunkUploadResponse>("/video/chunk/complete", {
      body: { upload_id: uploadID },
    });
  },

  publishVideo(payload: Pick<FeedVideo, "title" | "description" | "play_url" | "cover_url">) {
    return apiClient.request<FeedVideo>("/video/publish", {
      body: payload,
    });
  },

  likeVideo(videoID: number) {
    return apiClient.request<{ message: string }>("/like/like", {
      body: { video_id: videoID },
    });
  },

  unlikeVideo(videoID: number) {
    return apiClient.request<{ message: string }>("/like/unlike", {
      body: { video_id: videoID },
    });
  },

  isLiked(videoID: number) {
    return apiClient.request<{ is_liked: boolean }>("/like/isLiked", {
      body: { video_id: videoID },
    });
  },

  listComments(videoID: number) {
    return apiClient.request<Comment[]>("/comment/listAll", {
      auth: false,
      body: { video_id: videoID },
    });
  },

  publishComment(videoID: number, content: string) {
    return apiClient.request<{ message: string }>("/comment/publish", {
      body: { video_id: videoID, content },
    });
  },

  deleteComment(commentID: number) {
    return apiClient.request<{ message: string }>("/comment/delete", {
      body: { comment_id: commentID },
    });
  },

  follow(vloggerID: number) {
    return apiClient.request<{ message: string }>("/social/follow", {
      body: { vlogger_id: vloggerID },
    });
  },

  unfollow(vloggerID: number) {
    return apiClient.request<{ message: string }>("/social/unfollow", {
      body: { vlogger_id: vloggerID },
    });
  },

  isFollowing(vloggerID: number) {
    return apiClient.request<{ is_followed: boolean }>("/social/isFollowed", {
      body: { vlogger_id: vloggerID },
    });
  },

  sendMessage(toID: number, content: string) {
    return apiClient.request<Message>("/message/send", {
      body: { to_id: toID, content },
    });
  },

  listMessages(peerID: number, limit = 20, beforeID = 0) {
    return apiClient.request<MessageListResponse>("/message/list", {
      body: { peer_id: peerID, limit, before_id: beforeID },
    });
  },

  listMessageConversations(limit = 50) {
    return apiClient.request<MessageConversationListResponse>("/message/conversations", {
      body: { limit },
    });
  },

  listNotifications() {
    return apiClient.request<NotificationListResponse>("/notification/list", {
      method: "GET",
    });
  },

  unreadCount() {
    return apiClient.request<{ count: number }>("/notification/unreadCount", {
      method: "GET",
    });
  },

  markNotificationRead(id?: number) {
    return apiClient.request<{ message: string }>("/notification/markRead", {
      body: id ? { id } : {},
    });
  },

  report(targetType: "video" | "comment", targetID: number, reason: string) {
    return apiClient.request<ReportResponse>("/moderation/report", {
      body: { target_type: targetType, target_id: targetID, reason },
    });
  },

  track(videoID: number, type: EventType, idempotencyKey: string) {
    return apiClient.request<{ event: unknown }>("/event/track", {
      body: { video_id: videoID, type, idempotency_key: idempotencyKey },
    });
  },
};
