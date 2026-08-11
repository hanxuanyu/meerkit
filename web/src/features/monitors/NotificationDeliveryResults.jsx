import React from "react";
import { Bell, CheckCircle2, CircleDashed, Mail, MinusCircle, Send, Webhook, XCircle } from "lucide-react";
import { CollapsibleText } from "../../components/results/CollapsibleText";
import { Badge } from "../../components/ui/Badge";

const builtInChannelID = "builtin-inapp";

export function NotificationDeliveryResults({ events = [], channels = [], expandedTextKey = "", onToggleText = () => {} }) {
  if (!events.length) return <div className="notification-delivery-empty"><CircleDashed size={17} /><div><strong>本次执行无需发送通知</strong><span>没有条件或趋势规则产生触发、恢复事件。</span></div></div>;
  const channelMap = new Map(channels.map((channel) => [channel.id, channel]));
  return <div className="notification-event-list">{events.map((event) => {
    const deliveries = Object.entries(event.deliveries || {});
    return <section className="notification-event-group" key={event.id}><header><div><strong>{event.source === "status_trend" ? event.status_item_name || "状态趋势" : "监控条件"}</strong><span>{notificationEventLabel(event.event_type)}{event.trend_rule_name ? ` · ${event.trend_rule_name}` : ""}</span></div><Badge tone={event.event_type.includes("recovered") ? "success" : "warning"}>{notificationEventLabel(event.event_type)}</Badge></header><p>{event.summary}</p>{event.source === "status_trend" && <TrendDetail detail={event.trend_detail} />}{deliveries.length ? <div className="notification-delivery-list">{deliveries.map(([channelID, raw]) => {
		const channel = channelMap.get(channelID);
    const delivery = normalizeDelivery(raw);
    const status = deliveryStatus(delivery.status);
    const channelType = channel?.notifier_type || (channelID === builtInChannelID ? "inapp" : "");
    const ChannelIcon = channelIcon(channelType);
    const StatusIcon = status.icon;
    const channelName = channel?.name || (channelID === builtInChannelID ? "站内通知" : channelID);
		return <div className="notification-delivery-row" key={`${event.id}:${channelID}`}>
      <span className="notification-delivery-icon"><ChannelIcon size={16} /></span>
      <div className="notification-delivery-main"><strong>{channelName}</strong><span>{channelType ? channelType.toUpperCase() : "未知渠道"} · {channelID}</span></div>
      <div className="notification-delivery-status"><Badge tone={status.tone}><StatusIcon size={11} />{status.label}</Badge>{delivery.attempts > 0 && <span>尝试 {delivery.attempts} 次</span>}</div>
			{delivery.message && <CollapsibleText className="notification-delivery-message" label="投递信息" value={deliveryMessage(delivery.message)} expanded={expandedTextKey === `delivery:${event.id}:${channelID}`} onToggle={() => onToggleText(`delivery:${event.id}:${channelID}`)} />}
		</div>;
	})}</div> : <div className="notification-delivery-empty compact"><MinusCircle size={15} /><div><strong>未选择通知渠道</strong><span>事件已记录，但没有需要投递的渠道。</span></div></div>}</section>;
  })}</div>;
}

function TrendDetail({ detail = {} }) {
  const entries = Object.entries(detail || {}).filter(([, value]) => value !== undefined && value !== null && value !== "");
  if (!entries.length) return null;
  const labels = { window: "窗口", abnormal_count: "异常次数", minimum: "最低次数", value: "实际值", threshold: "阈值", operator: "比较", reason: "原因" };
  const operators = { gt: ">", gte: ">=", lt: "<", lte: "<=", eq: "=", neq: "不等于" };
  return <div className="notification-trend-detail">{entries.map(([key, value]) => <span key={key}><small>{labels[key] || key}</small><strong>{key === "operator" ? operators[value] || value : typeof value === "object" ? JSON.stringify(value) : String(value)}</strong></span>)}</div>;
}

function notificationEventLabel(type) {
  return ({ triggered: "条件触发", recovered: "条件恢复", trend_triggered: "趋势触发", trend_recovered: "趋势恢复" })[type] || type;
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
