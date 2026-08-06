import { useCallback, useEffect, useState } from "react";
import { api } from "../lib/api";
import { browserNotificationPreferenceKey, getBrowserNotificationStatus, showBrowserNotification } from "../features/notifications/browserNotifications";

export function useNotificationFeed() {
  const [recentNotifications, setRecentNotifications] = useState([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [inboxVersion, setInboxVersion] = useState(0);
  const [browserNotificationStatus, setBrowserNotificationStatus] = useState(getBrowserNotificationStatus);

  const refreshInboxSummary = useCallback(async () => {
    try {
      const [list, count] = await Promise.all([api("/api/v1/in-app-notifications?page=1&page_size=6"), api("/api/v1/in-app-notifications/unread-count")]);
      setRecentNotifications(list?.items || []);
      setUnreadCount(count?.count || 0);
    } catch {
      // The main refresh path reports connectivity failures; the bell stays quiet until recovery.
    }
  }, []);
  const bumpInboxVersion = useCallback(() => setInboxVersion((current) => current + 1), []);

  useEffect(() => {
    let active = true;
    let socket;
    let retryTimer;
    let retryDelay = 1000;
    const connect = () => {
      if (!active) return;
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      socket = new WebSocket(`${protocol}//${window.location.host}/api/v1/in-app-notifications/ws`);
      socket.onopen = () => {
        retryDelay = 1000;
        void refreshInboxSummary();
      };
      socket.onmessage = (message) => {
        try {
          const event = JSON.parse(message.data);
          if (typeof event.unread_count === "number") setUnreadCount(event.unread_count);
          if (event.type === "created" && event.notification) {
            setRecentNotifications((current) => [event.notification, ...current.filter((item) => item.id !== event.notification.id)].slice(0, 6));
            void showBrowserNotification(event.notification);
          }
          else void refreshInboxSummary();
          bumpInboxVersion();
        } catch {
          void refreshInboxSummary();
        }
      };
      socket.onerror = () => socket?.close();
      socket.onclose = () => {
        if (!active) return;
        window.clearTimeout(retryTimer);
        retryTimer = window.setTimeout(connect, retryDelay);
        retryDelay = Math.min(retryDelay * 2, 15000);
      };
    };
    connect();
    return () => {
      active = false;
      window.clearTimeout(retryTimer);
      socket?.close();
    };
  }, [bumpInboxVersion, refreshInboxSummary]);

  useEffect(() => {
    const syncBrowserNotificationStatus = (event) => {
      if (!event || event.type === "focus" || event.key === browserNotificationPreferenceKey) setBrowserNotificationStatus(getBrowserNotificationStatus());
    };
    window.addEventListener("storage", syncBrowserNotificationStatus);
    window.addEventListener("focus", syncBrowserNotificationStatus);
    return () => {
      window.removeEventListener("storage", syncBrowserNotificationStatus);
      window.removeEventListener("focus", syncBrowserNotificationStatus);
    };
  }, []);

  useEffect(() => {
    const syncWhenVisible = () => { if (document.visibilityState === "visible") void refreshInboxSummary(); };
    const timer = window.setInterval(syncWhenVisible, 15000);
    window.addEventListener("online", syncWhenVisible);
    document.addEventListener("visibilitychange", syncWhenVisible);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("online", syncWhenVisible);
      document.removeEventListener("visibilitychange", syncWhenVisible);
    };
  }, [refreshInboxSummary]);

  return { recentNotifications, unreadCount, inboxVersion, browserNotificationStatus, setBrowserNotificationStatus, refreshInboxSummary, bumpInboxVersion };
}
