import React, { useCallback, useEffect, useRef, useState } from "react";
import { Bell, BellOff, BellRing, Check, CheckCheck, ChevronRight, Circle, ExternalLink, Inbox, MailOpen, RefreshCw, Search, Trash2 } from "lucide-react";
import { api } from "../../lib/api";
import { formatDate } from "../../lib/formatters";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card } from "../../components/ui/Card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { EmptyState } from "../../components/ui/EmptyState";
import { IconButton } from "../../components/ui/IconButton";
import { Input } from "../../components/ui/Input";
import { Pagination } from "../../components/ui/Pagination";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { Switch } from "../../components/ui/Switch";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "../../components/ui/AlertDialog";

function eventLabel(type) {
  return { triggered: "触发", recovered: "恢复", test: "测试" }[type] || type || "通知";
}

function browserNotificationLabel(status) {
  return { enabled: "接收系统级提醒", denied: "已被浏览器阻止", unsupported: "当前环境不可用" }[status] || "开启后接收系统级提醒";
}

export function NotificationBell({ items = [], unreadCount = 0, onMarkRead, onMarkAllRead, onOpenCenter, onOpenNotification, browserNotificationStatus = "disabled", onToggleBrowserNotifications }) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef(null);

  useEffect(() => {
    if (!open) return undefined;
    const close = (event) => { if (!containerRef.current?.contains(event.target)) setOpen(false); };
    const escape = (event) => { if (event.key === "Escape") setOpen(false); };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", escape);
    };
  }, [open]);

  const openCenter = () => { setOpen(false); onOpenCenter(); };
  const openNotification = (notification) => { setOpen(false); onOpenNotification(notification); };

  return <div className="notification-bell" ref={containerRef}>
    <IconButton variant="ghost" size="default" title="站内通知" aria-label={`站内通知，${unreadCount} 条未读`} onClick={() => setOpen((current) => !current)}><Bell size={16} /></IconButton>
    {unreadCount > 0 && <span className="notification-badge">{unreadCount > 99 ? "99+" : unreadCount}</span>}
    {open && <div className="notification-popover">
      <div className="notification-popover-header"><div><strong>站内通知</strong><span>{unreadCount ? `${unreadCount} 条未读` : "已全部读完"}</span></div><Button variant="ghost" size="sm" disabled={!unreadCount} onClick={onMarkAllRead}><CheckCheck size={14} />全部已读</Button></div>
      <div className="browser-notification-setting"><span className="browser-notification-icon">{browserNotificationStatus === "enabled" ? <BellRing size={15} /> : <BellOff size={15} />}</span><span><strong>浏览器通知</strong><small>{browserNotificationLabel(browserNotificationStatus)}</small></span><Switch checked={browserNotificationStatus === "enabled"} aria-label={browserNotificationStatus === "enabled" ? "关闭浏览器通知" : "开启浏览器通知"} onCheckedChange={onToggleBrowserNotifications} /></div>
      <div className="notification-popover-list">{items.length ? items.map((notification) => <div className={`notification-popover-item ${notification.read ? "is-read" : ""}`} key={notification.id}>
        <button type="button" className="notification-popover-main" onClick={() => openNotification(notification)}><span className="notification-state-dot" /><span><strong>{notification.title}</strong><small>{notification.content}</small><time>{formatDate(notification.created_at)}</time></span></button>
        {!notification.read && <IconButton size="sm" title="标记为已读" aria-label={`将 ${notification.title} 标记为已读`} onClick={() => onMarkRead(notification.id)}><Check size={14} /></IconButton>}
      </div>) : <div className="notification-popover-empty"><MailOpen size={18} /><span>暂无站内通知</span></div>}</div>
      <button type="button" className="notification-popover-footer" onClick={openCenter}>进入通知中心<ChevronRight size={14} /></button>
    </div>}
  </div>;
}

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
    <div className="page-heading notification-center-heading"><div><div className="eyebrow">IN-APP NOTIFICATIONS</div><h1>通知中心</h1><p>查看监控触发、恢复和系统测试产生的站内通知。</p></div><div className="page-heading-actions"><div className="browser-notification-heading-control" title={browserNotificationLabel(browserNotificationStatus)}>{browserNotificationStatus === "enabled" ? <BellRing size={15} /> : <BellOff size={15} />}<span>浏览器通知</span><Switch checked={browserNotificationStatus === "enabled"} aria-label={browserNotificationStatus === "enabled" ? "关闭浏览器通知" : "开启浏览器通知"} onCheckedChange={onToggleBrowserNotifications} /></div><Button variant="outline" className="destructive-outline" disabled={loading} onClick={() => setConfirmDeleteRead(true)}><Trash2 size={15} />删除已读</Button><Button variant="outline" disabled={!unreadCount} onClick={markAll}><CheckCheck size={15} />全部标记已读</Button></div></div>
    <Card className="section-card notification-center-card">
      <div className="notification-center-toolbar"><div className="notification-center-search"><Search size={15} /><Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索标题或通知内容" aria-label="搜索站内通知" /></div><Select value={status} onValueChange={(value) => { setStatus(value); setPageInfo((current) => ({ ...current, page: 1 })); }}><SelectTrigger className="notification-status-select" aria-label="按已读状态筛选"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部通知</SelectItem><SelectItem value="unread">仅未读</SelectItem></SelectContent></Select><IconButton variant="outline" size="default" title="刷新" aria-label="刷新通知" onClick={() => void load()}><RefreshCw size={15} /></IconButton></div>
      {loading ? <div className="records-empty">正在加载通知...</div> : error ? <div className="records-empty field-error">{error}</div> : notifications.length ? <><div className="notification-center-list">{notifications.map((notification) => <div className={`notification-center-item ${notification.read ? "is-read" : ""}`} key={notification.id}>
        <button type="button" className="notification-center-main" onClick={() => void openNotification(notification)}><span className="notification-state-dot" /><span className="notification-center-copy"><span><strong>{notification.title}</strong><Badge tone={notification.event_type === "triggered" ? "warning" : notification.event_type === "recovered" ? "success" : "muted"}>{eventLabel(notification.event_type)}</Badge></span><small>{notification.content}</small><time>{formatDate(notification.created_at)}</time></span></button>
        {!notification.read ? <IconButton size="sm" title="标记为已读" aria-label={`将 ${notification.title} 标记为已读`} onClick={() => void markRead(notification.id)}><Circle size={14} /></IconButton> : <span className="notification-read-state"><Check size={13} />已读</span>}
      </div>)}</div><Pagination page={pageInfo.page} pageSize={pageInfo.page_size} total={pageInfo.total} onPageChange={(page) => setPageInfo((current) => ({ ...current, page }))} onPageSizeChange={(page_size) => setPageInfo((current) => ({ ...current, page: 1, page_size }))} disabled={loading} /></> : <EmptyState icon={Inbox} title="暂无站内通知" description={search || status === "unread" ? "没有匹配的通知。" : "监控事件触发后会显示在这里。"} />}
    </Card>
    {selected && <Dialog open onOpenChange={(open) => !open && closeNotification()}><DialogContent className="notification-detail-dialog"><DialogHeader><div><span className="eyebrow">NOTIFICATION DETAIL</span><DialogTitle>{selected.title}</DialogTitle><DialogDescription>{eventLabel(selected.event_type)} · {formatDate(selected.created_at)}</DialogDescription></div></DialogHeader><div className="modal-body notification-detail-body"><div className="notification-detail-meta"><div><span>状态</span><strong>{selected.read ? "已读" : "未读"}</strong></div><div><span>事件</span><strong>{eventLabel(selected.event_type)}</strong></div></div><section><h3>通知内容</h3><div className="notification-detail-content">{selected.content}</div></section></div><DialogFooter className="dialog-footer-split">{!selected.read && <Button variant="outline" onClick={() => void markRead(selected.id)}><Check size={15} />标记已读</Button>}<div className="dialog-footer-actions"><Button variant="ghost" onClick={closeNotification}>关闭</Button>{selected.monitor_id && selected.record_id && <Button onClick={() => { closeNotification(); onOpenExecution(selected.monitor_id, selected.record_id); }}><ExternalLink size={15} />查看执行详情</Button>}</div></DialogFooter></DialogContent></Dialog>}
    <AlertDialog open={confirmDeleteRead} onOpenChange={(open) => !open && !deletingRead && setConfirmDeleteRead(false)}><AlertDialogContent><AlertDialogHeader><div className="alert-dialog-icon"><Trash2 size={18} /></div><AlertDialogTitle>删除全部已读通知</AlertDialogTitle><AlertDialogDescription>将永久删除通知中心内所有已读通知，未读通知不受影响。此操作无法撤销。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={deletingRead}>取消</AlertDialogCancel><AlertDialogAction disabled={deletingRead} onClick={deleteRead}>{deletingRead ? "删除中..." : "确认删除"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </div>;
}
