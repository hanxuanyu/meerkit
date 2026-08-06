import React from "react";
import { ChevronRight, Clock3, Database, Inbox, Layers3, Plus, Radio, ShieldCheck, TriangleAlert } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { MonitorTable } from "../components/monitor/MonitorTable";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { LoadingList } from "../components/ui/Skeleton";

export function OverviewPage({ monitors, modules = [], loading, onCreate, onOpen, onRun, onViewRecords }) {
  const stats = { total: monitors.length, active: monitors.filter((item) => item.enabled).length, alert: monitors.filter((item) => item.runtime_state?.condition_active).length, healthy: monitors.filter((item) => item.runtime_state?.last_success).length };
  return <div className="page-stack"><PageHeader eyebrow="SYSTEM OVERVIEW" title="监控总览" description="跟踪采集结果、运行状态和条件变化。" actions={<Button onClick={onCreate}><Plus size={16} />新建监控</Button>} /><div className="stat-grid"><StatCard label="监控项" value={stats.total} hint={`${stats.active} 个正在运行`} icon={Layers3} /><StatCard label="当前正常" value={stats.healthy} hint="最近一次执行成功" icon={ShieldCheck} /><StatCard label="活跃触发" value={stats.alert} hint={stats.alert ? "需要关注" : "暂无待处理事件"} icon={TriangleAlert} tone={stats.alert ? "warning" : "neutral"} /><StatCard label="采集模块" value={modules.length} hint={moduleSummary(modules)} icon={Radio} /></div><Card className="section-card"><div className="section-header"><div><h2>最近监控项</h2><p>所有已配置的数据采集任务</p></div><Button variant="ghost" onClick={() => onOpen(monitors[0])} disabled={!monitors.length}>查看全部<ChevronRight size={15} /></Button></div>{loading ? <LoadingList /> : monitors.length ? <MonitorTable monitors={monitors.slice(0, 6)} modules={modules} onRun={onRun} onViewRecords={onViewRecords} /> : <EmptyState icon={Inbox} title="还没有监控项" description="创建第一个监控，开始观察采集结果的变化。" action={<Button onClick={onCreate}><Plus size={16} />创建监控</Button>} />}</Card><div className="lower-grid"><Card className="info-card"><div className="info-icon"><Database size={18} /></div><div><h3>本地历史数据</h3><p>执行结果保存于 SQLite，可在监控详情中查看完整变化过程。</p></div><ChevronRight size={16} className="muted-icon" /></Card><Card className="info-card"><div className="info-icon"><Clock3 size={18} /></div><div><h3>Cron 调度</h3><p>使用标准 5 段或 6 段表达式灵活安排执行时间。</p></div><ChevronRight size={16} className="muted-icon" /></Card></div></div>;
}

function moduleSummary(modules) {
  const names = modules.map((module) => module.name || module.type).filter(Boolean);
  if (!names.length) return "暂无可用模块";
  const visible = names.slice(0, 3);
  return `${visible.join(" · ")}${names.length > visible.length ? ` · +${names.length - visible.length}` : ""}`;
}

function StatCard({ label, value, hint, icon: Icon, tone = "neutral" }) {
  return <Card className="stat-card"><div className="stat-top"><span>{label}</span><div className={`stat-icon stat-icon-${tone}`}><Icon size={17} /></div></div><strong>{value}</strong><small>{hint}</small></Card>;
}
