import React from "react";
import { formatDate } from "../../lib/formatters";
import { CollapsibleText } from "../../components/results/CollapsibleText";
import { ResultRenderer } from "../../components/results/ResultRenderer";
import { Badge } from "../../components/ui/Badge";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { NotificationDeliveryResults } from "./NotificationDeliveryResults";
import { conditionMeta, eventMeta } from "./recordPresentation";

export function RecordDetailDialog({ record, descriptor, channels = [], onClose }) {
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="modal-wide record-detail-dialog">
      <DialogHeader><div><span className="eyebrow">RECORD DETAIL</span><DialogTitle>执行详情</DialogTitle><DialogDescription>{formatDate(record.started_at)} · {record.duration_ms} ms</DialogDescription></div></DialogHeader>
      <div className="modal-body record-detail-body"><RecordDetailContent record={record} descriptor={descriptor} channels={channels} /></div>
    </DialogContent>
  </Dialog>;
}

function RecordDetailContent({ record, descriptor, channels }) {
  const [expandedTextKey, setExpandedTextKey] = React.useState("");
  React.useEffect(() => setExpandedTextKey(""), [record.id]);
  const toggleText = React.useCallback((key) => setExpandedTextKey((current) => current === key ? "" : key), []);
  const condition = conditionMeta(record.condition_state);
  const event = eventMeta(record.event_type);
  return <>
    <div className="record-detail-badges"><RecordMetaBadge label="执行结果" value={record.success ? "成功" : "失败"} tone={record.success ? "success" : "warning"} /><RecordMetaBadge label="条件状态" value={condition.label} tone={condition.tone} /><RecordMetaBadge label="事件类型" value={event.label} tone={event.tone} /><RecordMetaBadge label="执行耗时" value={`${record.duration_ms} ms`} /><RecordMetaBadge className="record-hash-badge" label="结果哈希" value={record.result_hash || "-"} mono /></div>
    {record.error_message && <CollapsibleText className="record-error-block" label="执行错误" value={record.error_message} expanded={expandedTextKey === "record:error"} onToggle={() => toggleText("record:error")} />}
    <section><h3>采集结果</h3><ResultRenderer descriptor={descriptor} result={record.result} expandedTextKey={expandedTextKey} onToggleText={toggleText} /></section>
    <section><h3>通知投递</h3><NotificationDeliveryResults result={record.notification_result} eventType={record.event_type} channels={channels} expandedTextKey={expandedTextKey} onToggleText={toggleText} /></section>
  </>;
}

function RecordMetaBadge({ label, value, tone = "neutral", mono = false, className = "" }) {
  return <Badge variant="outline" tone={tone} className={`record-meta-badge ${className}`} title={`${label}：${value}`}><span>{label}</span><strong className={mono ? "record-meta-mono" : ""}>{value}</strong></Badge>;
}
