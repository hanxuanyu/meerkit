import React, { useEffect, useRef, useState } from "react";
import { Bell, BellOff, BellRing, Check, CheckCheck, ChevronRight, MailOpen } from "lucide-react";
import { formatDate } from "../../lib/formatters";
import { Button } from "../../components/ui/Button";
import { IconButton } from "../../components/ui/IconButton";
import { Switch } from "../../components/ui/Switch";
import { browserNotificationLabel } from "./notificationPresentation";

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
