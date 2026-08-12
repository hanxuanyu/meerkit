import React, { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { FolderSync, KeyRound, LoaderCircle, PackageOpen, RefreshCw, Search, ShieldAlert, Trash2, Upload } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "../components/ui/AlertDialog";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { IconButton } from "../components/ui/IconButton";
import { Input } from "../components/ui/Input";
import { Pagination } from "../components/ui/Pagination";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/Select";
import { LoadingList } from "../components/ui/Skeleton";
import { PluginLogsDialog } from "../features/plugins/PluginLogsDialog";
import { PluginTable } from "../features/plugins/PluginTable";
import { api } from "../lib/api";

const PluginDetailDialog = lazy(() => import("../features/plugins/PluginDetailDialog").then((module) => ({ default: module.PluginDetailDialog })));

const initialPage = { page: 1, page_size: 20, total: 0, total_pages: 0 };

export function PluginsPage({ notify, onChanged }) {
  const [plugins, setPlugins] = useState([]);
  const [pageInfo, setPageInfo] = useState(initialPage);
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [trustState, setTrustState] = useState("all");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const [detail, setDetail] = useState(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [logPlugin, setLogPlugin] = useState(null);
  const [confirmation, setConfirmation] = useState(null);
  const fileInput = useRef(null);
  const promptedSigners = useRef(new Set());

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true);
    if (!silent) setError("");
    const params = new URLSearchParams({ page: String(pageInfo.page), page_size: String(pageInfo.page_size) });
    if (search) params.set("q", search);
    if (status !== "all") params.set("status", status);
    if (trustState !== "all") params.set("trust_state", trustState);
    try {
      const value = await api(`/api/v1/plugins?${params.toString()}`);
      setPlugins(value?.items || []);
      setPageInfo((current) => ({ ...current, page: value?.page || 1, page_size: value?.page_size || current.page_size, total: value?.total || 0, total_pages: value?.total_pages || 0 }));
    } catch (loadError) {
      if (!silent) {
        setPlugins([]);
        setError(loadError.message);
        notify(loadError.message, "error");
      }
    } finally {
      if (!silent) setLoading(false);
    }
  }, [notify, pageInfo.page, pageInfo.page_size, search, status, trustState]);

  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    const interval = window.setInterval(() => { void load(true); }, 10000);
    return () => window.clearInterval(interval);
  }, [load]);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(searchInput.trim());
      setPageInfo((current) => current.page === 1 ? current : { ...current, page: 1 });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  const openDetails = useCallback(async (plugin) => {
    setDetail(plugin);
    setDetailLoading(true);
    try {
      const value = await api(`/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}`);
      setDetail(value);
    } catch (detailError) {
      notify(detailError.message, "error");
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

  const perform = async (key, action, success) => {
    setBusy(key);
    try {
      const value = await action();
      notify(success);
      await load(true);
      await onChanged?.();
      return { ok: true, value };
    } catch (actionError) {
      notify(actionError.message, "error");
      return { ok: false, value: null };
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
    await perform("upload", () => api("/api/v1/plugins/import", { method: "POST", body: data }), "插件包已导入，启用后即可创建对应监控项");
  };

  const toggleEnabled = (plugin) => {
    const key = `${plugin.id}@${plugin.version}:power`;
    const path = `/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}`;
    if (plugin.enabled) {
      void perform(key, () => api(`${path}/disable`, { method: "POST", body: "{}" }), "插件已禁用");
      return;
    }
    if (plugin.trust_state === "untrusted") {
      void openDetails(plugin);
      return;
    }
    if (!plugin.verified) {
      setConfirmation({ type: "enable", plugin });
      return;
    }
    void perform(key, () => api(`${path}/enable`, { method: "POST", body: JSON.stringify({ confirm_unverified: false }) }), "插件已启用");
  };

  const confirmAction = async () => {
    if (!confirmation) return;
    const { plugin, type } = confirmation;
    const key = `${plugin.id}@${plugin.version}:${type}`;
    const path = `/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}`;
    let result;
    if (type === "trust") {
      result = await perform(key, () => api(`${path}/trust`, { method: "POST", body: JSON.stringify({ fingerprint: plugin.signer_fingerprint }) }), "插件发布者已加入本地信任库");
      if (result.ok) setDetail((current) => current ? { ...current, ...result.value, readme: current.readme || "" } : current);
    } else if (type === "enable") {
      result = await perform(key, () => api(`${path}/enable`, { method: "POST", body: JSON.stringify({ confirm_unverified: true }) }), "插件已启用");
    } else {
      result = await perform(key, () => api(path, { method: "DELETE" }), "插件已卸载");
      if (result.ok && detail?.id === plugin.id && detail?.version === plugin.version) setDetail(null);
    }
    if (result.ok) setConfirmation(null);
  };

  const updateFilter = (setter) => (value) => {
    setter(value);
    setPageInfo((current) => ({ ...current, page: 1 }));
  };
  const hasFilters = Boolean(search || status !== "all" || trustState !== "all");

  return <div className="page-stack">
    <PageHeader eyebrow="EXTENSIONS" title="监控插件" description="管理当前平台可用的监控类型与插件进程。" />
    <Card className="section-card plugin-card">
      <div className="toolbar list-toolbar plugin-toolbar"><div className="monitor-search list-toolbar-search"><Search size={15} /><Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索名称、ID、开发者或模块" aria-label="搜索插件" /></div><div className="monitor-filters list-toolbar-actions"><Select value={status} onValueChange={updateFilter(setStatus)}><SelectTrigger className="monitor-filter-select" aria-label="按运行状态筛选"><SelectValue placeholder="全部状态" /></SelectTrigger><SelectContent><SelectItem value="all">全部状态</SelectItem><SelectItem value="enabled">已启用</SelectItem><SelectItem value="disabled">已停用</SelectItem><SelectItem value="healthy">运行中</SelectItem><SelectItem value="degraded">异常</SelectItem></SelectContent></Select><Select value={trustState} onValueChange={updateFilter(setTrustState)}><SelectTrigger className="monitor-filter-select" aria-label="按信任状态筛选"><SelectValue placeholder="全部信任状态" /></SelectTrigger><SelectContent><SelectItem value="all">全部信任</SelectItem><SelectItem value="development">开发源码</SelectItem><SelectItem value="official">官方可信</SelectItem><SelectItem value="trusted">已验证</SelectItem><SelectItem value="untrusted">待信任</SelectItem><SelectItem value="unsigned">未签名</SelectItem></SelectContent></Select><IconButton variant="outline" size="default" title="刷新" aria-label="刷新" disabled={loading} onClick={() => { void load(); }}><RefreshCw className={loading ? "spin" : ""} size={15} /></IconButton><input ref={fileInput} hidden type="file" accept=".zip,.tar.gz,application/zip,application/gzip" onChange={upload} /><Button size="sm" onClick={() => fileInput.current?.click()} disabled={busy === "upload"}>{busy === "upload" ? <LoaderCircle className="spin" size={15} /> : <Upload size={15} />}导入插件</Button><IconButton variant="outline" size="default" title="扫描插件目录" aria-label="扫描插件目录" disabled={Boolean(busy)} onClick={() => { void perform("scan", () => api("/api/v1/plugins/scan", { method: "POST" }), "插件目录扫描完成"); }}><FolderSync size={15} /></IconButton></div></div>
      {loading ? <LoadingList /> : error ? <div className="records-empty field-error">{error}</div> : plugins.length ? <><PluginTable plugins={plugins} busy={busy} onDetails={(plugin) => { void openDetails(plugin); }} onLogs={setLogPlugin} onToggleEnabled={toggleEnabled} onUninstall={(plugin) => setConfirmation({ type: "uninstall", plugin })} /><Pagination page={pageInfo.page} pageSize={pageInfo.page_size} total={pageInfo.total} onPageChange={(page) => setPageInfo((current) => ({ ...current, page }))} onPageSizeChange={(page_size) => setPageInfo((current) => ({ ...current, page: 1, page_size }))} disabled={loading} /></> : hasFilters ? <EmptyState icon={Search} title="没有匹配的插件" description="尝试调整搜索关键词或筛选条件。" /> : <EmptyState icon={PackageOpen} title="尚未安装插件" description="导入 zip 或 tar.gz 插件包后，可在此处启用对应监控类型。" action={<Button onClick={() => fileInput.current?.click()}><Upload size={15} />导入插件</Button>} />}
    </Card>
    {detail && <Suspense fallback={null}><PluginDetailDialog plugin={detail} loading={detailLoading} trusting={busy === `${detail.id}@${detail.version}:trust`} onClose={() => setDetail(null)} onTrust={() => setConfirmation({ type: "trust", plugin: detail })} /></Suspense>}
    {logPlugin && <PluginLogsDialog plugin={logPlugin} onClose={() => setLogPlugin(null)} />}
    <PluginActionDialog action={confirmation} busy={Boolean(busy)} onOpenChange={(open) => !open && !busy && setConfirmation(null)} onConfirm={() => { void confirmAction(); }} />
  </div>;
}

function PluginActionDialog({ action, busy, onOpenChange, onConfirm }) {
  const type = action?.type;
  const plugin = action?.plugin;
  const content = type === "trust"
    ? { icon: KeyRound, title: "信任插件发布者", description: <>确认前请从独立来源核对以下公钥指纹。信任后，同一公钥签名的其他插件会自动通过验证。<code className="plugin-confirm-fingerprint">{plugin?.signer_fingerprint}</code></>, action: "确认信任", variant: "default" }
    : type === "enable"
      ? { icon: ShieldAlert, title: "启用未验证插件", description: `“${plugin?.name || "此插件"}”没有可信签名，无法确认发布者身份。仅在确认插件来源和内容后继续。`, action: "仍然启用", variant: "destructive" }
      : { icon: Trash2, title: "卸载插件", description: `确定卸载“${plugin?.name || "此插件"}”吗？仍被监控项引用的插件无法卸载。`, action: "确认卸载", variant: "destructive" };
  const Icon = content.icon;
  return <AlertDialog open={Boolean(action)} onOpenChange={onOpenChange}><AlertDialogContent><AlertDialogHeader><div className="alert-dialog-icon"><Icon size={18} /></div><AlertDialogTitle>{content.title}</AlertDialogTitle><AlertDialogDescription>{content.description}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={busy}>取消</AlertDialogCancel><AlertDialogAction variant={content.variant} disabled={busy} onClick={(event) => { event.preventDefault(); onConfirm(); }}>{busy ? "处理中..." : content.action}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>;
}
