import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { MessagesPage } from "./pages/MessagesPage";
import { ProfilePage } from "./pages/ProfilePage";
import { PublishPage } from "./pages/PublishPage";
import { VideoFeedPage } from "./pages/VideoFeedPage";

export function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/feed/recommend" replace />} />
        <Route path="feed/:mode" element={<VideoFeedPage />} />
        <Route path="messages" element={<MessagesPage />} />
        <Route path="publish" element={<PublishPage />} />
        <Route path="profile" element={<ProfilePage />} />
        <Route path="*" element={<Navigate to="/feed/recommend" replace />} />
      </Route>
    </Routes>
  );
}
