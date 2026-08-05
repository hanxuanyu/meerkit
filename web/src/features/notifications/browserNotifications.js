const preferenceKey = "meerkit.browser-notifications.enabled";
const deliveryClaimKey = "meerkit.browser-notifications.last-delivery";
const serviceWorkerPath = "/notification-sw.js";
let workerRegistrationPromise;

function isSupported() {
  return typeof window !== "undefined" && window.isSecureContext && "Notification" in window;
}

function readPreference() {
  try {
    return window.localStorage.getItem(preferenceKey) === "true";
  } catch {
    return false;
  }
}

function writePreference(enabled) {
  try {
    window.localStorage.setItem(preferenceKey, String(enabled));
  } catch {
    // Permission can still be used for this page when storage is unavailable.
  }
}

export function getBrowserNotificationStatus() {
  if (!isSupported()) return "unsupported";
  if (window.Notification.permission === "denied") return "denied";
  if (window.Notification.permission === "granted" && readPreference()) return "enabled";
  return "disabled";
}

async function registerNotificationWorker() {
  if (!("serviceWorker" in navigator)) return null;
  if (!workerRegistrationPromise) {
    workerRegistrationPromise = navigator.serviceWorker.register(serviceWorkerPath, { scope: "/" }).then(() => navigator.serviceWorker.ready).catch((error) => {
      workerRegistrationPromise = undefined;
      throw error;
    });
  }
  return workerRegistrationPromise;
}

export async function enableBrowserNotifications() {
  if (!isSupported()) return "unsupported";
  let permission = window.Notification.permission;
  if (permission === "default") permission = await window.Notification.requestPermission();
  if (permission !== "granted") {
    writePreference(false);
    return permission === "denied" ? "denied" : "disabled";
  }
  writePreference(true);
  try {
    await registerNotificationWorker();
  } catch {
    // The Notification constructor remains available as a desktop fallback.
  }
  return "enabled";
}

export function disableBrowserNotifications() {
  writePreference(false);
  return getBrowserNotificationStatus();
}

function claimDelivery(id) {
  if (!id) return true;
  const now = Date.now();
  try {
    const previous = JSON.parse(window.localStorage.getItem(deliveryClaimKey) || "null");
    if (previous?.id === id && now - previous.at < 10000) return false;
    window.localStorage.setItem(deliveryClaimKey, JSON.stringify({ id, at: now }));
  } catch {
    // Notification tags still provide best-effort deduplication without storage.
  }
  return true;
}

function detailPath(notification) {
  if (notification?.id) return `/notifications/${encodeURIComponent(notification.id)}`;
  if (notification?.monitor_id && notification?.record_id) {
    return `/monitors/${encodeURIComponent(notification.monitor_id)}/records/${encodeURIComponent(notification.record_id)}`;
  }
  return "/notifications";
}

function notificationOptions(notification) {
  const content = String(notification?.content || "").trim();
  return {
    body: content.length > 240 ? `${content.slice(0, 237)}...` : content,
    icon: "/apple-touch-icon.png",
    badge: "/favicon.png",
    tag: `meerkit-in-app-${notification?.id || Date.now()}`,
    renotify: true,
    data: { path: detailPath(notification) }
  };
}

export async function showBrowserNotification(notification) {
  if (getBrowserNotificationStatus() !== "enabled" || !claimDelivery(notification?.id)) return false;
  const title = String(notification?.title || "Meerkit 站内通知");
  const options = notificationOptions(notification);
  let registration;
  try {
    registration = await registerNotificationWorker();
  } catch {
    registration = null;
  }
  if (registration) {
    try {
      await registration.showNotification(title, options);
      return true;
    } catch {
      // Continue with the constructor fallback where the browser supports it.
    }
  }
  try {
    const browserNotification = new window.Notification(title, options);
    browserNotification.onclick = () => {
      browserNotification.close();
      window.focus();
      window.location.assign(options.data.path);
    };
    return true;
  } catch {
    return false;
  }
}

export const browserNotificationPreferenceKey = preferenceKey;
