export type ID = number;

export type Account = {
  id: ID;
  username: string;
  avatar_url?: string;
  bio?: string;
};

export type Session = {
  token: string;
  refresh_token?: string;
  account_id: ID;
  username: string;
};

export type FeedAuthor = {
  id: ID;
  username: string;
};

export type FeedVideo = {
  id: ID;
  author?: FeedAuthor;
  author_id?: ID;
  username?: string;
  title: string;
  description?: string;
  play_url: string;
  cover_url: string;
  create_time?: number | string;
  created_at?: string;
  likes_count: number;
  comments_count: number;
  popularity?: number;
  is_liked?: boolean;
  is_following?: boolean;
  recommendation?: RankedVideo;
};

export type VideoDetailsResponse = {
  videos: FeedVideo[];
};

export type TimeFeedResponse = {
  video_list: FeedVideo[];
  next_before_time: number;
  has_more: boolean;
};

export type LikesCursor = {
  likes_count: number;
  id: ID;
};

export type LikesFeedResponse = {
  video_list: FeedVideo[];
  next_cursor?: LikesCursor;
  has_more: boolean;
};

export type PopularityCursor = {
  as_of: number;
  offset: number;
};

export type PopularityFeedResponse = {
  video_list: FeedVideo[];
  next_cursor?: PopularityCursor;
  has_more: boolean;
};

export type UploadURLResponse = {
  url: string;
  play_url?: string;
  cover_url?: string;
};

export type InitChunkUploadResponse = {
  upload_id: string;
  chunk_size: number;
  total_chunks: number;
  upload_chunks: number[];
};

export type ChunkStatusResponse = {
  uploaded_chunks: number[];
  total_chunks: number;
  complete: boolean;
};

export type CompleteChunkUploadResponse = {
  url: string;
  play_url: string;
};

export type RankedVideo = {
  video_id: ID;
  score: number;
  source: "latest" | "popularity" | "following" | "tag_interest" | "likes" | "mixed" | string;
  reasons?: string[];
};

export type RecommendResponse = {
  videos: RankedVideo[];
  next_cursor: string;
  has_more: boolean;
};

export type Comment = {
  id: ID;
  username: string;
  video_id: ID;
  author_id: ID;
  content: string;
  created_at?: string;
};

export type Message = {
  id: ID;
  from_id: ID;
  to_id: ID;
  content: string;
  is_read: boolean;
  created_at?: string;
};

export type MessageListResponse = {
  messages: Message[];
  next_before_id?: ID;
  has_more: boolean;
};

export type MessageConversation = {
  peer_id: ID;
  peer_username?: string;
  last_message: Message;
  unread_count: number;
  updated_at?: string;
};

export type MessageConversationListResponse = {
  conversations: MessageConversation[];
  unread_count: number;
};

export type Notification = {
  id: ID;
  recipient_id: ID;
  sender_id: ID;
  type: string;
  target_id: ID;
  content: string;
  is_read: boolean;
  created_at?: string;
};

export type NotificationListResponse = {
  notifications: Notification[];
};

export type ProfileResponse = {
  account: Account;
  video_count: number;
  total_likes: number;
  follower_count: number;
  vlogger_count: number;
};

export type ReportResponse = {
  report: {
    id: ID;
    target_id: ID;
    target_type: "video" | "comment";
    reason: string;
    status: string;
  };
};

export type EventType = "impression" | "view" | "play_complete" | "share";

export type FeedMode = "recommend" | "latest" | "following" | "popularity" | "likes";
