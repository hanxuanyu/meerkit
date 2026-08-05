import React from "react";
import { Activity, Plus, Radio, RefreshCw } from "lucide-react";
import { MonitorTable } from "../components/monitor/MonitorTable";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { LoadingList } from "../components/ui/Skeleton";

export function MonitorsPage({ monitors, loading, onCreate, onEdit, onRun, onDelete, onRefresh }) {
  return <div className="page-stack"><div className="page-heading"><div><div className="eyebrow">COLLECTION MODULES</div><h1>监控项</h1><p>配置采集模块、条件和执行计划。</p></div><Button onClick={onCreate}><Plus size={16} />新建监控</Button></div><Card className="section-card"><div className="toolbar"><div className="search-placeholder"><Activity size={15} />{monitors.length} 个监控项</div><Button variant="outline" size="icon" title="刷新" onClick={onRefresh}><RefreshCw size={15} /></Button></div>{loading ? <LoadingList /> : monitors.length ? <MonitorTable monitors={monitors} onOpen={onEdit} onRun={onRun} onEdit={onEdit} onDelete={onDelete} /> : <EmptyState icon={Radio} title="监控列表为空" description="使用独立模块采集 HTTP 或 TCP 数据。" action={<Button onClick={onCreate}><Plus size={16} />新建监控</Button>} />}</Card></div>;
}
