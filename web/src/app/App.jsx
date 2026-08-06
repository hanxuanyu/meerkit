import React, { useCallback, useEffect, useState } from "react";
import { AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { AppShell } from "../components/layout/AppShell";
import { Sidebar } from "../components/layout/Sidebar";
import { Topbar } from "../components/layout/Topbar";
import { WorkspaceTabs } from "../components/layout/WorkspaceTabs";
import { Toaster } from "../components/ui/Toast";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "../components/ui/AlertDialog";
import { api } from "../lib/api";
import { browserNotificationPreferenceKey, disableBrowserNotifications, enableBrowserNotifications, getBrowserNotificationStatus, showBrowserNotification } from "../features/notifications/browserNotifications";
import { ChannelDialog } from "../features/notifications/ChannelDialog";
import { NotificationBell, NotificationCenterPage } from "../features/notifications/InAppNotifications";
import { MonitorDialog } from "../features/monitors/MonitorDialog";
import { MonitorRecordsDialog, MonitorRecordsPage } from "../features/monitors/MonitorRecords";
import { MonitorsPage } from "../pages/MonitorsPage";
import { NotificationsPage } from "../pages/NotificationsPage";
import { OverviewPage } from "../pages/OverviewPage";
import { SettingsPage } from "../pages/SettingsPage";

const staticPaths = { overview: "/", monitors: "/monitors", inbox: "/notifications", notifications: "/notification-channels", settings: "/settings" };

function decodePathPart(value) {
  try { return decodeURIComponent(value); } catch { return value; }
}

function routeFromPath(pathname) {
  const recordMatch = pathname.match(/^\/monitors\/([^/]+)\/records(?:\/([^/]+))?\/?$/);
  if (recordMatch) return { page: `monitor-details:${decodePathPart(recordMatch[1])}`, recordID: recordMatch[2] ? decodePathPart(recordMatch[2]) : "", notificationID: "" };
  const notificationMatch = pathname.match(/^\/notifications(?:\/([^/]+))?\/?$/);
  if (notificationMatch) return { page: "inbox", recordID: "", notificationID: notificationMatch[1] ? decodePathPart(notificationMatch[1]) : "" };
  const page = Object.entries(staticPaths).find(([, path]) => path !== "/" && pathname.replace(/\/$/, "") === path)?.[0] || "overview";
  return { page, recordID: "", notificationID: "" };
}

function pathForRoute(page, recordID = "", notificationID = "") {
  if (page.startsWith("monitor-details:")) {
    const monitorID = page.slice("monitor-details:".length);
    return `/monitors/${encodeURIComponent(monitorID)}/records${recordID ? `/${encodeURIComponent(recordID)}` : ""}`;
  }
  if (page === "inbox") return `/notifications${notificationID ? `/${encodeURIComponent(notificationID)}` : ""}`;
  return staticPaths[page] || "/";
}

const initialRoute = routeFromPath(window.location.pathname);

export function App() {
  const [activePage, setActivePage] = useState(initialRoute.page);
  const [openTabs, setOpenTabs] = useState(() => initialRoute.page === "overview" ? ["overview"] : ["overview", initialRoute.page]);
  const [recordTabs, setRecordTabs] = useState({});
  const [routeRecordID, setRouteRecordID] = useState(initialRoute.recordID);
  const [routeNotificationID, setRouteNotificationID] = useState(initialRoute.notificationID);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const [mobileTopbarHidden, setMobileTopbarHidden] = useState(false);
  const [modules, setModules] = useState([]);
  const [notifiers, setNotifiers] = useState([]);
  const [monitors, setMonitors] = useState([]);
  const [channels, setChannels] = useState([]);
  const [recentNotifications, setRecentNotifications] = useState([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [inboxVersion, setInboxVersion] = useState(0);
  const [browserNotificationStatus, setBrowserNotificationStatus] = useState(getBrowserNotificationStatus);
  const [selectedMonitor, setSelectedMonitor] = useState(null);
  const [showMonitorDialog, setShowMonitorDialog] = useState(false);
  const [showChannelDialog, setShowChannelDialog] = useState(false);
  const [selectedChannel, setSelectedChannel] = useState(null);
  const [recordsMonitor, setRecordsMonitor] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [loading, setLoading] = useState(true);
  const [refreshVersion, setRefreshVersion] = useState(0);
  const [togglingMonitorId, setTogglingMonitorId] = useState("");
  const [togglingChannelId, setTogglingChannelId] = useState("");

  const notify = useCallback((message, tone = "success") => {
    if (tone === "error") toast.error(message);
    else toast.success(message);
  }, []);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [moduleResponse, notifierResponse, monitorResponse, channelResponse] = await Promise.all([
        api("/api/v1/modules"), api("/api/v1/notifiers"), api("/api/v1/monitors?page=1&page_size=100"), api("/api/v1/notification-channels")
      ]);
      setModules(moduleResponse?.items || []);
      setNotifiers(notifierResponse?.items || []);
      setMonitors(monitorResponse?.items || []);
      setChannels(channelResponse?.items || []);
    } catch (error) {
      notify(error.message, "error");
    } finally {
      setLoading(false);
      setRefreshVersion((current) => current + 1);
    }
  }, [notify]);

  const refreshInboxSummary = useCallback(async () => {
    try {
      const [list, count] = await Promise.all([api("/api/v1/in-app-notifications?page=1&page_size=6"), api("/api/v1/in-app-notifications/unread-count")]);
      setRecentNotifications(list?.items || []);
      setUnreadCount(count?.count || 0);
    } catch {
      // The main refresh path reports connectivity failures; the bell stays quiet until recovery.
    }
  }, []);

  useEffect(() => { void refresh(); void refreshInboxSummary(); }, [refresh, refreshInboxSummary]);
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
          setInboxVersion((current) => current + 1);
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
  }, [refreshInboxSummary]);
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
  useEffect(() => {
    let lastScrollY = window.scrollY;
    let frame = 0;
    const updateTopbar = () => {
      frame = 0;
      if (!window.matchMedia("(max-width: 640px)").matches) {
        setMobileTopbarHidden(false);
        lastScrollY = window.scrollY;
        return;
      }
      const nextScrollY = Math.max(window.scrollY, 0);
      const delta = nextScrollY - lastScrollY;
      if (nextScrollY <= 8) setMobileTopbarHidden(false);
      else if (delta > 8) setMobileTopbarHidden(true);
      else if (delta < -8) setMobileTopbarHidden(false);
      if (Math.abs(delta) > 8) lastScrollY = nextScrollY;
    };
    const onScroll = () => { if (!frame) frame = window.requestAnimationFrame(updateTopbar); };
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onScroll);
    return () => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onScroll);
      if (frame) window.cancelAnimationFrame(frame);
    };
  }, []);

  const activateRoute = useCallback((page, { replace = false, recordID = "", notificationID = "" } = {}) => {
    setActivePage(page);
    setOpenTabs((current) => current.includes(page) ? current : [...current, page]);
    setRouteRecordID(recordID);
    setRouteNotificationID(notificationID);
    const path = pathForRoute(page, recordID, notificationID);
    if (window.location.pathname !== path) window.history[replace ? "replaceState" : "pushState"]({}, "", path);
  }, []);
  const navigate = useCallback((page) => {
    setMobileSidebarOpen(false);
    setMobileTopbarHidden(false);
    activateRoute(page);
  }, [activateRoute]);
  const toggleNavigation = useCallback(() => {
    if (window.matchMedia("(max-width: 640px)").matches) setMobileSidebarOpen((current) => !current);
    else setSidebarCollapsed((current) => !current);
  }, []);

  useEffect(() => {
    const popState = () => {
      const route = routeFromPath(window.location.pathname);
      setActivePage(route.page);
      setOpenTabs((current) => current.includes(route.page) ? current : [...current, route.page]);
      setRouteRecordID(route.recordID);
      setRouteNotificationID(route.notificationID);
      setMobileSidebarOpen(false);
      setMobileTopbarHidden(false);
    };
    window.addEventListener("popstate", popState);
    return () => window.removeEventListener("popstate", popState);
  }, []);

  const monitorContext = useCallback((monitor) => ({ monitor, descriptor: modules.find((item) => item.type === monitor.module_type) }), [modules]);
  useEffect(() => {
    if (!activePage.startsWith("monitor-details:") || !modules.length) return;
    const monitorID = activePage.slice("monitor-details:".length);
    const known = monitors.find((item) => item.id === monitorID);
    if (known) {
      setRecordTabs((current) => ({ ...current, [activePage]: monitorContext(known) }));
      return;
    }
    let cancelled = false;
    api(`/api/v1/monitors/${monitorID}`).then((monitor) => {
      if (!cancelled) setRecordTabs((current) => ({ ...current, [activePage]: monitorContext(monitor) }));
    }).catch((error) => { if (!cancelled) notify(error.message, "error"); });
    return () => { cancelled = true; };
  }, [activePage, modules, monitorContext, monitors, notify]);

  const openMonitorTab = useCallback((monitor, recordID = "") => {
    const tabID = `monitor-details:${monitor.id}`;
    setRecordTabs((current) => ({ ...current, [tabID]: monitorContext(monitor) }));
    setRecordsMonitor(null);
    activateRoute(tabID, { recordID });
  }, [activateRoute, monitorContext]);
  const openExecution = useCallback(async (monitorID, recordID) => {
    try {
      const monitor = monitors.find((item) => item.id === monitorID) || await api(`/api/v1/monitors/${monitorID}`);
      openMonitorTab(monitor, recordID);
    } catch (error) {
      notify(error.message, "error");
    }
  }, [monitors, notify, openMonitorTab]);
  const openRecords = openMonitorTab;
  const openRecordsTab = openMonitorTab;

  const closeTab = useCallback((page) => {
    if (page === "overview") return;
    const index = openTabs.indexOf(page);
    const remaining = openTabs.filter((item) => item !== page);
    setOpenTabs(remaining);
    if (activePage === page) activateRoute(remaining[index - 1] || remaining[index] || "overview", { replace: true });
  }, [activateRoute, activePage, openTabs]);
  const closeOtherTabs = useCallback((page) => {
    setOpenTabs((current) => current.filter((item) => item === "overview" || item === page));
    activateRoute(page, { replace: true });
  }, [activateRoute]);
  const closeRightTabs = useCallback((page) => {
    const index = openTabs.indexOf(page);
    if (index < 0) return;
    const remaining = openTabs.slice(0, index + 1);
    setOpenTabs(remaining);
    if (!remaining.includes(activePage)) activateRoute(page, { replace: true });
  }, [activateRoute, activePage, openTabs]);

  const markNotificationRead = useCallback(async (id) => {
    try {
      await api(`/api/v1/in-app-notifications/${id}/read`, { method: "PATCH" });
      await refreshInboxSummary();
      setInboxVersion((current) => current + 1);
    } catch (error) {
      notify(error.message, "error");
    }
  }, [notify, refreshInboxSummary]);
  const markAllNotificationsRead = useCallback(async () => {
    try {
      await api("/api/v1/in-app-notifications/read-all", { method: "POST" });
      await refreshInboxSummary();
      setInboxVersion((current) => current + 1);
    } catch (error) {
      notify(error.message, "error");
    }
  }, [notify, refreshInboxSummary]);
  const handleNotificationsDeleted = useCallback(async (deleted) => {
    await refreshInboxSummary();
    setInboxVersion((current) => current + 1);
    notify(deleted ? `已删除 ${deleted} 条已读通知` : "没有可删除的已读通知");
  }, [notify, refreshInboxSummary]);
  const handleRecordsDeleted = useCallback((deleted) => {
    notify(deleted ? `已删除 ${deleted} 条执行记录` : "没有可删除的执行记录");
    void refresh();
  }, [notify, refresh]);
  const toggleBrowserNotifications = useCallback(async (enabled) => {
    if (!enabled) {
      setBrowserNotificationStatus(disableBrowserNotifications());
      notify("浏览器通知已关闭");
      return;
    }
    const status = await enableBrowserNotifications();
    setBrowserNotificationStatus(status);
    if (status === "enabled") notify("浏览器通知已开启");
    else if (status === "denied") notify("浏览器已阻止通知，请在站点权限中重新允许", "error");
    else if (status === "unsupported") notify("当前访问方式不支持浏览器通知，请使用 HTTPS 或 localhost", "error");
    else notify("未获得浏览器通知权限", "error");
  }, [notify]);
  const openNotification = useCallback((notification) => activateRoute("inbox", { notificationID: notification.id }), [activateRoute]);
  const changeNotificationRoute = useCallback((notificationID) => activateRoute("inbox", { notificationID, replace: !notificationID }), [activateRoute]);
  const changeRecordRoute = useCallback((recordID) => activateRoute(activePage, { recordID, replace: !recordID }), [activateRoute, activePage]);

  const runMonitor = useCallback(async (monitor) => {
    try {
      const record = await api(`/api/v1/monitors/${monitor.id}/run`, { method: "POST" });
      notify("监控已执行");
      void refresh();
      return record;
    } catch (error) {
      notify(error.message, "error");
      return null;
    }
  }, [notify, refresh]);
  const toggleMonitorEnabled = useCallback(async (monitor) => {
    if (togglingMonitorId) return;
    setTogglingMonitorId(monitor.id);
    try {
      await api(`/api/v1/monitors/${monitor.id}`, { method: "PATCH", body: JSON.stringify({ enabled: !monitor.enabled }) });
      notify(monitor.enabled ? "监控已停用" : "监控已启用");
      void refresh();
    } catch (error) { notify(error.message, "error"); }
    finally { setTogglingMonitorId(""); }
  }, [notify, refresh, togglingMonitorId]);
  const testMonitor = useCallback(async (payload) => {
    try {
      const response = await api("/api/v1/monitors/test", { method: "POST", body: JSON.stringify(payload) });
      if (!response?.success) notify(response?.observation?.error_message || "监控测试失败", "error");
      else notify("监控测试成功");
      return response;
    } catch (error) { notify(error.message, "error"); return null; }
  }, [notify]);
  const testNotification = useCallback(async (payload) => {
    try { await api("/api/v1/notification-channels/test", { method: "POST", body: JSON.stringify(payload) }); notify("测试通知已发送"); }
    catch (error) { notify(error.message, "error"); }
  }, [notify]);

  const openCreateMonitor = () => { setSelectedMonitor(null); setShowMonitorDialog(true); };
  const openEditMonitor = (monitor) => { setSelectedMonitor(monitor); setShowMonitorDialog(true); };
  const openMonitorList = (monitor) => { if (monitor) setSelectedMonitor(monitor); navigate("monitors"); };
  const deleteMonitor = (monitor) => setDeleteTarget(monitor);
  const confirmDeleteMonitor = async (event) => {
    event.preventDefault();
    if (!deleteTarget || deleting) return;
    const target = deleteTarget;
    setDeleting(true);
    try {
      await api(`/api/v1/monitors/${target.id}`, { method: "DELETE" });
      setDeleteTarget(null);
      const detailPage = `monitor-details:${target.id}`;
      setOpenTabs((current) => current.filter((item) => item !== detailPage));
      setRecordTabs((current) => { const next = { ...current }; delete next[detailPage]; return next; });
      if (activePage === detailPage) {
        activateRoute("monitors", { replace: true });
      }
      notify("监控已删除");
      void refresh();
    }
    catch (error) { notify(error.message, "error"); }
    finally { setDeleting(false); }
  };
  const openCreateChannel = useCallback(() => { setSelectedChannel(null); setShowChannelDialog(true); }, []);
  const openEditChannel = useCallback((channel) => { if (!channel.built_in) { setSelectedChannel(channel); setShowChannelDialog(true); } }, []);
  const toggleChannelEnabled = useCallback(async (channel) => {
    if (togglingChannelId) return;
    setTogglingChannelId(channel.id);
    try { await api(`/api/v1/notification-channels/${channel.id}`, { method: "PATCH", body: JSON.stringify({ enabled: !channel.enabled }) }); notify(channel.enabled ? "通知渠道已停用" : "通知渠道已启用"); void refresh(); }
    catch (error) { notify(error.message, "error"); }
    finally { setTogglingChannelId(""); }
  }, [notify, refresh, togglingChannelId]);

  const page = activePage === "overview"
    ? <OverviewPage monitors={monitors} modules={modules} loading={loading} onCreate={openCreateMonitor} onOpen={openMonitorList} onRun={runMonitor} onViewRecords={openRecords} />
    : activePage === "monitors"
      ? <MonitorsPage modules={modules} onCreate={openCreateMonitor} onEdit={openEditMonitor} onRun={runMonitor} onDelete={deleteMonitor} onViewRecords={openRecords} onToggleEnabled={toggleMonitorEnabled} togglingMonitorId={togglingMonitorId} onRefresh={refresh} refreshVersion={refreshVersion} />
      : activePage === "inbox"
        ? <NotificationCenterPage refreshVersion={inboxVersion} initialNotificationID={routeNotificationID} onNotificationRouteChange={changeNotificationRoute} onOpenExecution={openExecution} onMarkRead={markNotificationRead} onMarkAllRead={markAllNotificationsRead} onNotificationsDeleted={handleNotificationsDeleted} unreadCount={unreadCount} browserNotificationStatus={browserNotificationStatus} onToggleBrowserNotifications={toggleBrowserNotifications} />
        : activePage === "notifications"
          ? <NotificationsPage channels={channels} onCreate={openCreateChannel} onEdit={openEditChannel} onToggleEnabled={toggleChannelEnabled} togglingChannelId={togglingChannelId} onRefresh={refresh} />
          : activePage.startsWith("monitor-details:")
            ? recordTabs[activePage] ? <MonitorRecordsPage monitor={recordTabs[activePage].monitor} descriptor={recordTabs[activePage].descriptor} channels={channels} initialRecordID={routeRecordID} onRecordRouteChange={changeRecordRoute} onRecordsDeleted={handleRecordsDeleted} onEdit={openEditMonitor} onRun={runMonitor} onDelete={deleteMonitor} onToggleEnabled={toggleMonitorEnabled} togglingMonitorId={togglingMonitorId} /> : <div className="records-empty">正在加载监控详情...</div>
            : <SettingsPage />;

  const notificationBell = <NotificationBell items={recentNotifications} unreadCount={unreadCount} onMarkRead={markNotificationRead} onMarkAllRead={markAllNotificationsRead} onOpenCenter={() => navigate("inbox")} onOpenNotification={openNotification} browserNotificationStatus={browserNotificationStatus} onToggleBrowserNotifications={toggleBrowserNotifications} />;
  const overlays = <><Toaster position="bottom-right" richColors />{showMonitorDialog && <MonitorDialog monitor={selectedMonitor} modules={modules} channels={channels} onClose={() => setShowMonitorDialog(false)} onSaved={() => { setShowMonitorDialog(false); notify(selectedMonitor ? "监控已更新" : "监控已创建"); void refresh(); }} onError={(message) => notify(message, "error")} onTest={testMonitor} />}{recordsMonitor && <MonitorRecordsDialog monitor={recordsMonitor.monitor} descriptor={recordsMonitor.descriptor} onClose={() => setRecordsMonitor(null)} onOpenTab={openRecordsTab} onRecordsDeleted={handleRecordsDeleted} />}{showChannelDialog && <ChannelDialog channel={selectedChannel} notifiers={notifiers} monitors={monitors} modules={modules} onClose={() => setShowChannelDialog(false)} onSaved={() => { setShowChannelDialog(false); notify(selectedChannel ? "通知渠道已更新" : "通知渠道已创建"); void refresh(); }} onError={(message) => notify(message, "error")} onTest={testNotification} />}<AlertDialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && !deleting && setDeleteTarget(null)}><AlertDialogContent><AlertDialogHeader><div className="alert-dialog-icon"><AlertTriangle size={19} /></div><AlertDialogTitle>删除监控项</AlertDialogTitle><AlertDialogDescription>确定要删除“{deleteTarget?.name}”吗？相关配置和历史执行记录将一并删除，此操作无法撤销。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={deleting}>取消</AlertDialogCancel><AlertDialogAction disabled={deleting} onClick={confirmDeleteMonitor}>{deleting ? "删除中..." : "确认删除"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog></>;

  return <AppShell sidebar={<Sidebar activePage={activePage} collapsed={sidebarCollapsed} mobileOpen={mobileSidebarOpen} unreadCount={unreadCount} onCloseMobile={() => setMobileSidebarOpen(false)} onNavigate={navigate} />} topbar={<Topbar activePage={activePage} mobileHidden={mobileTopbarHidden && !mobileSidebarOpen} onRefresh={() => { void refresh(); void refreshInboxSummary(); }} sidebarCollapsed={sidebarCollapsed} onToggleSidebar={toggleNavigation} notificationBell={notificationBell} />} tabs={<WorkspaceTabs tabs={openTabs} activeId={activePage} onActivate={navigate} onClose={closeTab} onRefresh={() => { void refresh(); void refreshInboxSummary(); }} onCloseOthers={closeOtherTabs} onCloseRight={closeRightTabs} recordTabs={recordTabs} />} sidebarCollapsed={sidebarCollapsed} overlays={overlays}>{page}</AppShell>;
}
