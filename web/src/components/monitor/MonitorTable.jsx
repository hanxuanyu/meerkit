import React from "react";
import { Edit3, Eye, Globe2, Play, Power, PowerOff, Server, Trash2 } from "lucide-react";
import { formatDate } from "../../lib/formatters";
import { Badge } from "../ui/Badge";
import { IconButton } from "../ui/IconButton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../ui/Table";

export function MonitorTable({ monitors, onRun, onEdit, onDelete, onViewRecords, onToggleEnabled, togglingMonitorId }) {
  return <Table><TableHeader><TableRow><TableHead>监控项</TableHead><TableHead>模块</TableHead><TableHead>调度</TableHead><TableHead>最近执行</TableHead><TableHead>状态</TableHead><TableHead className="action-cell">操作</TableHead></TableRow></TableHeader><TableBody>{monitors.map((monitor) => { const endpoint = monitor.module_config?.url || `${monitor.module_config?.host || "-"}:${monitor.module_config?.port || "-"}`; return <TableRow key={monitor.id}><TableCell><div className="monitor-name"><div className={`module-mark module-${monitor.module_type}`}>{monitor.module_type === "http" ? <Globe2 size={15} /> : <Server size={15} />}</div><div><strong title={monitor.name}>{monitor.name}</strong><span title={endpoint}>{endpoint}</span></div></div></TableCell><TableCell><Badge variant="outline">{monitor.module_type.toUpperCase()}</Badge></TableCell><TableCell><div className="schedule-summary">{(monitor.schedules || []).map((schedule) => <code title={schedule} key={schedule}>{schedule}</code>)}</div></TableCell><TableCell><span className="last-run">{formatDate(monitor.runtime_state?.last_run_at)}</span></TableCell><TableCell><StatusBadge monitor={monitor} /></TableCell><TableCell className="action-cell"><div className="row-actions">{onViewRecords && <IconButton size="sm" title="打开监控详情" aria-label="打开监控详情" onClick={() => onViewRecords(monitor)}><Eye size={15} /></IconButton>}<IconButton size="sm" title="立即执行" aria-label="立即执行" onClick={() => onRun(monitor)}><Play size={15} /></IconButton>{onToggleEnabled && <IconButton size="sm" title={monitor.enabled ? "停用监控" : "启用监控"} aria-label={monitor.enabled ? "停用监控" : "启用监控"} disabled={togglingMonitorId === monitor.id} onClick={() => onToggleEnabled(monitor)}>{monitor.enabled ? <PowerOff size={15} /> : <Power size={15} />}</IconButton>}{onEdit && <IconButton size="sm" title="编辑" aria-label="编辑" onClick={() => onEdit(monitor)}><Edit3 size={15} /></IconButton>}{onDelete && <IconButton size="sm" title="删除" aria-label="删除" onClick={() => onDelete(monitor)}><Trash2 size={15} /></IconButton>}</div></TableCell></TableRow>; })}</TableBody></Table>;
}

function StatusBadge({ monitor }) {
  if (!monitor.enabled) return <Badge tone="muted">已停用</Badge>;
  if (monitor.runtime_state?.condition_active) return <Badge tone="warning"><span className="status-dot" />已触发</Badge>;
  if (monitor.runtime_state?.last_success) return <Badge tone="success"><span className="status-dot" />正常</Badge>;
  return <Badge tone="muted"><span className="status-dot" />等待执行</Badge>;
}
