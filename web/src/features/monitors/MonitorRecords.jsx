import React, { useCallback, useEffect, useState } from "react";
import { Copy, Edit3, ExternalLink, Gauge, LoaderCircle, Play, RefreshCw, Search, Trash2 } from "lucide-react";
import { api } from "../../lib/api";
import { formatDate } from "../../lib/formatters";
import { PageHeader } from "../../components/layout/PageHeader";
import { Badge } from "../../components/ui/Badge";
import { ActionMenu } from "../../components/ui/ActionMenu";
import { Card } from "../../components/ui/Card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { DeleteConfirmDialog } from "../../components/ui/DeleteConfirmDialog";
import { IconButton } from "../../components/ui/IconButton";
import { Input } from "../../components/ui/Input";
import { Pagination } from "../../components/ui/Pagination";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/Table";
import { MonitorSummary } from "../../components/monitor/MonitorSummary";
import { Button } from "../../components/ui/Button";
import { Switch } from "../../components/ui/Switch";
import { MonitorConfigurationDetails } from "../../components/monitor/MonitorConfigurationDetails";
import { RecordDetailDialog } from "./RecordDetailDialog";
import { conditionMeta, eventMeta, recordEventType } from "./recordPresentation";

export function MonitorRecordsDialog({ monitor, descriptor, channels = [], onClose, onOpenTab, onRecordsDeleted }) {
  return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent className="modal-wide"><DialogHeader><div><span className="eyebrow">EXECUTION HISTORY</span><DialogTitle>{monitor.name} · 执行记录</DialogTitle><DialogDescription>查看监控内容、每次采集结果、条件状态和通知结果。</DialogDescription></div><IconButton className="records-open-tab" size="default" title="在页签中打开" aria-label="在页签中打开" onClick={() => onOpenTab(monitor)}><ExternalLink size={16} /></IconButton></DialogHeader><div className="modal-body records-modal-body"><MonitorSummary monitor={monitor} descriptor={descriptor} /><MonitorRecordsPanel monitor={monitor} descriptor={descriptor} channels={channels} onRecordsDeleted={onRecordsDeleted} /></div></DialogContent></Dialog>;
}

export function MonitorRecordsPage({ monitor, descriptor, channels = [], initialRecordID = "", onRecordRouteChange, onRecordsDeleted, onEdit, onDuplicate, onRun, onDelete, onToggleEnabled, togglingMonitorId = "" }) {
  const [running, setRunning] = useState(false);
  const [recordsRefreshVersion, setRecordsRefreshVersion] = useState(0);
  const moduleUnavailable = monitor.module_available === false;
  const run = async () => {
    if (!onRun || running || moduleUnavailable) return;
    setRunning(true);
    try {
      const record = await onRun(monitor);
      if (record) setRecordsRefreshVersion((current) => current + 1);
    } finally {
      setRunning(false);
    }
  };
  const actions = <div className="monitor-detail-actions"><div className="monitor-detail-enabled" title={moduleUnavailable ? "对应插件不可用，无法切换监控状态" : monitor.enabled ? "停用监控" : "启用监控"}><Switch checked={monitor.enabled} disabled={moduleUnavailable || !onToggleEnabled || togglingMonitorId === monitor.id} aria-label={moduleUnavailable ? "对应插件不可用，无法切换监控状态" : monitor.enabled ? "停用监控" : "启用监控"} onCheckedChange={() => onToggleEnabled?.(monitor)} /><span>{monitor.enabled ? "已启用" : "已停用"}</span></div><Button variant="outline" className="monitor-detail-command" title={moduleUnavailable ? "对应插件不可用，调度已暂停" : running ? "正在执行" : "立即执行"} aria-label={moduleUnavailable ? "对应插件不可用，无法执行" : running ? "正在执行" : "立即执行"} disabled={running || moduleUnavailable} onClick={() => void run()}>{running ? <LoaderCircle className="spin" size={15} /> : <Play size={15} />}<span>{moduleUnavailable ? "插件不可用" : running ? "执行中..." : "立即执行"}</span></Button><ActionMenu triggerVariant="outline" label={`${monitor.name} 操作`} items={[onEdit && { label: "编辑", icon: Edit3, disabled: moduleUnavailable, onSelect: () => onEdit(monitor) }, onDuplicate && { label: "复制", icon: Copy, disabled: moduleUnavailable, onSelect: () => onDuplicate(monitor) }, onDelete && { label: "删除", icon: Trash2, destructive: true, onSelect: () => onDelete(monitor) }]} /></div>;
  return <div className="page-stack"><PageHeader eyebrow="MONITOR DETAIL" title={monitor.name} description={moduleUnavailable ? "对应插件当前不可用，定时调度已暂停；插件恢复后将自动继续。" : "查看监控配置、执行计划、触发条件和历史记录。"} /><Card className="section-card monitor-detail-card"><div className="monitor-detail-toolbar"><span><Gauge size={15} />监控控制</span>{actions}</div><MonitorConfigurationDetails monitor={monitor} descriptor={descriptor} channels={channels} /><div className="records-section-divider" /><MonitorRecordsPanel monitor={monitor} descriptor={descriptor} channels={channels} initialRecordID={initialRecordID} onRecordRouteChange={onRecordRouteChange} onRecordsDeleted={onRecordsDeleted} refreshVersion={recordsRefreshVersion} /></Card></div>;
}

