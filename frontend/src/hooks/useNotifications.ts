import { useCallback, useEffect, useState } from "react";
import { apiClient } from "../api/client";
import { pulsefeedApi } from "../api/pulsefeed";
import type { Notification, Session } from "../types/api";
import { useToast } from "./useToast";

export function useNotifications(session: Session | null) {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const { pushToast } = useToast();

  const refresh = useCallback(async () => {
    if (!session?.token) {
      setNotifications([]);
      setUnreadCount(0);
      return;
    }
    const [list, unread] = await Promise.all([
      pulsefeedApi.listNotifications(),
      pulsefeedApi.unreadCount(),
    ]);
    setNotifications(list.notifications || []);
    setUnreadCount(unread.count || 0);
  }, [session?.token]);

  const markRead = useCallback(
    async (id?: number) => {
      if (!session?.token) return;
      await pulsefeedApi.markNotificationRead(id);
      await refresh();
    },
    [refresh, session?.token],
  );

  useEffect(() => {
    refresh().catch(() => undefined);
  }, [refresh]);

  useEffect(() => {
    if (!session?.token) return undefined;
    const url = `${apiClient.absoluteUrl("/notification/stream")}?token=${encodeURIComponent(session.token)}`;
    const source = new EventSource(url);

    source.onmessage = (event) => {
      try {
        const notification = JSON.parse(event.data) as Notification;
        setNotifications((items) => [notification, ...items]);
        setUnreadCount((count) => count + 1);
        pushToast(notification.content || "收到新通知", "success");
      } catch {
        pushToast("收到新通知", "success");
      }
    };

    source.onerror = () => {
      source.close();
    };

    return () => source.close();
  }, [pushToast, session?.token]);

  return {
    notifications,
    unreadCount,
    refresh,
    markRead,
  };
}
