import React from "react";
import { Bell, CalendarClock, CheckCircle2, ChevronRight, CircleDashed, CircleOff, Eye, Inbox, Layers3, PlugZap, Plus, Radio, TriangleAlert, XCircle } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { LoadingList } from "../components/ui/Skeleton";
import { eventLabel } from "../features/notifications/notificationPresentation";
import { formatDate } from "../lib/formatters";

const statusOrder = ["healthy", "triggered", "failed", "waiting", "unavailable", "disabled"];
const statusDefinitions = {
  healthy: { label: "运行正常", tone: "success", icon: CheckCircle2 },
  triggered: { label: "已触发", tone: "warning", icon: TriangleAlert },
  failed: { label: "执行失败", tone: "warning", icon: XCircle },
  waiting: { label: "等待执行", tone: "muted", icon: CircleDashed },
  unavailable: { label: "插件不可用", tone: "warning", icon: PlugZap },
  disabled: { label: "已停用", tone: "muted", icon: CircleOff }
};

export function OverviewPage({ monitors = [], modules = [], channels = [], recentNotifications = [], unreadCount = 0, loading, onCreate, onOpenMonitor, onOpenMonitors, onOpenInbox, onOpenNotification }) {
  const monitorSummary = summarizeMonitors(monitors);
  const recentRuns = monitors
    .filter((monitor) => monitor.runtime_state?.last_run_at || monitor.runtime_state?.last_summary)
    .sort((left, right) => dateValue(right.runtime_state?.last_run_at) - dateValue(left.runtime_state?.last_run_at))
    .slice(0, 6);
  const upcomingRuns = monitors
    .filter((monitor) => monitor.enabled && monitor.module_available !== false && monitor.next_run_at)
    .sort((left, right) => dateValue(left.next_run_at) - dateValue(right.next_run_at))
    .slice(0, 5);
  const moduleRows = summarizeModules(monitors, modules);
  const enabledChannels = channels.filter((channel) => channel.enabled).length;

  return <div className="page-stack overview-page">
    <PageHeader
      className="overview-heading"
      eyebrow="SYSTEM OVERVIEW"
      title="监控总览"
      description="快速掌握监控状态、最近执行摘要和待处理事件。"
      actions={<div className="page-heading-actions overview-heading-actions"><Button variant="outline" onClick={onOpenMonitors}>查看监控项<ChevronRight size={15} /></Button><Button onClick={onCreate}><Plus size={16} />新建监控</Button></div>}
    />

    <div className="stat-grid overview-stat-grid">
      <StatCard label="监控项" value={loading ? "—" : monitorSummary.total} hint={loading ? "正在加载监控状态" : `${monitorSummary.enabled} 个已启用 · ${monitorSummary.disabled} 个已停用`} icon={Layers3} />
      <StatCard label="运行正常" value={loading ? "—" : monitorSummary.healthy} hint={loading ? "正在读取执行结果" : `${percentage(monitorSummary.successful, monitorSummary.runnable)}% 可运行监控最近一次成功`} icon={CheckCircle2} />
      <StatCard label="需要关注" value={loading ? "—" : monitorSummary.attention} hint={loading ? "正在检查异常状态" : `${monitorSummary.triggered} 个触发 · ${monitorSummary.failed} 个失败 · ${monitorSummary.unavailable} 个插件不可用`} icon={TriangleAlert} tone={monitorSummary.attention ? "warning" : "neutral"} />
      <StatCard label="通知渠道" value={loading ? "—" : enabledChannels} hint={loading ? "正在读取通知配置" : `${channels.length} 个已配置 · ${unreadCount} 条未读通知`} icon={Bell} />
    </div>

    <div className="overview-primary-grid">
      <Card className="overview-panel overview-status-panel">
        <SectionHeader title="状态分布" description="按当前运行状态查看监控覆盖情况。" action={<Button variant="ghost" onClick={onOpenMonitors}>查看列表<ChevronRight size={15} /></Button>} />
        <div className="overview-status-list">{statusOrder.map((key) => <StatusRow key={key} status={key} count={monitorSummary.statuses[key]} total={monitorSummary.total} />)}</div>
      </Card>

      <Card className="overview-panel overview-events-panel">
        <SectionHeader title="最近事件" description={unreadCount ? `${unreadCount} 条通知未读` : "暂无未读通知"} action={<Button variant="ghost" onClick={onOpenInbox}>通知中心<ChevronRight size={15} /></Button>} />
        {recentNotifications.length ? <div className="overview-event-list">{recentNotifications.slice(0, 5).map((notification) => <button type="button" className={`overview-event-row ${notification.read ? "is-read" : ""}`} key={notification.id} onClick={() => onOpenNotification?.(notification)}>
          <span className={`overview-event-dot overview-event-${notification.event_type || "other"}`} />
          <span className="overview-event-copy"><span><strong>{notification.title}</strong><Badge tone={notification.event_type === "triggered" ? "warning" : notification.event_type === "recovered" ? "success" : "muted"}>{eventLabel(notification.event_type)}</Badge></span><small>{notification.content || "暂无通知内容"}</small><time>{formatDate(notification.created_at)}</time></span>
          <ChevronRight size={14} className="overview-event-arrow" />
        </button>)}</div> : <div className="overview-empty"><Inbox size={18} /><span>监控触发或恢复后，事件会显示在这里。</span></div>}
      </Card>
    </div>

    <Card className="overview-panel overview-execution-panel">
      <SectionHeader title="最近执行摘要" description="展示各监控最近一次采集返回的摘要、结果和事件状态。" action={<Button variant="ghost" onClick={onOpenMonitors}>全部监控<ChevronRight size={15} /></Button>} />
      {loading ? <LoadingList count={4} /> : recentRuns.length ? <div className="overview-execution-list">{recentRuns.map((monitor) => <ExecutionSummaryRow key={monitor.id} monitor={monitor} descriptor={modules.find((module) => module.type === monitor.module_type)} onOpen={onOpenMonitor} />)}</div> : <EmptyState icon={Inbox} title={monitors.length ? "尚未有执行记录" : "还没有监控项"} description={monitors.length ? "监控执行后，最近一次结果摘要会显示在这里。" : "创建第一个监控，开始收集运行结果。"} action={!monitors.length && <Button onClick={onCreate}><Plus size={15} />创建监控</Button>} />}
    </Card>

    <div className="overview-secondary-grid">
      <Card className="overview-panel overview-modules-panel">
        <SectionHeader title="采集模块" description={`${modules.length} 个模块已注册，覆盖 ${moduleRows.filter((item) => item.count > 0).length} 种监控类型。`} />
        {moduleRows.length ? <div className="overview-module-list">{moduleRows.map((module) => <div className="overview-module-row" key={module.type}>
          <span className="overview-module-icon"><Radio size={16} /></span><span className="overview-module-copy"><strong>{module.name}</strong><small>{module.type}</small></span><span className="overview-module-count"><strong>{module.count}</strong><small>监控项</small></span>{module.unavailable ? <Badge tone="warning">不可用</Badge> : <Badge tone={module.count ? "success" : "muted"}>{module.count ? "使用中" : "未使用"}</Badge>}
        </div>)}</div> : <div className="overview-empty"><Radio size={18} /><span>暂无可用的采集模块。</span></div>}
      </Card>

      <Card className="overview-panel overview-schedule-panel">
        <SectionHeader title="即将执行" description="按计划排列的下一批监控任务。" />
        {upcomingRuns.length ? <div className="overview-schedule-list">{upcomingRuns.map((monitor) => <button type="button" className="overview-schedule-row" key={monitor.id} onClick={() => onOpenMonitor?.(monitor)}><span className="overview-schedule-time"><CalendarClock size={15} /><time>{formatDate(monitor.next_run_at, "暂无计划")}</time></span><span className="overview-schedule-name"><strong>{monitor.name}</strong><small>{monitor.schedules?.[0] || "未配置表达式"}</small></span><ChevronRight size={14} /></button>)}</div> : <div className="overview-empty"><CalendarClock size={18} /><span>{monitors.length ? "当前没有可安排的监控任务。" : "创建监控后，下一次执行时间会显示在这里。"}</span></div>}
      </Card>
    </div>
  </div>;
}

