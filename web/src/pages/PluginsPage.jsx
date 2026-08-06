import React, { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { Download, FileText, LoaderCircle, PackageOpen, Power, RefreshCw, Trash2, Upload } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { IconButton } from "../components/ui/IconButton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/Table";
import { api } from "../lib/api";

const PluginDetailDialog = lazy(() => import("../features/plugins/PluginDetailDialog").then((module) => ({ default: module.PluginDetailDialog })));

export function PluginsPage({ notify }) {
  const [plugins, setPlugins] = useState([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [detail, setDetail] = useState(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const fileInput = useRef(null);
  const promptedSigners = useRef(new Set());

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    try {
      const value = await api("/api/v1/plugins");
      setPlugins(value.items || []);
    } catch (error) {
      if (!silent) notify(error.message, "error");
    } finally {
      if (!silent) setLoading(false);
    }
  }, [notify]);

  useEffect(() => {
    void load();
    const interval = window.setInterval(() => { void load(true); }, 10000);
    return () => window.clearInterval(interval);
  }, [load]);

  const openDetails = useCallback(async (plugin) => {
    setDetail(plugin);
    setDetailLoading(true);
    try {
      const value = await api(`/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}`);
      setDetail(value);
    } catch (error) {
      notify(error.message, "error");
    } finally {
      setDetailLoading(false);
    }
  }, [notify]);

  useEffect(() => {
    if (detail) return;
    const pending = plugins.find((plugin) => plugin.trust_state === "untrusted" && !promptedSigners.current.has(plugin.signer_fingerprint));
    if (!pending) return;
    promptedSigners.current.add(pending.signer_fingerprint);
    void openDetails(pending);
  }, [detail, openDetails, plugins]);

  const run = async (key, action, success) => {
    setBusy(key);
    try {
      await action();
      notify(success);
      await load();
    } catch (error) {
      notify(error.message, "error");
    } finally {
      setBusy("");
    }
  };

  const trustPublisher = async () => {
    if (!detail?.signer_fingerprint) return;
    const confirmed = window.confirm(`确认信任此插件发布者吗？\n\n${detail.signer_fingerprint}\n\n信任后，由此公钥签名的其他插件将自动验证。`);
    if (!confirmed) return;
    const key = `${detail.id}@${detail.version}:trust`;
    setBusy(key);
    try {
      const value = await api(`/api/v1/plugins/${encodeURIComponent(detail.id)}/${encodeURIComponent(detail.version)}/trust`, { method: "POST", body: JSON.stringify({ fingerprint: detail.signer_fingerprint }) });
      setDetail((current) => ({ ...current, ...value, readme: current?.readme || "" }));
      notify("插件发布者已加入本地信任库");
      await load();
    } catch (error) {
      notify(error.message, "error");
    } finally {
      setBusy("");
    }
  };

  const upload = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    const data = new FormData();
    data.append("package", file);
    await run("upload", () => api("/api/v1/plugins/import", { method: "POST", body: data }), "插件包已导入，启用后即可创建对应监控项");
  };

  return <div className="page-stack">
    <PageHeader
      eyebrow="扩展"
      title="监控插件"
      description="管理当前平台可用的监控类型与插件进程。"
      actions={<div className="page-heading-actions">
        <input ref={fileInput} hidden type="file" accept=".zip,.tar.gz,application/zip,application/gzip" onChange={upload} />
        <Button onClick={() => fileInput.current?.click()} disabled={busy === "upload"}>
          {busy === "upload" ? <LoaderCircle className="spin" size={15} /> : <Upload size={15} />}
          导入插件包
        </Button>
        <IconButton variant="outline" title="扫描插件目录" onClick={() => run("scan", () => api("/api/v1/plugins/scan", { method: "POST" }), "插件目录扫描完成")}>
          <RefreshCw size={15} />
        </IconButton>
      </div>}
    />
    <Card className="plugin-card">
      {!loading && !plugins.length
        ? <EmptyState icon={PackageOpen} title="尚未安装插件" description="导入 zip 或 tar.gz 插件包后，可在此处启用对应监控类型。" />
        : <PluginTable plugins={plugins} busy={busy} run={run} onDetails={openDetails} />}
    </Card>
    {detail && <Suspense fallback={null}><PluginDetailDialog plugin={detail} loading={detailLoading} trusting={busy === `${detail.id}@${detail.version}:trust`} onClose={() => setDetail(null)} onTrust={() => { void trustPublisher(); }} /></Suspense>}
  </div>;
}

function PluginTable({ plugins, busy, run, onDetails }) {
  return <Table>
    <TableHeader><TableRow>
      <TableHead>插件</TableHead>
      <TableHead>模块</TableHead>
      <TableHead>信任状态</TableHead>
      <TableHead>运行状态</TableHead>
      <TableHead className="plugin-actions-column">操作</TableHead>
    </TableRow></TableHeader>
    <TableBody>{plugins.map((plugin) => <PluginRow key={`${plugin.id}@${plugin.version}`} plugin={plugin} busy={busy} run={run} onDetails={onDetails} />)}</TableBody>
  </Table>;
}

function PluginRow({ plugin, busy, run, onDetails }) {
  const key = `${plugin.id}@${plugin.version}`;
  const path = `/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}`;
  const toggle = () => {
    if (plugin.trust_state === "untrusted") {
      void onDetails(plugin);
      return;
    }
    const confirmed = plugin.verified || window.confirm("此插件未经过可信签名验证。确认承担风险并启用吗？");
    if (!confirmed) return;
    void run(
      `${key}:power`,
      () => api(`${path}/${plugin.enabled ? "disable" : "enable"}`, {
        method: "POST",
        body: JSON.stringify(plugin.enabled ? {} : { confirm_unverified: !plugin.verified }),
      }),
      plugin.enabled ? "插件已禁用" : "插件已启用",
    );
  };
  const uninstall = () => {
    if (window.confirm(`确定卸载“${plugin.name}”吗？`)) {
      void run(`${key}:delete`, () => api(path, { method: "DELETE" }), "插件已卸载");
    }
  };

  return <TableRow>
    <TableCell><button type="button" className="plugin-identity plugin-identity-button" onClick={() => { void onDetails(plugin); }}><strong>{plugin.name}</strong><span>{plugin.id} · {plugin.version}</span>{plugin.desp && <small>{plugin.desp}</small>}</button></TableCell>
    <TableCell><div className="plugin-modules">{(plugin.modules || []).map((module) => <Badge key={module.type} variant="outline">{module.name || module.type}</Badge>)}</div></TableCell>
    <TableCell><TrustBadge plugin={plugin} /></TableCell>
    <TableCell>
      <Badge tone={plugin.status === "healthy" ? "success" : plugin.status === "degraded" ? "danger" : "muted"}>{statusLabel(plugin.status)}</Badge>
      {plugin.error && <small className="plugin-error">{plugin.error}</small>}
    </TableCell>
    <TableCell><div className="plugin-actions">
      <IconButton title={plugin.enabled ? "禁用插件" : "启用插件"} disabled={Boolean(busy)} onClick={toggle}>
        {busy === `${key}:power` ? <LoaderCircle className="spin" size={15} /> : <Power size={15} />}
      </IconButton>
      <IconButton title="查看插件日志" onClick={() => window.open(`${path}/logs`, "_blank", "noopener,noreferrer")}><FileText size={15} /></IconButton>
      <IconButton title="导出插件包" onClick={() => window.location.assign(`${path}/export`)}><Download size={15} /></IconButton>
      <IconButton title="卸载插件" disabled={Boolean(busy)} onClick={uninstall}><Trash2 size={15} /></IconButton>
    </div></TableCell>
  </TableRow>;
}

function statusLabel(status) {
  return ({ healthy: "运行中", installed: "未启用", disabled: "已禁用", degraded: "异常", starting: "启动中", updating: "更新中" })[status] || status;
}

function TrustBadge({ plugin }) {
  if (plugin.trust_state === "official") return <Badge tone="success">官方可信</Badge>;
  if (plugin.trust_state === "trusted") return <Badge tone="success">已验证</Badge>;
  if (plugin.trust_state === "untrusted") return <Badge tone="warning">待信任</Badge>;
  return <Badge tone="muted">未签名</Badge>;
}
