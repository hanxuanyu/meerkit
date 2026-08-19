import React from "react";
import { Download, Eye, FileText, Package, Trash2 } from "lucide-react";
import { Badge } from "../../components/ui/Badge";
import { IconButton } from "../../components/ui/IconButton";
import { Switch } from "../../components/ui/Switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../components/ui/Table";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../../components/ui/Tooltip";
import { pluginStatusMeta, pluginTrustMeta } from "./pluginPresentation";

export function PluginTable({ plugins, busy, onDetails, onLogs, onToggleEnabled, onUninstall }) {
  return <TooltipProvider delayDuration={220}><Table className="plugin-table">
    <colgroup><col /><col className="plugin-col-modules" /><col className="plugin-col-trust" /><col className="plugin-col-status" /><col className="plugin-col-toggle" /><col className="plugin-col-actions" /></colgroup>
    <TableHeader><TableRow><TableHead>插件</TableHead><TableHead>模块</TableHead><TableHead>信任状态</TableHead><TableHead>运行状态</TableHead><TableHead className="toggle-cell">启用</TableHead><TableHead className="action-cell">操作</TableHead></TableRow></TableHeader>
    <TableBody>{plugins.map((plugin) => <PluginRow key={`${plugin.id}@${plugin.version}`} plugin={plugin} busy={busy} onDetails={onDetails} onLogs={onLogs} onToggleEnabled={onToggleEnabled} onUninstall={onUninstall} />)}</TableBody>
  </Table></TooltipProvider>;
}

function PluginRow({ plugin, busy, onDetails, onLogs, onToggleEnabled, onUninstall }) {
  const key = `${plugin.id}@${plugin.version}`;
  const path = `/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}`;
  const status = pluginStatusMeta(plugin.status);
  const trust = pluginTrustMeta(plugin.trust_state);
  return <TableRow>
    <TableCell className="plugin-primary-cell"><div className="plugin-title-layout"><div className="plugin-mark"><Package size={15} /></div><button type="button" className="plugin-identity plugin-identity-button" onClick={() => onDetails(plugin)}><strong>{plugin.name}</strong><span>{plugin.id} · {plugin.version}</span>{plugin.desp && <small>{plugin.desp}</small>}</button><span className="plugin-mobile-badges"><Badge tone={trust.tone}>{trust.label}</Badge><Badge tone={status.tone}>{status.label}</Badge></span></div></TableCell>
    <TableCell className="plugin-modules-cell" data-label="模块"><PluginModuleSummary modules={plugin.modules || []} /></TableCell>
    <TableCell className="plugin-trust-cell" data-label="信任状态"><Badge tone={trust.tone}>{trust.label}</Badge></TableCell>
    <TableCell className="plugin-status-cell" data-label="运行状态"><Badge tone={status.tone}>{status.label}</Badge>{plugin.error && <small className="plugin-error" title={plugin.error}>{plugin.error}</small>}</TableCell>
    <TableCell className="toggle-cell plugin-toggle-cell" data-label="启用"><span className="plugin-switch" title={plugin.enabled ? "禁用插件" : "启用插件"}><Switch checked={plugin.enabled} disabled={Boolean(busy)} aria-label={plugin.enabled ? `禁用${plugin.name}` : `启用${plugin.name}`} onCheckedChange={() => onToggleEnabled(plugin)} /></span></TableCell>
    <TableCell className="action-cell plugin-action-cell" data-label="操作"><div className="plugin-actions"><IconButton size="sm" title="查看插件详情" aria-label="查看插件详情" onClick={() => onDetails(plugin)}><Eye size={15} /></IconButton><IconButton size="sm" title="实时查看插件日志" aria-label="实时查看插件日志" onClick={() => onLogs(plugin)}><FileText size={15} /></IconButton>{plugin.trust_state !== "development" && <a className="plugin-action-link" href={`${path}/export`} title="导出插件包" aria-label="导出插件包"><Download size={15} /></a>}<IconButton size="sm" title="卸载插件" aria-label="卸载插件" disabled={Boolean(busy) || busy === `${key}:delete`} onClick={() => onUninstall(plugin)}><Trash2 size={15} /></IconButton></div></TableCell>
  </TableRow>;
}

function PluginModuleSummary({ modules }) {
  if (!modules.length) return <span className="plugin-modules-empty">无模块</span>;
  return <Tooltip><TooltipTrigger asChild><button type="button" className="plugin-module-count" aria-label={`查看 ${modules.length} 个模块`}><Package size={14} /><strong>{modules.length}</strong><span>个模块</span></button></TooltipTrigger><TooltipContent className="plugin-module-tooltip" side="top" align="start"><strong>包含模块</strong><div>{modules.map((module) => <span key={module.type}><b>{module.name || module.type}</b><code>{module.type}</code></span>)}</div></TooltipContent></Tooltip>;
}
