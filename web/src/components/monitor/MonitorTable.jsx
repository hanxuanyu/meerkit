import React from "react";
import { Edit3, Globe2, MoreHorizontal, Play, Server, Trash2 } from "lucide-react";
import { formatDate } from "../../lib/formatters";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../ui/Table";

export function MonitorTable({ monitors, onOpen, onRun, onEdit, onDelete }) {
  return <Table><TableHeader><TableRow><TableHead>监控项</TableHead><TableHead>模块</TableHead><TableHead>调度</TableHead><TableHead>最近执行</TableHead><TableHead>状态</TableHead><TableHead className="action-cell">操作</TableHead></TableRow></TableHeader><TableBody>{monitors.map((monitor) => <TableRow key={monitor.id} onClick={() => onOpen(monitor)}><TableCell><div className="monitor-name"><div className={`module-mark module-${monitor.module_type}`}>{monitor.module_type === "http" ? <Globe2 size={15} /> : <Server size={15} />}</div><div><strong>{monitor.name}</strong><span>{monitor.module_config?.url || `${monitor.module_config?.host || "-"}:${monitor.module_config?.port || "-"}`}</span></div></div></TableCell><TableCell><Badge variant="outline">{monitor.module_type.toUpperCase()}</Badge></TableCell><TableCell><code>{monitor.schedule}</code></TableCell><TableCell><span className="last-run">{formatDate(monitor.runtime_state?.last_run_at)}</span></TableCell><TableCell><StatusBadge monitor={monitor} /></TableCell><TableCell className="action-cell" onClick={(event) => event.stopPropagation()}><div className="row-actions"><Button variant="ghost" size="icon" title="立即执行" onClick={() => onRun(monitor)}><Play size={15} /></Button>{onEdit && <Button variant="ghost" size="icon" title="编辑" onClick={() => onEdit(monitor)}><Edit3 size={15} /></Button>}{onDelete && <Button variant="ghost" size="icon" title="删除" onClick={() => onDelete(monitor)}><Trash2 size={15} /></Button>}<Button variant="ghost" size="icon" title="更多"><MoreHorizontal size={15} /></Button></div></TableCell></TableRow>)}</TableBody></Table>;
}

function StatusBadge({ monitor }) {
  if (!monitor.enabled) return <Badge tone="muted">已停用</Badge>;
  if (monitor.runtime_state?.condition_active) return <Badge tone="warning"><span className="status-dot" />已触发</Badge>;
  if (monitor.runtime_state?.last_success) return <Badge tone="success"><span className="status-dot" />正常</Badge>;
  return <Badge tone="muted"><span className="status-dot" />等待执行</Badge>;
}
