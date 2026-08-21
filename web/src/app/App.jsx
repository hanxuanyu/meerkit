import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { AppShell } from "../components/layout/AppShell";
import { PageChromeScope } from "../components/layout/PageChrome";
import { Sidebar } from "../components/layout/Sidebar";
import { Topbar } from "../components/layout/Topbar";
import { WorkspaceTabs } from "../components/layout/WorkspaceTabs";
import { api } from "../lib/api";
import { duplicateChannelDraft, duplicateMonitorDraft } from "../lib/duplicates";
import { disableBrowserNotifications, enableBrowserNotifications } from "../features/notifications/browserNotifications";
import { NotificationBell } from "../features/notifications/NotificationBell";
import { NotificationCenterPage } from "../features/notifications/InAppNotifications";
import { MonitorRecordsPage } from "../features/monitors/MonitorRecords";
import { MonitorsPage } from "../pages/MonitorsPage";
import { NotificationsPage } from "../pages/NotificationsPage";
import { OverviewPage } from "../pages/OverviewPage";
import { SettingsPage } from "../pages/SettingsPage";
import { StatusBoardPage } from "../pages/StatusBoardPage";
import { PluginsPage } from "../pages/PluginsPage";
import { SystemLogsPage } from "../pages/SystemLogsPage";
import { BrowserDebugPage } from "../pages/BrowserDebugPage";
import { BrowserPage } from "../pages/BrowserPage";
import { AppOverlays } from "./AppOverlays";
import { pathForRoute, routeFromPath } from "./routes";
import { useMobileShell } from "./useMobileShell";
import { useNotificationFeed } from "./useNotificationFeed";

const initialRoute = routeFromPath(window.location.pathname);

