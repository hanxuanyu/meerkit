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
import { ChannelDialog } from "../features/notifications/ChannelDialog";
import { MonitorDialog } from "../features/monitors/MonitorDialog";
import { MonitorsPage } from "../pages/MonitorsPage";
import { NotificationsPage } from "../pages/NotificationsPage";
import { OverviewPage } from "../pages/OverviewPage";
import { SettingsPage } from "../pages/SettingsPage";

export function App() {
  const [activePage, setActivePage] = useState("overview");
  const [openTabs, setOpenTabs] = useState(["overview"]);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [modules, setModules] = useState([]);
  const [notifiers, setNotifiers] = useState([]);
  const [monitors, setMonitors] = useState([]);
  const [channels, setChannels] = useState([]);
  const [selectedMonitor, setSelectedMonitor] = useState(null);
  const [showMonitorDialog, setShowMonitorDialog] = useState(false);
  const [showChannelDialog, setShowChannelDialog] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [loading, setLoading] = useState(true);

  const notify = useCallback((message, tone = "success") => {
    if (tone === "error") toast.error(message);
    else toast.success(message);
  }, []);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [moduleResponse, notifierResponse, monitorResponse, channelResponse] = await Promise.all([
        api("/api/v1/modules"), api("/api/v1/notifiers"), api("/api/v1/monitors"), api("/api/v1/notification-channels")
      ]);
      setModules(moduleResponse?.items || []);
      setNotifiers(notifierResponse?.items || []);
      setMonitors(monitorResponse?.items || []);
      setChannels(channelResponse?.items || []);
    } catch (error) {
      notify(error.message, "error");
    } finally {
      setLoading(false);
    }
  }, [notify]);

  useEffect(() => { void refresh(); }, [refresh]);

  const runMonitor = useCallback(async (monitor) => {
    try { await api(`/api/v1/monitors/${monitor.id}/run`, { method: "POST" }); notify("监控已执行"); void refresh(); }
    catch (error) { notify(error.message, "error"); }
  }, [notify, refresh]);

  const navigate = useCallback((page) => { setActivePage(page); setOpenTabs((current) => current.includes(page) ? current : [...current, page]); }, []);
  const closeTab = useCallback((page) => {
    if (page === "overview") return;
    const index = openTabs.indexOf(page);
    const remaining = openTabs.filter((item) => item !== page);
    setOpenTabs(remaining);
    if (activePage === page) setActivePage(remaining[index - 1] || remaining[index] || "overview");
  }, [activePage, openTabs]);
  const closeOtherTabs = useCallback((page) => {
    setOpenTabs((current) => current.filter((item) => item === "overview" || item === page));
    setActivePage(page);
  }, []);
  const closeRightTabs = useCallback((page) => {
    const index = openTabs.indexOf(page);
    if (index < 0) return;
    const remaining = openTabs.slice(0, index + 1);
    setOpenTabs(remaining);
    if (!remaining.includes(activePage)) setActivePage(page);
  }, [activePage, openTabs]);
  const openCreateMonitor = () => { setSelectedMonitor(null); setShowMonitorDialog(true); };
  const openEditMonitor = (monitor) => { setSelectedMonitor(monitor); setShowMonitorDialog(true); };
  const openMonitorList = (monitor) => { if (monitor) setSelectedMonitor(monitor); navigate("monitors"); };

  const deleteMonitor = (monitor) => setDeleteTarget(monitor);
  const confirmDeleteMonitor = async (event) => {
    event.preventDefault();
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    try {
      await api(`/api/v1/monitors/${deleteTarget.id}`, { method: "DELETE" });
      setDeleteTarget(null);
      notify("监控已删除");
      void refresh();
    } catch (error) {
      notify(error.message, "error");
    } finally {
      setDeleting(false);
    }
  };

  const testChannel = async (channel) => {
    try { await api(`/api/v1/notification-channels/${channel.id}/test`, { method: "POST" }); notify("测试通知已发送"); }
    catch (error) { notify(error.message, "error"); }
  };

  const page = activePage === "overview"
    ? <OverviewPage monitors={monitors} loading={loading} onCreate={openCreateMonitor} onOpen={openMonitorList} onRun={runMonitor} />
    : activePage === "monitors"
      ? <MonitorsPage monitors={monitors} loading={loading} onCreate={openCreateMonitor} onEdit={openEditMonitor} onRun={runMonitor} onDelete={deleteMonitor} onRefresh={refresh} />
      : activePage === "notifications"
        ? <NotificationsPage channels={channels} onCreate={() => setShowChannelDialog(true)} onRefresh={refresh} onTest={testChannel} />
        : <SettingsPage />;

  const overlays = <><Toaster position="bottom-right" closeButton richColors />{showMonitorDialog && <MonitorDialog monitor={selectedMonitor} modules={modules} channels={channels} defaultTimezone="Asia/Shanghai" onClose={() => setShowMonitorDialog(false)} onSaved={() => { setShowMonitorDialog(false); notify(selectedMonitor ? "监控已更新" : "监控已创建"); void refresh(); }} onError={(message) => notify(message, "error")} />}{showChannelDialog && <ChannelDialog notifiers={notifiers} onClose={() => setShowChannelDialog(false)} onSaved={() => { setShowChannelDialog(false); notify("通知渠道已创建"); void refresh(); }} onError={(message) => notify(message, "error")} />}<AlertDialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && !deleting && setDeleteTarget(null)}><AlertDialogContent><AlertDialogHeader><div className="alert-dialog-icon"><AlertTriangle size={19} /></div><AlertDialogTitle>删除监控项</AlertDialogTitle><AlertDialogDescription>确定要删除“{deleteTarget?.name}”吗？相关配置和历史执行记录将一并删除，此操作无法撤销。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={deleting}>取消</AlertDialogCancel><AlertDialogAction disabled={deleting} onClick={confirmDeleteMonitor}>{deleting ? "删除中..." : "确认删除"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog></>;

  return <AppShell sidebar={<Sidebar activePage={activePage} collapsed={sidebarCollapsed} onNavigate={navigate} />} topbar={<Topbar activePage={activePage} onRefresh={refresh} sidebarCollapsed={sidebarCollapsed} onToggleSidebar={() => setSidebarCollapsed((current) => !current)} />} tabs={<WorkspaceTabs tabs={openTabs} activeId={activePage} onActivate={setActivePage} onClose={closeTab} onRefresh={refresh} onCloseOthers={closeOtherTabs} onCloseRight={closeRightTabs} />} sidebarCollapsed={sidebarCollapsed} overlays={overlays}>{page}</AppShell>;
}