function SectionHeader({ title, description, action }) {
  return <div className="section-header overview-section-header"><div><h2>{title}</h2><p>{description}</p></div>{action}</div>;
}

function StatCard({ label, value, hint, icon: Icon, tone = "neutral" }) {
  return <Card className="stat-card"><div className="stat-top"><span>{label}</span><div className={`stat-icon stat-icon-${tone}`}><Icon size={17} /></div></div><strong>{value}</strong><small>{hint}</small></Card>;
}

function StatusRow({ status, count, total }) {
  const definition = statusDefinitions[status];
  const Icon = definition.icon;
  const ratio = percentage(count, total);
  return <div className={`overview-status-row overview-status-row-${status}`}><span className={`overview-status-icon overview-status-${definition.tone}`}><Icon size={15} /></span><span className="overview-status-name">{definition.label}</span><span className="overview-status-track"><span style={{ width: `${ratio}%` }} /></span><strong>{count}</strong></div>;
}

function ExecutionSummaryRow({ monitor, descriptor, onOpen }) {
  const status = monitorStatus(monitor);
  const configSummary = listSummary(monitor, descriptor);
  const summary = compactSummary(monitor.runtime_state?.last_summary);
  return <div className="overview-execution-row">
    <div className="overview-execution-identity"><span className="overview-module-icon"><Radio size={16} /></span><span><strong>{monitor.name}</strong><small>{descriptor?.name || monitor.module_type?.toUpperCase() || "未知模块"}{configSummary ? ` · ${configSummary}` : ""}</small></span></div>
    <div className="overview-execution-copy"><p title={monitor.runtime_state?.last_summary || summary}>{summary}</p><span><MonitorStatusBadge monitor={monitor} /><time>{formatDate(monitor.runtime_state?.last_run_at)}</time></span></div>
    <Button variant="ghost" className="overview-execution-open" title="打开执行详情" aria-label={`打开 ${monitor.name} 的执行详情`} onClick={() => onOpen?.(monitor)}><Eye size={15} /></Button>
  </div>;
}

