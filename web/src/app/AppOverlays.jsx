import React from "react";
import { AlertTriangle } from "lucide-react";
import { Toaster } from "../components/ui/Toast";
import { DeleteConfirmDialog } from "../components/ui/DeleteConfirmDialog";
import { ChannelDialog } from "../features/notifications/ChannelDialog";
import { MonitorDialog } from "../features/monitors/MonitorDialog";
import { MonitorRecordsDialog } from "../features/monitors/MonitorRecords";
import { RecordDetailDialog } from "../features/monitors/RecordDetailDialog";

export function AppOverlays({ monitorDialog, recordsDialog, executionDetailDialog, channelDialog, deleteMonitorDialog }) {
  return <>
    <Toaster position="bottom-right" richColors />
    {monitorDialog.open && <MonitorDialog monitor={monitorDialog.monitor} modules={monitorDialog.modules} channels={monitorDialog.channels} onClose={monitorDialog.onClose} onSaved={monitorDialog.onSaved} onError={monitorDialog.onError} onTest={monitorDialog.onTest} />}
    {recordsDialog.context && <MonitorRecordsDialog monitor={recordsDialog.context.monitor} descriptor={recordsDialog.context.descriptor} channels={recordsDialog.channels} onClose={recordsDialog.onClose} onOpenTab={recordsDialog.onOpenTab} onRecordsDeleted={recordsDialog.onRecordsDeleted} />}
    {executionDetailDialog.state && <RecordDetailDialog record={executionDetailDialog.state.record} descriptor={executionDetailDialog.state.record?.descriptor} channels={executionDetailDialog.channels} loading={executionDetailDialog.state.loading} error={executionDetailDialog.state.error} onClose={executionDetailDialog.onClose} />}
    {channelDialog.open && <ChannelDialog channel={channelDialog.channel} notifiers={channelDialog.notifiers} monitors={channelDialog.monitors} modules={channelDialog.modules} onClose={channelDialog.onClose} onSaved={channelDialog.onSaved} onError={channelDialog.onError} onTest={channelDialog.onTest} />}
    <DeleteConfirmDialog open={Boolean(deleteMonitorDialog.target)} onOpenChange={deleteMonitorDialog.onOpenChange} title="删除监控项" description={`确定要删除“${deleteMonitorDialog.target?.name || ""}”吗？相关配置和历史执行记录将一并删除，此操作无法撤销。`} busy={deleteMonitorDialog.busy} onConfirm={deleteMonitorDialog.onConfirm} icon={AlertTriangle} iconSize={19} />
  </>;
}
