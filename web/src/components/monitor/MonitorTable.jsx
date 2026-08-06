import React from "react";
import { Edit3, Eye, Play, Radio, Trash2 } from "lucide-react";
import { formatDate } from "../../lib/formatters";
import { Badge } from "../ui/Badge";
import { IconButton } from "../ui/IconButton";
import { Switch } from "../ui/Switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../ui/Table";

export function MonitorTable({ monitors, modules = [], onRun, onEdit, onDelete, onViewRecords, onToggleEnabled, togglingMonitorId }) {
  return <Table className="monitor-table">
    <colgroup className="monitor-table-columns"><col /><col className="monitor-col-module" /><col className="monitor-col-schedule" /><col className="monitor-col-next-run" /><col className="monitor-col-status" /><col className="monitor-col-toggle" /><col className="monitor-col-actions" /></colgroup>
    <TableHeader><TableRow><TableHead>监控项</TableHead><TableHead>模块</TableHead><TableHead>调度</TableHead><TableHead>下次执行</TableHead><TableHead>状态</TableHead><TableHead className="toggle-cell">启用</TableHead><TableHead className="action-cell">操作</TableHead></TableRow></TableHeader>
    <TableBody>{monitors.map((monitor) => {
      const descriptor = modules.find((item) => item.type === monitor.module_type);
      const summary = formatListSummary(monitor.module_config, descriptor);
      const moduleName = descriptor?.name || monitor.module_type?.toUpperCase() || "未知模块";
      const moduleUnavailable = monitor.module_available === false;
      return <TableRow key={monitor.id}>
        <TableCell className="monitor-primary-cell">
          <div className="monitor-name">
            <div className="module-mark" title={moduleName}><Radio size={15} /></div>
            <div className="monitor-title-copy">
              <strong title={monitor.name}>{monitor.name}</strong>
              {summary && <span title={summary}>{summary}</span>}
            </div>
            <span className="monitor-mobile-badges"><Badge variant="outline">{moduleName}</Badge><StatusBadge monitor={monitor} /></span>
          </div>
        </TableCell>
        <TableCell className="monitor-module-cell" data-label="模块"><Badge variant="outline">{moduleName}</Badge></TableCell>
        <TableCell className="monitor-schedule-cell" data-label="调度"><div className="schedule-summary">{(monitor.schedules || []).map((schedule) => <code title={schedule} key={schedule}>{schedule}</code>)}</div></TableCell>
        <TableCell className="monitor-next-run-cell" data-label="下次执行"><span className="last-run">{formatDate(monitor.next_run_at, !monitor.enabled ? "已停用" : moduleUnavailable ? "调度暂停" : "暂无计划")}</span></TableCell>
        <TableCell className="monitor-status-cell" data-label="状态"><StatusBadge monitor={monitor} /></TableCell>
        <TableCell className="toggle-cell monitor-toggle-cell" data-label="启用"><span className="monitor-switch" title={moduleUnavailable ? "对应插件不可用，无法切换监控状态" : monitor.enabled ? "停用监控" : "启用监控"}><Switch checked={monitor.enabled} disabled={moduleUnavailable || !onToggleEnabled || togglingMonitorId === monitor.id} aria-label={moduleUnavailable ? "对应插件不可用，无法切换监控状态" : monitor.enabled ? "停用监控" : "启用监控"} onCheckedChange={() => onToggleEnabled?.(monitor)} /></span></TableCell>
        <TableCell className="action-cell monitor-action-cell" data-label="操作"><div className="row-actions">{onViewRecords && <IconButton size="sm" title="打开监控详情" aria-label="打开监控详情" onClick={() => onViewRecords(monitor)}><Eye size={15} /></IconButton>}<IconButton size="sm" title={moduleUnavailable ? "对应插件不可用，调度已暂停" : "立即执行"} aria-label={moduleUnavailable ? "对应插件不可用，无法执行" : "立即执行"} disabled={moduleUnavailable} onClick={() => onRun(monitor)}><Play size={15} /></IconButton>{onEdit && <IconButton size="sm" title={moduleUnavailable ? "对应插件不可用，无法编辑监控" : "编辑"} aria-label={moduleUnavailable ? "对应插件不可用，无法编辑监控" : "编辑"} disabled={moduleUnavailable} onClick={() => onEdit(monitor)}><Edit3 size={15} /></IconButton>}{onDelete && <IconButton size="sm" title="删除" aria-label="删除" onClick={() => onDelete(monitor)}><Trash2 size={15} /></IconButton>}</div></TableCell>
      </TableRow>;
    })}</TableBody>
  </Table>;
}

function StatusBadge({ monitor }) {
  if (!monitor.enabled) return <Badge tone="muted">已停用</Badge>;
  if (monitor.module_available === false) return <Badge tone="warning" title="对应插件已禁用、异常或尚未加载，定时调度已暂停"><span className="status-dot" />插件不可用</Badge>;
  if (monitor.runtime_state?.condition_active) return <Badge tone="warning"><span className="status-dot" />已触发</Badge>;
  if (monitor.runtime_state?.last_success) return <Badge tone="success"><span className="status-dot" />正常</Badge>;
  return <Badge tone="muted"><span className="status-dot" />等待执行</Badge>;
}

function formatListSummary(config = {}, descriptor) {
  const declaration = descriptor?.list_summary;
  if (!Array.isArray(declaration?.fields) || declaration.fields.length === 0) return "";
  const parameters = new Map((descriptor.parameters || []).map((parameter) => [parameter.key, parameter]));
  const values = declaration.fields.flatMap((field) => {
    if (parameters.get(field)?.secret) return [];
    const value = config?.[field];
    if (value === undefined || value === null || value === "") return [];
    if (typeof value === "object") return [];
    return [String(value)];
  });
  return values.join(declaration.separator ?? " · ");
}
