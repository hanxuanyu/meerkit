import React, { useCallback, useEffect, useState } from "react";
import { ExternalLink, History, RefreshCw, Search } from "lucide-react";
import { api } from "../../lib/api";
import { formatDate } from "../../lib/formatters";
import { Badge } from "../../components/ui/Badge";
import { Card } from "../../components/ui/Card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { IconButton } from "../../components/ui/IconButton";
import { Input } from "../../components/ui/Input";
import { Pagination } from "../../components/ui/Pagination";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/Table";
import { ResultRenderer } from "../../components/results/ResultRenderer";
import { MonitorSummary } from "../../components/monitor/MonitorSummary";

export function MonitorRecordsDialog({ monitor, descriptor, onClose, onOpenTab }) {
  return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent className="modal-wide"><DialogHeader><div><span className="eyebrow">EXECUTION HISTORY</span><DialogTitle>{monitor.name} · 执行记录</DialogTitle><DialogDescription>查看监控内容、每次采集结果、条件状态和通知结果。</DialogDescription></div><IconButton className="records-open-tab" size="default" title="在页签中打开" aria-label="在页签中打开" onClick={() => onOpenTab(monitor)}><ExternalLink size={16} /></IconButton></DialogHeader><div className="modal-body records-modal-body"><MonitorSummary monitor={monitor} descriptor={descriptor} /><MonitorRecordsPanel monitor={monitor} descriptor={descriptor} /></div></DialogContent></Dialog>;
}

export function MonitorRecordsPage({ monitor, descriptor }) {
  return <div className="page-stack"><div className="page-heading"><div><div className="eyebrow">MONITOR DETAIL</div><h1>{monitor.name}</h1><p>查看监控内容、执行记录和结果集详情。</p></div><div className="settings-icon"><History size={18} /></div></div><Card className="section-card"><MonitorSummary monitor={monitor} descriptor={descriptor} /><div className="records-section-divider" /><MonitorRecordsPanel monitor={monitor} descriptor={descriptor} /></Card></div>;
}

function MonitorRecordsPanel({ monitor, descriptor }) {
  const [records, setRecords] = useState([]);
  const [pageInfo, setPageInfo] = useState({ page: 1, page_size: 20, total: 0, total_pages: 0 });
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [eventType, setEventType] = useState("all");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedRecord, setSelectedRecord] = useState(null);

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

  useEffect(() => { void loadRecords(); }, [loadRecords]);
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

  return <><div className="records-toolbar"><div><strong>{pageInfo.total}</strong><span> 条执行记录</span></div><IconButton variant="outline" size="default" title="刷新记录" aria-label="刷新记录" onClick={() => void loadRecords()}><RefreshCw size={15} /></IconButton></div><div className="records-filters"><div className="records-search"><Search size={14} /><Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索错误、事件、状态或结果内容" aria-label="搜索执行记录" /></div><Select value={status} onValueChange={updateFilter(setStatus)}><SelectTrigger className="records-filter-select" aria-label="按执行结果筛选"><SelectValue placeholder="全部结果" /></SelectTrigger><SelectContent><SelectItem value="all">全部结果</SelectItem><SelectItem value="success">成功</SelectItem><SelectItem value="failed">失败</SelectItem></SelectContent></Select><Select value={eventType} onValueChange={updateFilter(setEventType)}><SelectTrigger className="records-filter-select" aria-label="按事件筛选"><SelectValue placeholder="全部事件" /></SelectTrigger><SelectContent><SelectItem value="all">全部事件</SelectItem><SelectItem value="triggered">触发</SelectItem><SelectItem value="recovered">恢复</SelectItem><SelectItem value="none">无事件</SelectItem></SelectContent></Select></div>{loading ? <div className="records-empty">正在加载执行记录...</div> : error ? <div className="records-empty field-error">{error}</div> : records.length === 0 ? <div className="records-empty">{pageInfo.total === 0 && (search || status !== "all" || eventType !== "all") ? "没有匹配的执行记录" : "暂无执行记录"}</div> : <><Table><TableHeader><TableRow><TableHead>开始时间</TableHead><TableHead>耗时</TableHead><TableHead>执行结果</TableHead><TableHead>条件状态</TableHead><TableHead>事件</TableHead><TableHead>错误</TableHead></TableRow></TableHeader><TableBody>{records.map((record) => <TableRow key={record.id} className="record-row" onClick={() => setSelectedRecord(record)}><TableCell>{formatDate(record.started_at)}</TableCell><TableCell>{record.duration_ms} ms</TableCell><TableCell><Badge tone={record.success ? "success" : "warning"}>{record.success ? "成功" : "失败"}</Badge></TableCell><TableCell>{record.condition_state || "unknown"}</TableCell><TableCell>{record.event_type || "none"}</TableCell><TableCell><span className="record-error" title={record.error_message || "-"}>{record.error_message || "-"}</span></TableCell></TableRow>)}</TableBody></Table><Pagination page={pageInfo.page} pageSize={pageInfo.page_size} total={pageInfo.total} onPageChange={(page) => setPageInfo((current) => ({ ...current, page }))} onPageSizeChange={changePageSize} disabled={loading} /></>}{selectedRecord && <RecordDetailDialog monitor={monitor} record={selectedRecord} descriptor={descriptor} onClose={() => setSelectedRecord(null)} />}</>;
}

function RecordDetailDialog({ monitor, record, descriptor, onClose }) {
  return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent className="modal-wide record-detail-dialog"><DialogHeader><div><span className="eyebrow">RECORD DETAIL</span><DialogTitle>执行详情</DialogTitle><DialogDescription>{formatDate(record.started_at)} · {record.duration_ms} ms</DialogDescription></div></DialogHeader><div className="modal-body record-detail-body"><MonitorSummary monitor={monitor} descriptor={descriptor} /><div className="records-section-divider" /><RecordDetailContent record={record} descriptor={descriptor} /></div></DialogContent></Dialog>;
}

function RecordDetailContent({ record, descriptor }) {
  return <><div className="record-detail-grid"><div><span>执行结果</span><strong>{record.success ? "成功" : "失败"}</strong></div><div><span>条件状态</span><strong>{record.condition_state || "unknown"}</strong></div><div><span>事件类型</span><strong>{record.event_type || "none"}</strong></div><div><span>结果哈希</span><code>{record.result_hash || "-"}</code></div></div>{record.error_message && <div className="record-error-block">{record.error_message}</div>}<section><h3>采集结果</h3><ResultRenderer descriptor={descriptor} result={record.result} /></section><section><h3>通知结果</h3><pre>{JSON.stringify(record.notification_result || {}, null, 2)}</pre></section></>;
}
