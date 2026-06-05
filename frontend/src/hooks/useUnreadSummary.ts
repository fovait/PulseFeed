import { useCallback, useEffect, useState } from "react";
import { apiClient } from "../api/client";
import { pulsefeedApi } from "../api/pulsefeed";
import type { Notification, Session } from "../types/api";

const UNREAD_REFRESH_EVENT = "pulsefeed:unread-refresh";
const UNREAD_POLL_MS = 15_000;

export function requestUnreadRefresh() {
  window.dispatchEvent(new CustomEvent(UNREAD_REFRESH_EVENT));
}

export function useUnreadSummary(session: Session | null) {
  const [notificationUnread, setNotificationUnread] = useState(0);
  const [messageUnread, setMessageUnread] = useState(0);

  const refresh = useCallback(async () => {
    if (!session?.token) {
      setNotificationUnread(0);
      setMessageUnread(0);
      return;
    }

    const [notificationResult, messageResult] = await Promise.allSettled([
      pulsefeedApi.unreadCount(),
      pulsefeedApi.listMessageConversations(),
    ]);

    if (notificationResult.status === "fulfilled") {
      setNotificationUnread(notificationResult.value.count || 0);
    }
    if (messageResult.status === "fulfilled") {
      setMessageUnread(messageResult.value.unread_count || 0);
    }
  }, [session?.token]);

  useEffect(() => {
    refresh().catch(() => undefined);
  }, [refresh]);

  useEffect(() => {
    if (!session?.token) return undefined;

    const intervalID = window.setInterval(() => {
      refresh().catch(() => undefined);
    }, UNREAD_POLL_MS);

    const refreshWhenVisible = () => {
      if (document.visibilityState === "visible") {
        refresh().catch(() => undefined);
      }
    };

    window.addEventListener(UNREAD_REFRESH_EVENT, refresh);
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refreshWhenVisible);

    return () => {
      window.clearInterval(intervalID);
      window.removeEventListener(UNREAD_REFRESH_EVENT, refresh);
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refreshWhenVisible);
    };
  }, [refresh, session?.token]);

  useEffect(() => {
    if (!session?.token) return undefined;

    const url = `${apiClient.absoluteUrl("/notification/stream")}?token=${encodeURIComponent(session.token)}`;
    const source = new EventSource(url);

    source.onmessage = (event) => {
      try {
        const notification = JSON.parse(event.data) as Notification;
        setNotificationUnread((count) => count + (notification.is_read ? 0 : 1));
      } catch {
        setNotificationUnread((count) => count + 1);
      }
    };

    source.onerror = () => {
      source.close();
    };

    return () => source.close();
  }, [session?.token]);

  return {
    notificationUnread,
    messageUnread,
    totalUnread: notificationUnread + messageUnread,
    refresh,
  };
}