function MonitorRecordsPanel({ monitor, descriptor, channels = [], initialRecordID = "", onRecordRouteChange, onRecordsDeleted, refreshVersion = 0 }) {
  const [records, setRecords] = useState([]);
  const [pageInfo, setPageInfo] = useState({ page: 1, page_size: 20, total: 0, total_pages: 0 });
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [eventType, setEventType] = useState("all");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedRecord, setSelectedRecord] = useState(null);
  const [confirmDeleteRecords, setConfirmDeleteRecords] = useState(false);
  const [deletingRecords, setDeletingRecords] = useState(false);

  const loadRecords = useCallback(async () => {
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ page: String(pageInfo.page), page_size: String(pageInfo.page_size) });
    if (search) params.set("q", search);
    if (status !== "all") params.set("status", status);
    if (eventType !== "all") params.set("event_type", eventType);
    try {
      const response = await api(`/api/v1/monitors/${monitor.id}/records?${params.toString()}`);
      setRecords(response?.items || []);
      setPageInfo((current) => ({ ...current, page: response?.page || current.page, page_size: response?.page_size || current.page_size, total: response?.total || 0, total_pages: response?.total_pages || 0 }));
    } catch (loadError) {
      setError(loadError.message);
      setRecords([]);
    } finally {
      setLoading(false);
    }
  }, [eventType, monitor.id, pageInfo.page, pageInfo.page_size, search, status]);

  useEffect(() => { void loadRecords(); }, [loadRecords, refreshVersion]);
  useEffect(() => {
    if (initialRecordID === null) return undefined;
    if (!initialRecordID) {
      if (onRecordRouteChange) setSelectedRecord(null);
      return;
    }
    if (selectedRecord?.id === initialRecordID) return;
    let cancelled = false;
    api(`/api/v1/monitors/${monitor.id}/records/${initialRecordID}`)
      .then((record) => { if (!cancelled) setSelectedRecord(record); })
      .catch((loadError) => {
        if (!cancelled) {
          setError(loadError.message);
          onRecordRouteChange?.("");
        }
      });
    return () => { cancelled = true; };
  }, [initialRecordID, monitor.id, onRecordRouteChange, selectedRecord?.id]);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(searchInput.trim());
      setPageInfo((current) => current.page === 1 ? current : { ...current, page: 1 });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  const updateFilter = (setter) => (value) => {
    setter(value);
    setPageInfo((current) => ({ ...current, page: 1 }));
  };
  const changePageSize = (value) => setPageInfo((current) => ({ ...current, page: 1, page_size: value }));
  const openRecord = async (record) => {
    try { setSelectedRecord(await api(`/api/v1/monitors/${monitor.id}/records/${record.id}`)); }
    catch (loadError) { setError(loadError.message); return; }
    onRecordRouteChange?.(record.id);
  };
  const closeRecord = () => {
    setSelectedRecord(null);
    onRecordRouteChange?.("");
  };
  const deleteRecords = async (event) => {
    event.preventDefault();
    if (deletingRecords) return;
    setDeletingRecords(true);
    try {
      const response = await api(`/api/v1/monitors/${monitor.id}/records`, { method: "DELETE" });
      closeRecord();
      setRecords([]);
      setPageInfo((current) => ({ ...current, page: 1, total: 0, total_pages: 0 }));
      setConfirmDeleteRecords(false);
      onRecordsDeleted?.(response?.deleted || 0);
    } catch (deleteError) {
      setError(deleteError.message);
    } finally {
      setDeletingRecords(false);
    }
  };

  return <>
    <div className="records-toolbar"><div><strong>{pageInfo.total}</strong><span> 条执行记录</span></div><div className="records-toolbar-actions"><Button variant="outline" size="sm" className="destructive-outline" disabled={loading || pageInfo.total === 0} onClick={() => setConfirmDeleteRecords(true)}><Trash2 size={14} />清空记录</Button><IconButton variant="outline" size="default" title="刷新记录" aria-label="刷新记录" onClick={() => void loadRecords()}><RefreshCw size={15} /></IconButton></div></div>
    <div className="records-filters"><div className="records-search"><Search size={14} /><Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索错误、事件、状态或结果内容" aria-label="搜索执行记录" /></div><Select value={status} onValueChange={updateFilter(setStatus)}><SelectTrigger className="records-filter-select" aria-label="按执行结果筛选"><SelectValue placeholder="全部结果" /></SelectTrigger><SelectContent><SelectItem value="all">全部结果</SelectItem><SelectItem value="success">成功</SelectItem><SelectItem value="failed">失败</SelectItem></SelectContent></Select><Select value={eventType} onValueChange={updateFilter(setEventType)}><SelectTrigger className="records-filter-select" aria-label="按事件筛选"><SelectValue placeholder="全部事件" /></SelectTrigger><SelectContent><SelectItem value="all">全部事件</SelectItem><SelectItem value="triggered">触发</SelectItem><SelectItem value="recovered">恢复</SelectItem><SelectItem value="trend_triggered">趋势触发</SelectItem><SelectItem value="trend_recovered">趋势恢复</SelectItem><SelectItem value="none">无事件</SelectItem></SelectContent></Select></div>
    {loading ? <div className="records-empty">正在加载执行记录...</div> : error ? <div className="records-empty field-error">{error}</div> : records.length === 0 ? <div className="records-empty">{pageInfo.total === 0 && (search || status !== "all" || eventType !== "all") ? "没有匹配的执行记录" : "暂无执行记录"}</div> : <><Table className="records-table"><TableHeader><TableRow><TableHead>开始时间</TableHead><TableHead>耗时</TableHead><TableHead>执行结果</TableHead><TableHead>条件状态</TableHead><TableHead>事件</TableHead><TableHead>错误</TableHead></TableRow></TableHeader><TableBody>{records.map((record) => { const condition = conditionMeta(record.condition_state); const event = eventMeta(recordEventType(record)); return <TableRow key={record.id} className="record-row" onClick={() => openRecord(record)}><TableCell className="record-primary-cell" data-label="开始时间"><span>{formatDate(record.started_at)}</span><span className="record-mobile-badges"><Badge tone={record.success ? "success" : "warning"}>{record.success ? "成功" : "失败"}</Badge><Badge tone={condition.tone}>{condition.label}</Badge><Badge tone={event.tone}>{event.label}</Badge></span></TableCell><TableCell className="record-duration-cell" data-label="耗时">{record.duration_ms} ms</TableCell><TableCell className="record-result-cell" data-label="执行结果"><Badge tone={record.success ? "success" : "warning"}>{record.success ? "成功" : "失败"}</Badge></TableCell><TableCell className="record-condition-cell" data-label="条件状态">{condition.label}</TableCell><TableCell className="record-event-cell" data-label="事件">{event.label}</TableCell><TableCell className="record-error-cell" data-label="错误"><span className="record-error" title={record.error_message || "-"}>{record.error_message || "-"}</span></TableCell></TableRow>; })}</TableBody></Table><Pagination page={pageInfo.page} pageSize={pageInfo.page_size} total={pageInfo.total} onPageChange={(page) => setPageInfo((current) => ({ ...current, page }))} onPageSizeChange={changePageSize} disabled={loading} /></>}
    {selectedRecord && <RecordDetailDialog record={selectedRecord} descriptor={selectedRecord.descriptor || descriptor} channels={channels} onClose={closeRecord} />}
    <DeleteConfirmDialog open={confirmDeleteRecords} onOpenChange={(open) => !open && !deletingRecords && setConfirmDeleteRecords(false)} title="删除全部执行记录" description={`将永久删除“${monitor.name}”的全部历史执行记录。监控配置和通知不会被删除，此操作无法撤销。`} busy={deletingRecords} onConfirm={deleteRecords} />
  </>;
}
