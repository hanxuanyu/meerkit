import React from "react";
import { formatDate } from "../../lib/formatters";
import { ResultRenderer } from "../../components/results/ResultRenderer";
import { Badge } from "../../components/ui/Badge";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { conditionMeta, eventMeta } from "./recordPresentation";

export function RecordDetailDialog({ record, descriptor, onClose }) {
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="modal-wide record-detail-dialog">
      <DialogHeader><div><span className="eyebrow">RECORD DETAIL</span><DialogTitle>执行详情</DialogTitle><DialogDescription>{formatDate(record.started_at)} · {record.duration_ms} ms</DialogDescription></div></DialogHeader>
      <div className="modal-body record-detail-body"><RecordDetailContent record={record} descriptor={descriptor} /></div>
    </DialogContent>
  </Dialog>;
}

function RecordDetailContent({ record, descriptor }) {
  const condition = conditionMeta(record.condition_state);
  const event = eventMeta(record.event_type);
  return <>
    <div className="record-detail-badges"><RecordMetaBadge label="执行结果" value={record.success ? "成功" : "失败"} tone={record.success ? "success" : "warning"} /><RecordMetaBadge label="条件状态" value={condition.label} tone={condition.tone} /><RecordMetaBadge label="事件类型" value={event.label} tone={event.tone} /><RecordMetaBadge label="执行耗时" value={`${record.duration_ms} ms`} /><RecordMetaBadge className="record-hash-badge" label="结果哈希" value={record.result_hash || "-"} mono /></div>
    {record.error_message && <div className="record-error-block">{record.error_message}</div>}
    <section><h3>采集结果</h3><ResultRenderer descriptor={descriptor} result={record.result} /></section>
    <section><h3>通知结果</h3><pre>{JSON.stringify(record.notification_result || {}, null, 2)}</pre></section>
  </>;
}

function RecordMetaBadge({ label, value, tone = "neutral", mono = false, className = "" }) {
  return <Badge variant="outline" tone={tone} className={`record-meta-badge ${className}`} title={`${label}：${value}`}><span>{label}</span><strong className={mono ? "record-meta-mono" : ""}>{value}</strong></Badge>;
}
