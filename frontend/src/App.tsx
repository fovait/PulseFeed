import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { AdminModerationPage } from "./pages/AdminModerationPage";
import { FollowListPage } from "./pages/FollowListPage";
import { MessagesPage } from "./pages/MessagesPage";
import { NotificationsPage } from "./pages/NotificationsPage";
import { ProfilePage } from "./pages/ProfilePage";
import { PublishPage } from "./pages/PublishPage";
import { SearchPage } from "./pages/SearchPage";
import { TagFeedPage } from "./pages/TagFeedPage";
import { UserProfilePage } from "./pages/UserProfilePage";
import { VideoDetailPage } from "./pages/VideoDetailPage";
import { VideoFeedPage } from "./pages/VideoFeedPage";

export function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/feed/recommend" replace />} />
        <Route path="feed/:mode" element={<VideoFeedPage />} />
        <Route path="messages" element={<MessagesPage />} />
        <Route path="notifications" element={<NotificationsPage />} />
        <Route path="publish" element={<PublishPage />} />
        <Route path="profile" element={<ProfilePage />} />
        <Route path="user/:id" element={<UserProfilePage />} />
        <Route path="user/:id/followers" element={<FollowListPage mode="followers" />} />
        <Route path="user/:id/following" element={<FollowListPage mode="following" />} />
        <Route path="video/:id" element={<VideoDetailPage />} />
        <Route path="search" element={<SearchPage />} />
        <Route path="admin/moderation" element={<AdminModerationPage />} />
        <Route path="tag" element={<TagFeedPage />} />
        <Route path="tag/:name" element={<TagFeedPage />} />
        <Route path="*" element={<Navigate to="/feed/recommend" replace />} />
      </Route>
    </Routes>
  );
}
