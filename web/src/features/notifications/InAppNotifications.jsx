import React, { useCallback, useEffect, useState } from "react";
import { BellOff, BellRing, Check, CheckCheck, Circle, Inbox, RefreshCw, Search, Trash2 } from "lucide-react";
import { api } from "../../lib/api";
import { formatDate } from "../../lib/formatters";
import { PageHeader } from "../../components/layout/PageHeader";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card } from "../../components/ui/Card";
import { DeleteConfirmDialog } from "../../components/ui/DeleteConfirmDialog";
import { EmptyState } from "../../components/ui/EmptyState";
import { IconButton } from "../../components/ui/IconButton";
import { Input } from "../../components/ui/Input";
import { Pagination } from "../../components/ui/Pagination";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { Switch } from "../../components/ui/Switch";
import { NotificationDetailDialog } from "./NotificationDetailDialog";
import { browserNotificationLabel, eventLabel } from "./notificationPresentation";

export function NotificationCenterPage({ refreshVersion = 0, initialNotificationID = "", onNotificationRouteChange, onOpenExecution, onMarkRead, onMarkAllRead, onNotificationsDeleted, unreadCount = 0, browserNotificationStatus = "disabled", onToggleBrowserNotifications }) {
  const [notifications, setNotifications] = useState([]);
  const [pageInfo, setPageInfo] = useState({ page: 1, page_size: 20, total: 0, total_pages: 0 });
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [selected, setSelected] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [confirmDeleteRead, setConfirmDeleteRead] = useState(false);
  const [deletingRead, setDeletingRead] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ page: String(pageInfo.page), page_size: String(pageInfo.page_size) });
    if (search) params.set("q", search);
    if (status === "unread") params.set("unread", "true");
    try {
      const response = await api(`/api/v1/in-app-notifications?${params.toString()}`);
      setNotifications(response?.items || []);
      setPageInfo((current) => ({ ...current, page: response?.page || current.page, page_size: response?.page_size || current.page_size, total: response?.total || 0, total_pages: response?.total_pages || 0 }));
    } catch (loadError) {
      setError(loadError.message);
      setNotifications([]);
    } finally {
      setLoading(false);
    }
  }, [pageInfo.page, pageInfo.page_size, search, status]);

  useEffect(() => { void load(); }, [load, refreshVersion]);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(searchInput.trim());
      setPageInfo((current) => current.page === 1 ? current : { ...current, page: 1 });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [searchInput]);
  useEffect(() => {
    if (!initialNotificationID) {
      setSelected(null);
      return;
    }
    let cancelled = false;
    api(`/api/v1/in-app-notifications/${initialNotificationID}`).then((notification) => {
      if (!cancelled) {
        setSelected(notification.read ? notification : { ...notification, read: true });
        if (!notification.read) void onMarkRead(notification.id);
      }
    }).catch(() => {
      if (!cancelled) onNotificationRouteChange("");
    });
    return () => { cancelled = true; };
  }, [initialNotificationID, onMarkRead, onNotificationRouteChange]);

  const openNotification = async (notification) => {
    setSelected(notification);
    onNotificationRouteChange(notification.id);
    if (!notification.read) await onMarkRead(notification.id);
  };
  const closeNotification = () => { setSelected(null); onNotificationRouteChange(""); };
  const markAll = async () => { await onMarkAllRead(); await load(); };
  const markRead = async (id) => { await onMarkRead(id); await load(); setSelected((current) => current?.id === id ? { ...current, read: true } : current); };
  const deleteRead = async (event) => {
    event.preventDefault();
    if (deletingRead) return;
    setDeletingRead(true);
    try {
      const response = await api("/api/v1/in-app-notifications/read", { method: "DELETE" });
      if (selected?.read) closeNotification();
      setNotifications((current) => current.filter((notification) => !notification.read));
      setPageInfo((current) => ({ ...current, page: 1, total: Math.max(0, current.total - (response?.deleted || 0)) }));
      setConfirmDeleteRead(false);
      await onNotificationsDeleted?.(response?.deleted || 0);
      if (pageInfo.page === 1) await load();
    } catch (deleteError) {
      setError(deleteError.message);
    } finally {
      setDeletingRead(false);
    }
  };

  return <div className="page-stack">
    <PageHeader className="notification-center-heading" eyebrow="IN-APP NOTIFICATIONS" title="通知中心" description="查看监控触发、恢复和系统测试产生的站内通知。" actions={<div className="page-heading-actions"><div className="browser-notification-heading-control" title={browserNotificationLabel(browserNotificationStatus)}>{browserNotificationStatus === "enabled" ? <BellRing size={15} /> : <BellOff size={15} />}<span>浏览器通知</span><Switch checked={browserNotificationStatus === "enabled"} aria-label={browserNotificationStatus === "enabled" ? "关闭浏览器通知" : "开启浏览器通知"} onCheckedChange={onToggleBrowserNotifications} /></div><Button variant="outline" className="destructive-outline" disabled={loading} onClick={() => setConfirmDeleteRead(true)}><Trash2 size={15} />删除已读</Button><Button variant="outline" disabled={!unreadCount} onClick={markAll}><CheckCheck size={15} />全部标记已读</Button></div>} />
    <Card className="section-card notification-center-card">
      <div className="notification-center-toolbar"><div className="notification-center-search"><Search size={15} /><Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索标题或通知内容" aria-label="搜索站内通知" /></div><Select value={status} onValueChange={(value) => { setStatus(value); setPageInfo((current) => ({ ...current, page: 1 })); }}><SelectTrigger className="notification-status-select" aria-label="按已读状态筛选"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部通知</SelectItem><SelectItem value="unread">仅未读</SelectItem></SelectContent></Select><IconButton variant="outline" size="default" title="刷新" aria-label="刷新通知" onClick={() => void load()}><RefreshCw size={15} /></IconButton></div>
      {loading ? <div className="records-empty">正在加载通知...</div> : error ? <div className="records-empty field-error">{error}</div> : notifications.length ? <><div className="notification-center-list">{notifications.map((notification) => <div className={`notification-center-item ${notification.read ? "is-read" : ""}`} key={notification.id}>
        <button type="button" className="notification-center-main" onClick={() => void openNotification(notification)}><span className="notification-state-dot" /><span className="notification-center-copy"><span><strong>{notification.title}</strong><Badge tone={notification.event_type.includes("triggered") ? "warning" : notification.event_type.includes("recovered") ? "success" : "muted"}>{eventLabel(notification.event_type)}</Badge></span><small>{notification.content}</small><time>{formatDate(notification.created_at)}</time></span></button>
        {!notification.read ? <IconButton size="sm" title="标记为已读" aria-label={`将 ${notification.title} 标记为已读`} onClick={() => void markRead(notification.id)}><Circle size={14} /></IconButton> : <span className="notification-read-state"><Check size={13} />已读</span>}
      </div>)}</div><Pagination page={pageInfo.page} pageSize={pageInfo.page_size} total={pageInfo.total} onPageChange={(page) => setPageInfo((current) => ({ ...current, page }))} onPageSizeChange={(page_size) => setPageInfo((current) => ({ ...current, page: 1, page_size }))} disabled={loading} /></> : <EmptyState icon={Inbox} title="暂无站内通知" description={search || status === "unread" ? "没有匹配的通知。" : "监控事件触发后会显示在这里。"} />}
    </Card>
    {selected && <NotificationDetailDialog notification={selected} onClose={closeNotification} onMarkRead={markRead} onOpenExecution={onOpenExecution} />}
    <DeleteConfirmDialog open={confirmDeleteRead} onOpenChange={(open) => !open && !deletingRead && setConfirmDeleteRead(false)} title="删除全部已读通知" description="将永久删除通知中心内所有已读通知，未读通知不受影响。此操作无法撤销。" busy={deletingRead} onConfirm={deleteRead} />
  </div>;
}
