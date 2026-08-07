import React, { useCallback, useEffect, useState } from "react";
import { Plus, Radio, RefreshCw, Search } from "lucide-react";
import { api } from "../lib/api";
import { PageHeader } from "../components/layout/PageHeader";
import { MonitorTable } from "../components/monitor/MonitorTable";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { IconButton } from "../components/ui/IconButton";
import { Input } from "../components/ui/Input";
import { Pagination } from "../components/ui/Pagination";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/Select";
import { LoadingList } from "../components/ui/Skeleton";

export function MonitorsPage({ modules = [], onCreate, onEdit, onDuplicate, onRun, onDelete, onViewRecords, onToggleEnabled, togglingMonitorId, onRefresh, refreshVersion = 0 }) {
  const [monitors, setMonitors] = useState([]);
  const [pageInfo, setPageInfo] = useState({ page: 1, page_size: 20, total: 0, total_pages: 0 });
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [moduleType, setModuleType] = useState("all");
  const [status, setStatus] = useState("all");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const loadMonitors = useCallback(async () => {
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ page: String(pageInfo.page), page_size: String(pageInfo.page_size) });
    if (search) params.set("q", search);
    if (moduleType !== "all") params.set("module_type", moduleType);
    if (status !== "all") params.set("status", status);
    try {
      const response = await api(`/api/v1/monitors?${params.toString()}`);
      setMonitors(response?.items || []);
      setPageInfo((current) => ({ ...current, page: response?.page || current.page, page_size: response?.page_size || current.page_size, total: response?.total || 0, total_pages: response?.total_pages || 0 }));
    } catch (loadError) {
      setError(loadError.message);
      setMonitors([]);
    } finally {
      setLoading(false);
    }
  }, [moduleType, pageInfo.page, pageInfo.page_size, search, status]);

  useEffect(() => { void loadMonitors(); }, [loadMonitors, refreshVersion]);
  useEffect(() => {
    if (moduleType !== "all" && !modules.some((module) => module.type === moduleType)) {
      setModuleType("all");
      setPageInfo((current) => ({ ...current, page: 1 }));
    }
  }, [modules, moduleType]);
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

  return <div className="page-stack"><PageHeader className="page-heading-with-action" eyebrow="COLLECTION MODULES" title="监控项" description="配置采集模块、条件和执行计划。" actions={<Button onClick={onCreate}><Plus size={16} />新建监控</Button>} /><Card className="section-card"><div className="toolbar monitor-toolbar"><div className="monitor-search"><Search size={15} /><Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索名称、地址或配置内容" aria-label="搜索监控项" /></div><div className="monitor-filters"><Select value={moduleType} onValueChange={updateFilter(setModuleType)}><SelectTrigger className="monitor-filter-select" aria-label="按模块筛选"><SelectValue placeholder="全部模块" /></SelectTrigger><SelectContent><SelectItem value="all">全部模块</SelectItem>{modules.map((module) => <SelectItem key={module.type} value={module.type}>{module.name || module.type}</SelectItem>)}</SelectContent></Select><Select value={status} onValueChange={updateFilter(setStatus)}><SelectTrigger className="monitor-filter-select" aria-label="按状态筛选"><SelectValue placeholder="全部状态" /></SelectTrigger><SelectContent><SelectItem value="all">全部状态</SelectItem><SelectItem value="enabled">已启用</SelectItem><SelectItem value="disabled">已停用</SelectItem><SelectItem value="unavailable">插件不可用</SelectItem><SelectItem value="triggered">已触发</SelectItem><SelectItem value="healthy">运行正常</SelectItem><SelectItem value="waiting">等待执行</SelectItem></SelectContent></Select><IconButton variant="outline" size="default" title="刷新" aria-label="刷新" onClick={onRefresh}><RefreshCw size={15} /></IconButton></div></div>{loading ? <LoadingList /> : error ? <div className="records-empty field-error">{error}</div> : monitors.length ? <><MonitorTable monitors={monitors} modules={modules} onRun={onRun} onEdit={onEdit} onDuplicate={onDuplicate} onDelete={onDelete} onViewRecords={onViewRecords} onToggleEnabled={onToggleEnabled} togglingMonitorId={togglingMonitorId} /><Pagination page={pageInfo.page} pageSize={pageInfo.page_size} total={pageInfo.total} onPageChange={(page) => setPageInfo((current) => ({ ...current, page }))} onPageSizeChange={changePageSize} disabled={loading} /></> : pageInfo.total === 0 && (search || moduleType !== "all" || status !== "all") ? <EmptyState icon={Search} title="没有匹配的监控项" description="尝试调整搜索关键词或筛选条件。" /> : <EmptyState icon={Radio} title="监控列表为空" description="使用已注册的采集模块创建监控任务。" action={<Button onClick={onCreate}><Plus size={16} />创建监控</Button>} />}</Card></div>;
}