function MonitorStatusBadge({ monitor }) {
  const status = monitorStatus(monitor);
  return <Badge tone={statusDefinitions[status].tone}>{statusDefinitions[status].label}</Badge>;
}

function summarizeMonitors(monitors) {
  const statuses = Object.fromEntries(statusOrder.map((key) => [key, 0]));
  monitors.forEach((monitor) => { statuses[monitorStatus(monitor)] += 1; });
  const successful = monitors.filter((monitor) => monitor.enabled && monitor.module_available !== false && monitor.runtime_state?.last_success).length;
  const runnable = monitors.length - statuses.disabled - statuses.unavailable;
  return { total: monitors.length, statuses, enabled: monitors.length - statuses.disabled, healthy: statuses.healthy, successful, runnable, triggered: statuses.triggered, failed: statuses.failed, unavailable: statuses.unavailable, disabled: statuses.disabled, attention: statuses.triggered + statuses.failed + statuses.unavailable };
}

function summarizeModules(monitors, modules) {
  const knownTypes = new Set(modules.map((module) => module.type));
  const rows = modules.map((module) => ({ type: module.type, name: module.name || module.type, count: monitors.filter((monitor) => monitor.module_type === module.type).length, unavailable: false }));
  const missingTypes = [...new Set(monitors.filter((monitor) => !knownTypes.has(monitor.module_type)).map((monitor) => monitor.module_type).filter(Boolean))];
  return [...rows, ...missingTypes.map((type) => ({ type, name: type.toUpperCase(), count: monitors.filter((monitor) => monitor.module_type === type).length, unavailable: true }))];
}

function monitorStatus(monitor) {
  if (!monitor.enabled) return "disabled";
  if (monitor.module_available === false) return "unavailable";
  if (monitor.runtime_state?.condition_active) return "triggered";
  if (monitor.runtime_state?.last_success) return "healthy";
  if (monitor.runtime_state?.last_run_at) return "failed";
  return "waiting";
}

function listSummary(monitor, descriptor) {
  const fields = descriptor?.list_summary?.fields;
  if (!Array.isArray(fields)) return "";
  const config = objectValue(monitor.module_config);
  const secretFields = new Set((descriptor.parameters || []).filter((parameter) => parameter.secret).map((parameter) => parameter.key));
  return fields.filter((field) => !secretFields.has(field)).map((field) => config[field]).filter((value) => value !== undefined && value !== null && value !== "" && typeof value !== "object").join(descriptor.list_summary.separator || " · ");
}

function objectValue(value) {
  if (!value) return {};
  if (typeof value === "object") return value;
  try { return JSON.parse(value); } catch { return {}; }
}

function compactSummary(value) {
  const summary = String(value || "").replace(/\s+/g, " ").trim();
  return summary || "尚未执行";
}

function dateValue(value) {
  const parsed = value ? new Date(value).getTime() : 0;
  return Number.isFinite(parsed) ? parsed : 0;
}

function percentage(value, total) {
  return total ? Math.round((value / total) * 100) : 0;
}
