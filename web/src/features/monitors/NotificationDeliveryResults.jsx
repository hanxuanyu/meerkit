import React from "react";
import { Bell, CheckCircle2, CircleDashed, Mail, MinusCircle, Send, Webhook, XCircle } from "lucide-react";
import { CollapsibleText } from "../../components/results/CollapsibleText";
import { Badge } from "../../components/ui/Badge";

const builtInChannelID = "builtin-inapp";

export function NotificationDeliveryResults({ result = {}, eventType = "none", channels = [], expandedTextKey = "", onToggleText = () => {} }) {
  const deliveries = Object.entries(result || {});
  if (!deliveries.length) {
    const notified = eventType === "triggered" || eventType === "recovered";
    return <div className="notification-delivery-empty"><CircleDashed size={17} /><div><strong>{notified ? "暂无投递结果" : "本次执行无需发送通知"}</strong><span>{notified ? "通知可能仍在发送，刷新执行记录后可查看最新状态。" : "只有触发或恢复事件才会执行通知渠道。"}</span></div></div>;
  }

  const channelMap = new Map(channels.map((channel) => [channel.id, channel]));
  return <div className="notification-delivery-list">{deliveries.map(([channelID, raw]) => {
    const channel = channelMap.get(channelID);
    const delivery = normalizeDelivery(raw);
    const status = deliveryStatus(delivery.status);
    const channelType = channel?.notifier_type || (channelID === builtInChannelID ? "inapp" : "");
    const ChannelIcon = channelIcon(channelType);
    const StatusIcon = status.icon;
    const channelName = channel?.name || (channelID === builtInChannelID ? "站内通知" : channelID);
    return <div className="notification-delivery-row" key={channelID}>
      <span className="notification-delivery-icon"><ChannelIcon size={16} /></span>
      <div className="notification-delivery-main"><strong>{channelName}</strong><span>{channelType ? channelType.toUpperCase() : "未知渠道"} · {channelID}</span></div>
      <div className="notification-delivery-status"><Badge tone={status.tone}><StatusIcon size={11} />{status.label}</Badge>{delivery.attempts > 0 && <span>尝试 {delivery.attempts} 次</span>}</div>
      {delivery.message && <CollapsibleText className="notification-delivery-message" label="投递信息" value={deliveryMessage(delivery.message)} expanded={expandedTextKey === `delivery:${channelID}`} onToggle={() => onToggleText(`delivery:${channelID}`)} />}
    </div>;
  })}</div>;
}

function normalizeDelivery(value) {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return { status: String(value.status || "unknown"), attempts: Number(value.attempts) || 0, message: value.message ? String(value.message) : "" };
  }
  return { status: "unknown", attempts: 0, message: value === undefined || value === null ? "" : String(value) };
}

function deliveryStatus(status) {
  return ({
    sent: { label: "已发送", tone: "success", icon: CheckCircle2 },
    error: { label: "发送失败", tone: "danger", icon: XCircle },
    skipped: { label: "已跳过", tone: "muted", icon: MinusCircle },
    pending: { label: "发送中", tone: "warning", icon: CircleDashed },
  })[status] || { label: status === "unknown" ? "状态未知" : status, tone: "muted", icon: CircleDashed };
}

function channelIcon(type) {
  if (type === "inapp") return Bell;
  if (type === "smtp") return Mail;
  if (type === "webhook") return Webhook;
  return Send;
}

function deliveryMessage(message) {
  if (message === "channel disabled") return "通知渠道已停用";
  if (message === "unknown notifier type") return "未找到对应的通知器";
  return message;
}
