const staticPaths = { overview: "/", monitors: "/monitors", inbox: "/notifications", notifications: "/notification-channels", settings: "/settings" };

function decodePathPart(value) {
  try { return decodeURIComponent(value); } catch { return value; }
}

export function routeFromPath(pathname) {
  const recordMatch = pathname.match(/^\/monitors\/([^/]+)\/records(?:\/([^/]+))?\/?$/);
  if (recordMatch) return { page: `monitor-details:${decodePathPart(recordMatch[1])}`, recordID: recordMatch[2] ? decodePathPart(recordMatch[2]) : "", notificationID: "" };
  const notificationMatch = pathname.match(/^\/notifications(?:\/([^/]+))?\/?$/);
  if (notificationMatch) return { page: "inbox", recordID: "", notificationID: notificationMatch[1] ? decodePathPart(notificationMatch[1]) : "" };
  const page = Object.entries(staticPaths).find(([, path]) => path !== "/" && pathname.replace(/\/$/, "") === path)?.[0] || "overview";
  return { page, recordID: "", notificationID: "" };
}

export function pathForRoute(page, recordID = "", notificationID = "") {
  if (page.startsWith("monitor-details:")) {
    const monitorID = page.slice("monitor-details:".length);
    return `/monitors/${encodeURIComponent(monitorID)}/records${recordID ? `/${encodeURIComponent(recordID)}` : ""}`;
  }
  if (page === "inbox") return `/notifications${notificationID ? `/${encodeURIComponent(notificationID)}` : ""}`;
  return staticPaths[page] || "/";
}
