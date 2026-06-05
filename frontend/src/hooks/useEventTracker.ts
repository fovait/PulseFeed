import { useCallback, useMemo, useRef } from "react";
import { pulsefeedApi } from "../api/pulsefeed";
import type { EventType, Session } from "../types/api";

function sessionKey() {
  const key = "pulsefeed.eventSession";
  const existing = sessionStorage.getItem(key);
  if (existing) return existing;
  const next = crypto.randomUUID ? crypto.randomUUID() : String(Date.now());
  sessionStorage.setItem(key, next);
  return next;
}

export function useEventTracker(session: Session | null) {
  const seenRef = useRef(new Set<string>());
  const currentSession = useMemo(() => sessionKey(), []);

  return useCallback(
    async (videoID: number, type: EventType) => {
      if (!session?.account_id || !videoID) return;
      const timeWindow = Math.floor(Date.now() / 30_000);
      const key = `${session.account_id}/${videoID}/${type}/${currentSession}/${timeWindow}`;
      if (seenRef.current.has(key)) return;
      seenRef.current.add(key);
      try {
        await pulsefeedApi.track(videoID, type, key.slice(0, 128));
      } catch {
        // Tracking must never block playback or core interactions.
      }
    },
    [currentSession, session?.account_id],
  );
}
