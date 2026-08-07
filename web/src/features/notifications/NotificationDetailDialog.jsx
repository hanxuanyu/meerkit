import React from "react";
import { Check, Eye } from "lucide-react";
import { formatDate } from "../../lib/formatters";
import { Button } from "../../components/ui/Button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { eventLabel } from "./notificationPresentation";

export function NotificationDetailDialog({ notification, onClose, onMarkRead, onOpenExecution }) {
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="notification-detail-dialog">
      <DialogHeader><div><span className="eyebrow">NOTIFICATION DETAIL</span><DialogTitle>{notification.title}</DialogTitle><DialogDescription>{eventLabel(notification.event_type)} · {formatDate(notification.created_at)}</DialogDescription></div></DialogHeader>
      <div className="modal-body notification-detail-body"><div className="notification-detail-meta"><div><span>状态</span><strong>{notification.read ? "已读" : "未读"}</strong></div><div><span>事件</span><strong>{eventLabel(notification.event_type)}</strong></div></div><section><h3>通知内容</h3><div className="notification-detail-content">{notification.content}</div></section></div>
      <DialogFooter className="dialog-footer-split">{!notification.read && <Button variant="outline" onClick={() => void onMarkRead(notification.id)}><Check size={15} />标记已读</Button>}<div className="dialog-footer-actions"><Button variant="ghost" onClick={onClose}>关闭</Button>{notification.monitor_id && notification.record_id && <Button onClick={() => { onClose(); onOpenExecution(notification.monitor_id, notification.record_id); }}><Eye size={15} />查看执行详情</Button>}</div></DialogFooter>
    </DialogContent>
  </Dialog>;
}
