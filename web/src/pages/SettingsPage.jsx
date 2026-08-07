import React, { useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, FileCog, KeyRound, RotateCcw, Save } from "lucide-react";
import { toast } from "sonner";
import { api } from "../lib/api";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Input } from "../components/ui/Input";
import { IconButton } from "../components/ui/IconButton";
import { Switch } from "../components/ui/Switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/Tabs";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "../components/ui/AlertDialog";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../components/ui/Dialog";

const sourceLabels = {
  command_line: "命令行",
  environment: "环境变量",
  config_file: "config.yaml",
  default: "默认值"
};

const runtimeTypeLabels = {
  storage: "存储策略",
  scheduler: "调度器",
  logging: "日志运行参数",
  plugins: "插件日志",
  auth: "认证策略"
};

export function SettingsPage() {
  const [metadata, setMetadata] = useState(null);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [unsavedCounts, setUnsavedCounts] = useState({ runtime: 0, auth: 0 });
  const [resetRequest, setResetRequest] = useState(null);
  const [resetSignal, setResetSignal] = useState(null);
  const [keyDialogOpen, setKeyDialogOpen] = useState(false);

  const reload = async () => {
    const value = await api("/api/v1/system/config");
    setMetadata(value);
    setError("");
    return value;
  };

  const unsavedCount = unsavedCounts.runtime + unsavedCounts.auth;

  useEffect(() => {
    let cancelled = false;
    api("/api/v1/system/config")
      .then((value) => { if (!cancelled) setMetadata(value); })
      .catch((loadError) => { if (!cancelled) setError(loadError.message); });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (unsavedCount === 0) return undefined;
    const warnBeforeUnload = (event) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", warnBeforeUnload);
    return () => window.removeEventListener("beforeunload", warnBeforeUnload);
  }, [unsavedCount]);

  const confirmReset = async () => {
    if (!resetRequest) return;
    const target = resetRequest;
    setSaving(true);
    try {
      const endpoint = target === "all" ? "/api/v1/system/config/runtime/reset" : `/api/v1/system/config/runtime/${target}/reset`;
      await api(endpoint, { method: "POST" });
      await reload();
      setResetSignal({ target });
      toast.success(target === "all" ? "已恢复全部默认配置" : `${runtimeTypeLabels[target] || target} 已恢复默认`);
      setResetRequest(null);
    } catch (resetError) {
      setError(resetError.message);
      toast.error(resetError.message);
    } finally {
      setSaving(false);
    }
  };

  return <div className="page-stack">
    <PageHeader eyebrow="SYSTEM CONFIGURATION" title="系统设置" description="动态配置保存在数据库并可即时生效，启动配置由 YAML 和环境变量提供。" />
    {error && <div className="settings-config-error">{error}</div>}
    {unsavedCount > 0 && <div className="settings-unsaved-banner" role="alert"><AlertTriangle size={17} /><div><strong>有 {unsavedCount} 项配置尚未保存</strong><span>请点击对应配置项右侧的保存按钮，否则切换页面或刷新后修改会丢失。</span></div></div>}
    <Tabs defaultValue="runtime" className="settings-config-tabs">
      <TabsList>
        <TabsTrigger value="runtime">动态配置</TabsTrigger>
        <TabsTrigger value="startup">启动配置</TabsTrigger>
      </TabsList>
      <TabsContent value="runtime" forceMount>
        <section className="settings-config-runtime">
          <div className="section-header"><div><h2>动态配置</h2><p>配置保存在 system_configs 表中，修改后无需重启服务。</p></div><div className="settings-config-actions"><Button variant="outline" size="sm" onClick={() => setKeyDialogOpen(true)}><KeyRound size={14} />修改管理员密钥</Button><Button variant="outline" size="sm" onClick={() => setResetRequest("all")} disabled={saving}><RotateCcw size={14} />恢复全部默认</Button></div></div>
          {metadata ? <RuntimeConfigTable items={metadata.runtime_items || []} onSaved={reload} onDirtyChange={(count) => setUnsavedCounts((current) => current.runtime === count ? current : { ...current, runtime: count })} onRequestReset={setResetRequest} resetSignal={resetSignal} resetting={saving} /> : <div className="records-empty">正在加载配置...</div>}
        </section>
      </TabsContent>
      <TabsContent value="startup" forceMount>
        <section className="settings-config-startup">
          <div className="section-header"><div><h2>启动配置</h2><p>{metadata?.config_file ? `配置文件：${metadata.config_file}` : "当前使用默认配置文件路径。"}</p></div><div className="settings-config-icon"><FileCog size={17} /></div></div>
          {metadata ? <ConfigTable items={metadata.items || []} /> : <div className="records-empty">正在加载配置...</div>}
        </section>
      </TabsContent>
    </Tabs>
    <AdminKeyDialog open={keyDialogOpen} onOpenChange={setKeyDialogOpen} onDirtyChange={(count) => setUnsavedCounts((current) => current.auth === count ? current : { ...current, auth: count })} />
    <AlertDialog open={Boolean(resetRequest)} onOpenChange={(open) => { if (!open && !saving) setResetRequest(null); }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <div className="alert-dialog-icon"><RotateCcw size={18} /></div>
          <AlertDialogTitle>确认恢复默认配置？</AlertDialogTitle>
          <AlertDialogDescription>{resetRequest === "all" ? "这将恢复所有动态配置的默认值，当前数据库中的修改和未保存修改都会丢失。管理员密钥不会被重置。" : `这将恢复“${runtimeTypeLabels[resetRequest] || resetRequest}”的默认值，当前类型中的修改和未保存修改都会丢失。`}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={saving}>取消</AlertDialogCancel>
          <AlertDialogAction disabled={saving} onClick={(event) => { event.preventDefault(); void confirmReset(); }}>{saving ? "恢复中..." : "确认恢复"}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>;
}

function AdminKeyDialog({ open, onOpenChange, onDirtyChange }) {
  const [currentAccessKey, setCurrentAccessKey] = useState("");
  const [accessKey, setAccessKey] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [discardRequested, setDiscardRequested] = useState(false);
  const dirty = Boolean(currentAccessKey || accessKey || confirm);
  const valid = Boolean(currentAccessKey && accessKey.length >= 12 && accessKey === confirm);
  const mismatch = Boolean(confirm && accessKey !== confirm);

  useEffect(() => onDirtyChange?.(dirty ? 1 : 0), [dirty, onDirtyChange]);

  const clear = () => {
    setCurrentAccessKey("");
    setAccessKey("");
    setConfirm("");
  };

  const requestClose = () => {
    if (busy) return;
    if (dirty) {
      setDiscardRequested(true);
      return;
    }
    onOpenChange(false);
  };

  const submit = async (event) => {
    event.preventDefault();
    if (!valid) {
      toast.error(accessKey !== confirm ? "两次输入的新密钥不一致" : "新密钥至少需要 12 个字符");
      return;
    }
    setBusy(true);
    try {
      await api("/api/v1/auth/change-key", { method: "POST", body: JSON.stringify({ current_access_key: currentAccessKey, access_key: accessKey, confirm }) });
      clear();
      onOpenChange(false);
      toast.success("管理员访问密钥已修改，请重新登录");
      window.dispatchEvent(new CustomEvent("meerkit:unauthorized"));
    } catch (changeError) {
      toast.error(changeError.message);
    } finally {
      setBusy(false);
    }
  };

  return <>
    <Dialog open={open} onOpenChange={(nextOpen) => nextOpen ? onOpenChange(true) : requestClose()}>
      <DialogContent className="admin-key-dialog">
        <DialogHeader><div className="admin-key-dialog-heading"><KeyRound size={20} /><div><span className="eyebrow">SECURITY</span><DialogTitle>修改管理员密钥</DialogTitle><DialogDescription>修改后将撤销所有已登录会话，需要重新登录。</DialogDescription></div></div></DialogHeader>
        <form onSubmit={submit}>
          <input className="admin-key-username" type="text" name="username" autoComplete="username" value="admin" readOnly tabIndex={-1} aria-hidden="true" />
          <div className="modal-body admin-key-dialog-body"><label className="admin-key-field"><span>当前密钥</span><Input type="password" name="current-password" autoFocus autoComplete="current-password" required value={currentAccessKey} onChange={(event) => setCurrentAccessKey(event.target.value)} /></label><label className="admin-key-field"><span>新密钥</span><Input type="password" name="new-password" autoComplete="new-password" minLength={12} required value={accessKey} onChange={(event) => setAccessKey(event.target.value)} /></label><label className="admin-key-field"><span>确认新密钥</span><Input type="password" name="confirm-password" autoComplete="new-password" minLength={12} required value={confirm} onChange={(event) => setConfirm(event.target.value)} /></label>{mismatch && <p className="admin-key-form-error">两次输入的新密钥不一致</p>}</div>
          <DialogFooter><Button type="button" variant="ghost" onClick={requestClose} disabled={busy}>取消</Button><Button type="submit" disabled={busy || !dirty || !valid}>{busy ? "修改中..." : <><KeyRound size={15} />修改密钥</>}</Button></DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
    <AlertDialog open={discardRequested} onOpenChange={(nextOpen) => { if (!nextOpen) setDiscardRequested(false); }}>
      <AlertDialogContent><AlertDialogHeader><div className="alert-dialog-icon"><AlertTriangle size={18} /></div><AlertDialogTitle>放弃未保存的密钥修改？</AlertDialogTitle><AlertDialogDescription>当前填写的密钥不会被保存。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>继续编辑</AlertDialogCancel><AlertDialogAction onClick={() => { clear(); setDiscardRequested(false); onOpenChange(false); }}>放弃修改</AlertDialogAction></AlertDialogFooter></AlertDialogContent>
    </AlertDialog>
  </>;
}

function ConfigTable({ items }) {
  return <div className="startup-config-items">{items.map((item) => { const defaultValue = formatValue(item.default); const effectiveValue = formatValue(item.value); return <article className="startup-config-item" key={item.path}><div className="startup-config-item-header"><div className="startup-config-item-key"><code title={item.path}>{item.path}</code></div><p className="startup-config-item-description" title={item.description}>{item.description}</p></div><div className="startup-config-values"><div><span>默认值</span><code className="config-value config-default" title={defaultValue}>{defaultValue}</code></div><div><span>生效值</span><code className="config-value" title={effectiveValue}>{effectiveValue}</code></div></div><Badge variant="outline" className="startup-config-source">{sourceLabels[item.source] || item.source}</Badge></article>; })}</div>;
}

function RuntimeConfigTable({ items, onSaved, onDirtyChange, onRequestReset, resetSignal, resetting }) {
  const groups = useMemo(() => items.reduce((result, item) => { (result[item.type] ||= []).push(item); return result; }, {}), [items]);
  const [dirtyByType, setDirtyByType] = useState({});
  const reportDirty = React.useCallback((type, count) => setDirtyByType((current) => current[type] === count ? current : { ...current, [type]: count }), []);
  const unsavedCount = Object.values(dirtyByType).reduce((total, count) => total + count, 0);

  useEffect(() => onDirtyChange?.(unsavedCount), [onDirtyChange, unsavedCount]);

  return <div className="runtime-config-groups">{Object.entries(groups).map(([type, group]) => <RuntimeConfigGroup key={type} type={type} items={group} onSaved={onSaved} onDirtyChange={reportDirty} onRequestReset={onRequestReset} resetSignal={resetSignal} resetting={resetting} />)}</div>;
}

function RuntimeConfigGroup({ type, items, onSaved, onDirtyChange, onRequestReset, resetSignal, resetting }) {
  const [drafts, setDrafts] = useState(() => Object.fromEntries(items.map((item) => [item.path, item.value])));
  const [busyPath, setBusyPath] = useState("");
  const previousValues = useRef(new Map(items.map((item) => [item.path, item.value])));
  const previousResetSignal = useRef(resetSignal);

  useEffect(() => {
    const resetThisGroup = resetSignal && resetSignal !== previousResetSignal.current && (resetSignal.target === "all" || resetSignal.target === type);
    if (resetThisGroup) {
      setDrafts(Object.fromEntries(items.map((item) => [item.path, item.value])));
    } else {
      setDrafts((current) => {
        const next = { ...current };
        let changed = false;
        for (const item of items) {
          const hasDraft = Object.prototype.hasOwnProperty.call(current, item.path);
          const previousValue = previousValues.current.get(item.path);
          const draftWasDirty = hasDraft && previousValues.current.has(item.path) && !Object.is(normalizeValue(current[item.path], item.default), previousValue);
          if (!hasDraft || !draftWasDirty) {
            if (!Object.is(next[item.path], item.value)) changed = true;
            next[item.path] = item.value;
          }
        }
        return changed ? next : current;
      });
    }
    previousValues.current = new Map(items.map((item) => [item.path, item.value]));
    previousResetSignal.current = resetSignal;
  }, [items, resetSignal, type]);

  const dirtyPaths = useMemo(() => items.filter((item) => !Object.is(normalizeValue(drafts[item.path], item.default), item.value)).map((item) => item.path), [drafts, items]);
  const dirtyPathSet = useMemo(() => new Set(dirtyPaths), [dirtyPaths]);

  useEffect(() => onDirtyChange?.(type, dirtyPaths.length), [dirtyPaths.length, onDirtyChange, type]);

  const save = async (item) => {
    setBusyPath(item.path);
    try {
      await api(`/api/v1/system/config/runtime/${type}`, { method: "PATCH", body: JSON.stringify({ version: item.version, path: item.path, value: normalizeValue(drafts[item.path], item.default) }) });
      await onSaved();
      toast.success(`${item.path} 已保存`);
    } catch (saveError) {
      toast.error(saveError.message);
    } finally {
      setBusyPath("");
    }
  };

  return <section className="runtime-config-group"><div className="runtime-config-group-header"><div><div className="runtime-config-group-title"><h3>{runtimeTypeLabels[type] || type}</h3>{dirtyPaths.length > 0 && <span className="runtime-config-unsaved-label"><AlertTriangle size={12} />{dirtyPaths.length} 项未保存</span>}</div><p>配置类型：<code>{type}</code></p></div><IconButton variant="outline" title="恢复此类型默认值" aria-label="恢复此类型默认值" onClick={() => onRequestReset(type)} disabled={resetting}><RotateCcw size={14} /></IconButton></div><div className="runtime-config-items">{items.map((item) => { const dirty = dirtyPathSet.has(item.path); return <article key={item.path} className={`runtime-config-item${dirty ? " is-unsaved" : ""}`}><div className="runtime-config-item-header"><div className="runtime-config-path"><code title={item.path}>{item.path}</code>{dirty && <span>未保存</span>}{dirty && <AlertTriangle className="runtime-config-item-alert" size={15} aria-label="此配置尚未保存" />}</div><p className="runtime-config-item-description" title={item.description}>{item.description}</p></div><div className="runtime-config-item-fields"><div className="runtime-config-field"><span>当前值</span><RuntimeEditor item={item} value={drafts[item.path]} onChange={(value) => setDrafts((current) => ({ ...current, [item.path]: value }))} /></div><div className="runtime-config-field"><span>默认值</span><code className="config-value config-default">{formatValue(item.default)}</code></div></div><div className="runtime-config-item-footer"><IconButton variant="outline" title={dirty ? "保存未保存的配置" : "保存配置"} aria-label={dirty ? "保存未保存的配置" : "保存配置"} onClick={() => save(item)} disabled={!dirty || busyPath !== ""}><Save size={14} /></IconButton></div></article>; })}</div></section>;
}

function RuntimeEditor({ item, value, onChange }) {
  if (typeof item.default === "boolean") return <Switch checked={Boolean(value)} onCheckedChange={onChange} aria-label={item.path} />;
  const choices = choicesFor(item.path);
  if (choices) return <select className="runtime-config-editor field-control" value={value ?? ""} onChange={(event) => onChange(event.target.value)} aria-label={item.path}>{choices.map((choice) => <option key={choice.value} value={choice.value}>{choice.label}</option>)}</select>;
  return <Input className="runtime-config-editor" type={typeof item.default === "number" ? "number" : "text"} min={item.path.endsWith("max_concurrency") ? 1 : item.path.endsWith("poll_milliseconds") ? 100 : undefined} step={typeof item.default === "number" ? 1 : undefined} value={value ?? ""} onChange={(event) => onChange(event.target.value)} aria-label={item.path} />;
}

function choicesFor(path) {
  if (path.endsWith(".level") || path.endsWith(".log_level")) return [{ value: "debug", label: "debug" }, { value: "info", label: "info" }, { value: "warn", label: "warn" }, { value: "error", label: "error" }];
  if (path.endsWith(".format") || path.endsWith(".log_format")) return [{ value: "simple", label: "simple" }, { value: "text", label: "text" }, { value: "json", label: "json" }];
  return null;
}

function normalizeValue(value, sample) {
  if (typeof sample === "number") return Number(value);
  if (typeof sample === "boolean") return Boolean(value);
  return String(value ?? "");
}

function formatValue(value) {
  if (typeof value === "boolean") return value ? "true" : "false";
  if (value === null || value === undefined || value === "") return "-";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