export function App() {
  const [activePage, setActivePage] = useState(initialRoute.page);
  const [openTabs, setOpenTabs] = useState(() => initialRoute.page === "overview" ? ["overview"] : ["overview", initialRoute.page]);
  const [recordTabs, setRecordTabs] = useState({});
  const [routeRecordID, setRouteRecordID] = useState(initialRoute.recordID);
  const [routeNotificationID, setRouteNotificationID] = useState(initialRoute.notificationID);
  const tabRouteStateRef = useRef(new Map([[initialRoute.page, { recordID: initialRoute.recordID, notificationID: initialRoute.notificationID }]]));
  const { sidebarCollapsed, mobileSidebarOpen, mobileTopbarHidden, toggleNavigation, closeMobileSidebar, resetMobileNavigation } = useMobileShell();
  const [modules, setModules] = useState([]);
  const [notifiers, setNotifiers] = useState([]);
  const [monitors, setMonitors] = useState([]);
  const [channels, setChannels] = useState([]);
  const { recentNotifications, unreadCount, inboxVersion, browserNotificationStatus, setBrowserNotificationStatus, refreshInboxSummary, bumpInboxVersion } = useNotificationFeed();
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
  const [executionDetail, setExecutionDetail] = useState(null);
  const executionDetailRequest = useRef(0);
  const pageContentRef = useRef(null);
  const pageScrollPositionsRef = useRef(new Map());
  const activePageRef = useRef(activePage);
  activePageRef.current = activePage;

  const notify = useCallback((message, tone = "success") => {
    if (tone === "error") toast.error(message);
    else toast.success(message);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api("/api/v1/auth/logout", { method: "POST" });
      window.dispatchEvent(new CustomEvent("meerkit:unauthorized"));
    } catch (error) {
      notify(error.message, "error");
    }
  }, [notify]);

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

  useEffect(() => { void refresh(); void refreshInboxSummary(); }, [refresh, refreshInboxSummary]);

  const activateRoute = useCallback((page, { replace = false, recordID, notificationID } = {}) => {
    const savedRoute = tabRouteStateRef.current.get(page) || {};
    const nextRecordID = recordID === undefined ? savedRoute.recordID || "" : recordID;
    const nextNotificationID = notificationID === undefined ? savedRoute.notificationID || "" : notificationID;
    tabRouteStateRef.current.set(page, { recordID: nextRecordID, notificationID: nextNotificationID });
    setActivePage(page);
    setOpenTabs((current) => current.includes(page) ? current : [...current, page]);
    setRouteRecordID(nextRecordID);
    setRouteNotificationID(nextNotificationID);
    const path = pathForRoute(page, nextRecordID, nextNotificationID);
    if (window.location.pathname !== path) window.history[replace ? "replaceState" : "pushState"]({}, "", path);
  }, []);
  const navigate = useCallback((page) => {
    resetMobileNavigation();
    activateRoute(page);
  }, [activateRoute, resetMobileNavigation]);

  useEffect(() => {
    const popState = () => {
      const route = routeFromPath(window.location.pathname);
      setActivePage(route.page);
      setOpenTabs((current) => current.includes(route.page) ? current : [...current, route.page]);
      setRouteRecordID(route.recordID);
      setRouteNotificationID(route.notificationID);
      tabRouteStateRef.current.set(route.page, { recordID: route.recordID, notificationID: route.notificationID });
      resetMobileNavigation();
    };
    window.addEventListener("popstate", popState);
    return () => window.removeEventListener("popstate", popState);
  }, [resetMobileNavigation]);

  const rememberPageScroll = useCallback((event) => {
    pageScrollPositionsRef.current.set(activePageRef.current, event.currentTarget.scrollTop);
  }, []);
  useLayoutEffect(() => {
    const content = pageContentRef.current;
    if (!content) return;
    const saved = pageScrollPositionsRef.current.get(activePage) || 0;
    content.scrollTop = saved;
  }, [activePage]);

  const monitorContext = useCallback((monitor) => ({ monitor, descriptor: modules.find((item) => item.type === monitor.module_type) }), [modules]);
  useEffect(() => {
    if (!activePage.startsWith("monitor-details:")) return;
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
  const closeExecution = useCallback(() => {
    executionDetailRequest.current += 1;
    setExecutionDetail(null);
  }, []);
  const openExecution = useCallback(async (monitorID, recordID) => {
    const requestID = executionDetailRequest.current + 1;
    executionDetailRequest.current = requestID;
    setExecutionDetail({ loading: true, error: "", record: null });
    try {
      const record = await api(`/api/v1/monitors/${monitorID}/records/${recordID}`);
      if (executionDetailRequest.current !== requestID) return;
      setExecutionDetail({ loading: false, error: "", record });
    } catch (error) {
      if (executionDetailRequest.current !== requestID) return;
      setExecutionDetail({ loading: false, error: error.message, record: null });
    }
  }, []);
  const openRecords = openMonitorTab;
  const openRecordsTab = openMonitorTab;

  const closeTab = useCallback((page) => {
    if (page === "overview") return;
    const index = openTabs.indexOf(page);
    const remaining = openTabs.filter((item) => item !== page);
    setOpenTabs(remaining);
    pageScrollPositionsRef.current.delete(page);
    tabRouteStateRef.current.delete(page);
    if (activePage === page) activateRoute(remaining[index - 1] || remaining[index] || "overview", { replace: true });
  }, [activateRoute, activePage, openTabs]);
  const closeOtherTabs = useCallback((page) => {
    setOpenTabs((current) => current.filter((item) => item === "overview" || item === page));
    pageScrollPositionsRef.current.forEach((_value, key) => { if (key !== "overview" && key !== page) pageScrollPositionsRef.current.delete(key); });
    tabRouteStateRef.current.forEach((_value, key) => { if (key !== "overview" && key !== page) tabRouteStateRef.current.delete(key); });
    activateRoute(page, { replace: true });
  }, [activateRoute]);
  const closeRightTabs = useCallback((page) => {
    const index = openTabs.indexOf(page);
    if (index < 0) return;
    const remaining = openTabs.slice(0, index + 1);
    setOpenTabs(remaining);
    pageScrollPositionsRef.current.forEach((_value, key) => { if (!remaining.includes(key)) pageScrollPositionsRef.current.delete(key); });
    tabRouteStateRef.current.forEach((_value, key) => { if (!remaining.includes(key)) tabRouteStateRef.current.delete(key); });
    if (!remaining.includes(activePage)) activateRoute(page, { replace: true });
  }, [activateRoute, activePage, openTabs]);

  const markNotificationRead = useCallback(async (id) => {
    try {
      await api(`/api/v1/in-app-notifications/${id}/read`, { method: "PATCH" });
      await refreshInboxSummary();
      bumpInboxVersion();
    } catch (error) {
      notify(error.message, "error");
    }
  }, [bumpInboxVersion, notify, refreshInboxSummary]);
  const markAllNotificationsRead = useCallback(async () => {
    try {
      await api("/api/v1/in-app-notifications/read-all", { method: "POST" });
      await refreshInboxSummary();
      bumpInboxVersion();
    } catch (error) {
      notify(error.message, "error");
    }
  }, [bumpInboxVersion, notify, refreshInboxSummary]);
  const handleNotificationsDeleted = useCallback(async (deleted) => {
    await refreshInboxSummary();
    bumpInboxVersion();
    notify(deleted ? `已删除 ${deleted} 条已读通知` : "没有可删除的已读通知");
  }, [bumpInboxVersion, notify, refreshInboxSummary]);
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
    if (togglingMonitorId || monitor.module_available === false) return;
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
  const openEditMonitor = (monitor) => { if (monitor.module_available === false) return; setSelectedMonitor(monitor); setShowMonitorDialog(true); };
  const openDuplicateMonitor = (monitor) => { if (monitor.module_available === false) return; setSelectedMonitor(duplicateMonitorDraft(monitor)); setShowMonitorDialog(true); };
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
      pageScrollPositionsRef.current.delete(detailPage);
      tabRouteStateRef.current.delete(detailPage);
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
  const openDuplicateChannel = useCallback((channel) => { if (!channel.built_in) { setSelectedChannel(duplicateChannelDraft(channel)); setShowChannelDialog(true); } }, []);
  const toggleChannelEnabled = useCallback(async (channel) => {
    if (togglingChannelId) return;
    setTogglingChannelId(channel.id);
    try { await api(`/api/v1/notification-channels/${channel.id}`, { method: "PATCH", body: JSON.stringify({ enabled: !channel.enabled }) }); notify(channel.enabled ? "通知渠道已停用" : "通知渠道已启用"); void refresh(); }
    catch (error) { notify(error.message, "error"); }
    finally { setTogglingChannelId(""); }
  }, [notify, refresh, togglingChannelId]);

  const renderPage = (pageKey) => pageKey === "overview"
    ? <OverviewPage monitors={monitors} modules={modules} channels={channels} recentNotifications={recentNotifications} unreadCount={unreadCount} loading={loading} onCreate={openCreateMonitor} onOpenMonitor={openRecords} onOpenMonitors={() => navigate("monitors")} onOpenInbox={() => navigate("inbox")} onOpenNotification={openNotification} />
    : pageKey === "monitors"
      ? <MonitorsPage modules={modules} onCreate={openCreateMonitor} onEdit={openEditMonitor} onDuplicate={openDuplicateMonitor} onRun={runMonitor} onDelete={deleteMonitor} onViewRecords={openRecords} onToggleEnabled={toggleMonitorEnabled} togglingMonitorId={togglingMonitorId} onRefresh={refresh} refreshVersion={refreshVersion} />
      : pageKey === "statusBoard"
        ? <StatusBoardPage monitors={monitors} channels={channels} refreshVersion={refreshVersion} onOpenExecution={openExecution} notify={notify} />
      : pageKey === "inbox"
        ? <NotificationCenterPage refreshVersion={inboxVersion} initialNotificationID={pageKey === activePage ? routeNotificationID : null} onNotificationRouteChange={changeNotificationRoute} onOpenExecution={openExecution} onMarkRead={markNotificationRead} onMarkAllRead={markAllNotificationsRead} onNotificationsDeleted={handleNotificationsDeleted} unreadCount={unreadCount} browserNotificationStatus={browserNotificationStatus} onToggleBrowserNotifications={toggleBrowserNotifications} />
      : pageKey === "notifications"
        ? <NotificationsPage channels={channels} onCreate={openCreateChannel} onEdit={openEditChannel} onDuplicate={openDuplicateChannel} onToggleEnabled={toggleChannelEnabled} togglingChannelId={togglingChannelId} onRefresh={refresh} />
        : pageKey === "plugins"
          ? <PluginsPage notify={notify} onChanged={refresh} />
        : pageKey === "browserDebug"
          ? <BrowserDebugPage />
        : pageKey === "browser"
          ? <BrowserPage />
        : pageKey === "logs"
          ? <SystemLogsPage />
        : pageKey.startsWith("monitor-details:")
          ? recordTabs[pageKey] ? <MonitorRecordsPage monitor={recordTabs[pageKey].monitor} descriptor={recordTabs[pageKey].descriptor} channels={channels} initialRecordID={pageKey === activePage ? routeRecordID : null} onRecordRouteChange={changeRecordRoute} onRecordsDeleted={handleRecordsDeleted} onEdit={openEditMonitor} onDuplicate={openDuplicateMonitor} onRun={runMonitor} onDelete={deleteMonitor} onToggleEnabled={toggleMonitorEnabled} togglingMonitorId={togglingMonitorId} /> : <div className="records-empty">正在加载监控详情...</div>
          : <SettingsPage />;

  const pagePanels = openTabs.map((pageKey) => <div className={`page-tab-panel ${pageKey === activePage ? "is-active" : ""}`} key={pageKey} aria-hidden={pageKey !== activePage}><PageChromeScope pageKey={pageKey}>{renderPage(pageKey)}</PageChromeScope></div>);

  const notificationBell = <NotificationBell items={recentNotifications} unreadCount={unreadCount} onMarkRead={markNotificationRead} onMarkAllRead={markAllNotificationsRead} onOpenCenter={() => navigate("inbox")} onOpenNotification={openNotification} browserNotificationStatus={browserNotificationStatus} onToggleBrowserNotifications={toggleBrowserNotifications} />;
  const overlays = <AppOverlays
    monitorDialog={{ open: showMonitorDialog, monitor: selectedMonitor, modules, channels, onClose: () => setShowMonitorDialog(false), onSaved: () => { setShowMonitorDialog(false); notify(selectedMonitor?.id ? "监控已更新" : selectedMonitor?.__duplicate ? "监控副本已创建" : "监控已创建"); void refresh(); }, onError: (message) => notify(message, "error"), onTest: testMonitor }}
    recordsDialog={{ context: recordsMonitor, channels, onClose: () => setRecordsMonitor(null), onOpenTab: openRecordsTab, onRecordsDeleted: handleRecordsDeleted }}
    executionDetailDialog={{ state: executionDetail, channels, onClose: closeExecution }}
    channelDialog={{ open: showChannelDialog, channel: selectedChannel, notifiers, monitors, modules, onClose: () => setShowChannelDialog(false), onSaved: () => { setShowChannelDialog(false); notify(selectedChannel?.id ? "通知渠道已更新" : selectedChannel?.__duplicate ? "通知渠道副本已创建" : "通知渠道已创建"); void refresh(); }, onError: (message) => notify(message, "error"), onTest: testNotification }}
    deleteMonitorDialog={{ target: deleteTarget, busy: deleting, onOpenChange: (open) => !open && !deleting && setDeleteTarget(null), onConfirm: confirmDeleteMonitor }}
  />;

  return <AppShell activePage={activePage} contentRef={pageContentRef} onContentScroll={rememberPageScroll} sidebar={<Sidebar activePage={activePage} collapsed={sidebarCollapsed} mobileOpen={mobileSidebarOpen} unreadCount={unreadCount} onCloseMobile={closeMobileSidebar} onNavigate={navigate} />} topbar={<Topbar activePage={activePage} mobileHidden={mobileTopbarHidden && !mobileSidebarOpen} onLogout={() => { void logout(); }} onRefresh={() => { void refresh(); void refreshInboxSummary(); }} sidebarCollapsed={sidebarCollapsed} onToggleSidebar={toggleNavigation} notificationBell={notificationBell} />} tabs={<WorkspaceTabs tabs={openTabs} activeId={activePage} onActivate={navigate} onClose={closeTab} onRefresh={() => { void refresh(); void refreshInboxSummary(); }} onCloseOthers={closeOtherTabs} onCloseRight={closeRightTabs} recordTabs={recordTabs} />} sidebarCollapsed={sidebarCollapsed} overlays={overlays}>{pagePanels}</AppShell>;
}
