import React, { useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, Bell, Database, Download, FileArchive, FileCheck2, FileCog, KeyRound, LayoutDashboard, LockKeyhole, LockOpen, Plus, Radio, RefreshCw, RotateCcw, Save, Trash2, Upload } from "lucide-react";
import { toast } from "sonner";
import { api, apiBlob } from "../lib/api";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Input } from "../components/ui/Input";
import { IconButton } from "../components/ui/IconButton";
import { Switch } from "../components/ui/Switch";
import { Checkbox } from "../components/ui/Checkbox";
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
  const [exportDialogOpen, setExportDialogOpen] = useState(false);
  const [importDialogOpen, setImportDialogOpen] = useState(false);

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
          <div className="section-header"><div><h2>动态配置</h2><p>配置保存在 system_configs 表中，修改后无需重启服务。</p></div><div className="settings-config-actions"><Button variant="outline" size="sm" onClick={() => setExportDialogOpen(true)}><Upload size={14} />导出</Button><Button variant="outline" size="sm" onClick={() => setImportDialogOpen(true)} disabled={saving || unsavedCount > 0} title={unsavedCount > 0 ? "请先保存或放弃当前修改" : undefined}><Download size={14} />导入</Button><Button variant="outline" size="sm" onClick={() => setKeyDialogOpen(true)}><KeyRound size={14} />修改管理员密钥</Button><Button variant="outline" size="sm" onClick={() => setResetRequest("all")} disabled={saving}><RotateCcw size={14} />恢复全部默认</Button></div></div>
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
    <ConfigExportDialog open={exportDialogOpen} onOpenChange={setExportDialogOpen} />
    <ConfigImportDialog open={importDialogOpen} onOpenChange={setImportDialogOpen} onImported={reload} />
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

const transferOptions = [
  { key: "monitors", label: "监控项配置", description: "采集参数可能包含访问凭据", icon: Radio, sensitive: true },
  { key: "notification_channels", label: "通知渠道配置", description: "Webhook、SMTP 等渠道及凭据", icon: Bell, sensitive: true },
  { key: "status_board_items", label: "状态看板配置", description: "看板项、阈值和趋势规则", icon: LayoutDashboard }
];

function ConfigExportDialog({ open, onOpenChange }) {
  const [selected, setSelected] = useState({ monitors: true, notification_channels: true, status_board_items: true, admin_key: false });
  const [encryptionKey, setEncryptionKey] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [encrypted, setEncrypted] = useState(true);
  const [unencryptedConfirmed, setUnencryptedConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const keyValid = !encrypted || (encryptionKey.length >= 12 && encryptionKey === confirmation);

  const close = (force = false) => {
    if (busy && !force) return;
    setEncryptionKey("");
    setConfirmation("");
    setEncrypted(true);
    setUnencryptedConfirmed(false);
    onOpenChange(false);
  };

  const submit = async (event) => {
    event.preventDefault();
    if (!keyValid) { toast.error(encryptionKey !== confirmation ? "两次输入的配置包密码不一致" : "配置包密码至少需要 12 个字符"); return; }
    setBusy(true);
    try {
      const { blob, filename, summary } = await apiBlob("/api/v1/system/config/transfer/export", { method: "POST", body: JSON.stringify({ ...selected, encrypted, encryption_key: encryptionKey, encryption_key_confirm: confirmation }) });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      setBusy(false);
      close(true);
      toast.success("配置包已导出", { description: formatExportSummary(summary || { ...selected, encrypted, runtime_types: 5 }), duration: 9000 });
    } catch (exportError) { toast.error(exportError.message); } finally { setBusy(false); }
  };

  return <Dialog open={open} onOpenChange={(nextOpen) => nextOpen ? onOpenChange(true) : close()}>
    <DialogContent className="config-transfer-dialog">
      <DialogHeader><div className="config-transfer-heading"><Upload size={20} /><div><DialogTitle>导出配置</DialogTitle><DialogDescription>动态配置始终包含，可附加其他应用配置。</DialogDescription></div></div></DialogHeader>
      <form onSubmit={submit}>
        <div className="modal-body config-transfer-body">
          <div className={`config-export-encryption${encrypted ? " is-encrypted" : " is-plain"}`}><span className="config-transfer-option-icon">{encrypted ? <LockKeyhole size={16} /> : <LockOpen size={16} />}</span><span><strong>{encrypted ? "加密整个配置包" : "不加密配置包"}</strong><small>{encrypted ? "所有导出内容使用同一密码加密" : "所有导出内容将以明文保存"}</small></span><Switch checked={encrypted} onCheckedChange={(checked) => { setEncrypted(checked); setUnencryptedConfirmed(false); }} aria-label="加密整个配置包" /></div>
          <TransferOption checked disabled icon={Database} label="动态配置" description="导入时始终整体替换，不参与合并" />
          {transferOptions.map((option) => <TransferOption key={option.key} checked={selected[option.key]} icon={option.icon} label={option.label} description={option.description} sensitive={option.sensitive} onCheckedChange={(checked) => setSelected((current) => ({ ...current, [option.key]: Boolean(checked) }))} />)}
          <TransferOption checked={selected.admin_key} icon={KeyRound} label="管理员密钥" description="加密保存密钥哈希，导入后现有会话失效" sensitive onCheckedChange={(checked) => setSelected((current) => ({ ...current, admin_key: Boolean(checked) }))} />
          {encrypted && <div className="config-transfer-key-fields"><label><span>配置包密码</span><Input autoFocus type="password" minLength={12} autoComplete="new-password" value={encryptionKey} onChange={(event) => setEncryptionKey(event.target.value)} required /></label><label><span>确认配置包密码</span><Input type="password" minLength={12} autoComplete="new-password" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} required /></label>{confirmation && encryptionKey !== confirmation && <p>两次输入的配置包密码不一致</p>}</div>}
          {!encrypted && <div className="config-transfer-warning is-danger config-unencrypted-warning"><AlertTriangle size={17} /><div><strong>配置包将不加密</strong><span>动态配置、监控参数、通知凭据和管理员密钥（如选择）都可能以明文保存。请仅存放在可信位置。</span><label><Checkbox checked={unencryptedConfirmed} onCheckedChange={(checked) => setUnencryptedConfirmed(Boolean(checked))} /><em>我已了解未加密导出的风险</em></label></div></div>}
        </div>
        <DialogFooter><Button type="button" variant="ghost" onClick={() => close()} disabled={busy}>取消</Button><Button type="submit" variant={encrypted ? "default" : "destructive"} disabled={busy || !keyValid || (!encrypted && !unencryptedConfirmed)}>{busy ? "正在导出..." : <><Upload size={15} />导出压缩包</>}</Button></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}

function TransferOption({ checked, disabled, icon: Icon, label, description, sensitive, onCheckedChange }) {
  return <label className={`config-transfer-option${disabled ? " is-required" : ""}`}><Checkbox checked={checked} disabled={disabled} onCheckedChange={onCheckedChange} /><span className="config-transfer-option-icon"><Icon size={16} /></span><span><strong>{label}{sensitive && <small>敏感</small>}</strong><em>{description}</em></span></label>;
}

function ConfigImportDialog({ open, onOpenChange, onImported }) {
  const inputRef = useRef(null);
  const [file, setFile] = useState(null);
  const [mode, setMode] = useState("merge");
  const [encryptionKey, setEncryptionKey] = useState("");
  const [dragging, setDragging] = useState(false);
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState(null);

  const chooseFile = (value) => {
    if (!value) return;
    if (!value.name.toLowerCase().endsWith(".zip")) { toast.error("请选择 ZIP 配置文件"); return; }
    if (value.size > 10 * 1024 * 1024) { toast.error("配置文件不能超过 10 MB"); return; }
    setFile(value); setPreview(null);
  };
  const close = (force = false) => {
    if (busy && !force) return;
    setFile(null); setMode("merge"); setEncryptionKey(""); setDragging(false); setPreview(null);
    if (inputRef.current) inputRef.current.value = "";
    onOpenChange(false);
  };
  const createFormData = () => {
    const body = new FormData();
    body.append("file", file);
    body.append("mode", mode);
    body.append("encryption_key", encryptionKey);
    return body;
  };
  const submit = async (event) => {
    event.preventDefault();
    if (!file) { toast.error("请先选择配置包"); return; }
    setBusy(true);
    try {
      if (!preview) {
        const result = await api("/api/v1/system/config/transfer/import/preview", { method: "POST", body: createFormData() });
        setPreview(result);
        return;
      }
      const result = await api("/api/v1/system/config/transfer/import", { method: "POST", body: createFormData() });
      if (!result.admin_key_imported) await onImported();
      setBusy(false);
      close(true);
      toast.success(mode === "replace" ? "配置已完全覆盖" : "配置已合并导入", { description: formatImportSummary(result.summary, result.admin_key_imported), duration: 12000 });
      if (result.admin_key_imported) window.setTimeout(() => window.dispatchEvent(new CustomEvent("meerkit:unauthorized")), 4000);
    } catch (importError) { toast.error(importError.message); } finally { setBusy(false); }
  };

  return <Dialog open={open} onOpenChange={(nextOpen) => nextOpen ? onOpenChange(true) : close()}>
    <DialogContent className="config-transfer-dialog config-import-dialog">
      <DialogHeader><div className="config-transfer-heading">{preview ? <FileCheck2 size={20} /> : <Download size={20} />}<div><DialogTitle>{preview ? "确认导入变更" : "导入配置"}</DialogTitle><DialogDescription>{preview ? "核对即将新增、覆盖和删除的配置项。" : "上传由 Meerkit 导出的 ZIP 配置包。"}</DialogDescription></div></div></DialogHeader>
      <form onSubmit={submit}>
        <div className="modal-body config-transfer-body">
          <input ref={inputRef} className="sr-only" type="file" accept=".zip,application/zip" onChange={(event) => chooseFile(event.target.files?.[0])} />
          <button className={`config-import-dropzone${dragging ? " is-dragging" : ""}${preview ? " is-locked" : ""}`} type="button" disabled={Boolean(preview)} onClick={() => inputRef.current?.click()} onDragEnter={(event) => { event.preventDefault(); if (!preview) setDragging(true); }} onDragOver={(event) => event.preventDefault()} onDragLeave={(event) => { event.preventDefault(); setDragging(false); }} onDrop={(event) => { event.preventDefault(); setDragging(false); if (!preview) chooseFile(event.dataTransfer.files?.[0]); }}>
            <FileArchive size={24} /><span>{file ? <><strong>{file.name}</strong><small>{formatFileSize(file.size)}</small></> : <><strong>拖拽配置包到此处</strong><small>或点击选择 ZIP 文件，最大 10 MB</small></>}</span>
          </button>
          <fieldset className="config-import-mode" disabled={Boolean(preview)}><legend>导入方式</legend><label className={mode === "merge" ? "is-selected" : ""}><input type="radio" name="import-mode" value="merge" checked={mode === "merge"} onChange={() => { setMode("merge"); setPreview(null); }} /><span><strong>合并</strong><small>不同 ID 新增，相同 ID 覆盖</small></span></label><label className={mode === "replace" ? "is-selected" : ""}><input type="radio" name="import-mode" value="replace" checked={mode === "replace"} onChange={() => { setMode("replace"); setPreview(null); }} /><span><strong>覆盖</strong><small>清空业务配置后完全替换</small></span></label></fieldset>
          {mode === "replace" && <div className="config-transfer-warning is-danger"><AlertTriangle size={16} /><span>覆盖会清空现有监控项、状态看板和非内置通知渠道。执行记录、站内通知不会从配置包导入。</span></div>}
          <label className="config-import-key"><span>配置包密码 <small>导入加密包时必填</small></span><Input type="password" autoComplete="current-password" value={encryptionKey} disabled={Boolean(preview)} onChange={(event) => { setEncryptionKey(event.target.value); setPreview(null); }} placeholder="未加密配置包可留空" /></label>
          {preview && <ConfigImportPreview preview={preview} />}
        </div>
        <DialogFooter>{preview && <Button type="button" variant="ghost" onClick={() => setPreview(null)} disabled={busy}>修改选项</Button>}<Button type="button" variant="ghost" onClick={() => close()} disabled={busy}>取消</Button><Button type="submit" disabled={busy || !file} variant={preview && mode === "replace" ? "destructive" : "default"}>{busy ? (preview ? "正在导入..." : "正在分析...") : preview ? <><Download size={15} />确认导入</> : <><FileCheck2 size={15} />预览变更</>}</Button></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}

const importSummarySections = [
  { key: "runtime", label: "动态配置", icon: Database },
  { key: "monitors", label: "监控项", icon: Radio },
  { key: "notification_channels", label: "通知渠道", icon: Bell },
  { key: "status_board_items", label: "状态看板", icon: LayoutDashboard },
  { key: "admin_key", label: "管理员密钥", icon: KeyRound }
];

const importChangeKinds = [
  { key: "added", label: "新增", icon: Plus },
  { key: "overwritten", label: "覆盖", icon: RefreshCw },
  { key: "deleted", label: "删除", icon: Trash2 }
];

function ConfigImportPreview({ preview }) {
  const sections = importSummarySections.map((section) => ({ ...section, changes: preview.summary?.[section.key] || {} })).filter((section) => countChanges(section.changes) > 0);
  const totals = sumChanges(preview.summary);
  const exportedAt = preview.exported_at ? new Date(preview.exported_at).toLocaleString("zh-CN", { hour12: false }) : "未知";
  return <section className="config-import-preview" aria-label="导入变更预览">
    <div className="config-import-preview-heading"><div><strong>变更预览</strong><span>{preview.encrypted ? "加密配置包" : "未加密配置包"} · 导出于 {exportedAt}</span></div><div className="config-import-preview-totals"><span className="is-added">新增 {totals.added}</span><span className="is-overwritten">覆盖 {totals.overwritten}</span><span className="is-deleted">删除 {totals.deleted}</span></div></div>
    <div className="config-import-preview-sections">{sections.map(({ key, label, icon: Icon, changes }) => <div className="config-import-preview-section" key={key}><div className="config-import-preview-section-title"><Icon size={14} /><strong>{label}</strong><span>{countChanges(changes)} 项变更</span></div>{importChangeKinds.map((kind) => <ConfigChangeGroup key={kind.key} kind={kind} items={changes[kind.key] || []} />)}</div>)}</div>
  </section>;
}

function ConfigChangeGroup({ kind, items }) {
  if (!items.length) return null;
  const Icon = kind.icon;
  return <details className={`config-import-change-group is-${kind.key}`} open={items.length <= 6}><summary><Icon size={12} /><span>{kind.label}</span><strong>{items.length}</strong></summary><ul>{items.map((item) => <li key={item.id}><span>{item.name || item.id}</span><code>{item.id}</code></li>)}</ul></details>;
}

function countChanges(changes = {}) {
  return importChangeKinds.reduce((total, kind) => total + (changes[kind.key]?.length || 0), 0);
}

function sumChanges(summary = {}) {
  return importSummarySections.reduce((totals, section) => {
    for (const kind of importChangeKinds) totals[kind.key] += summary?.[section.key]?.[kind.key]?.length || 0;
    return totals;
  }, { added: 0, overwritten: 0, deleted: 0 });
}

function formatImportSummary(summary, adminKeyImported) {
  const sections = importSummarySections.map((section) => {
    const changes = summary?.[section.key] || {};
    const parts = importChangeKinds.map((kind) => [kind.label, changes[kind.key]?.length || 0]).filter(([, count]) => count > 0).map(([label, count]) => `${label} ${count}`);
    return parts.length ? `${section.label}：${parts.join("、")}` : "";
  }).filter(Boolean);
  if (adminKeyImported) sections.push("管理员密钥已替换，即将返回登录页");
  return sections.join("；") || "配置内容无业务项变更";
}

function formatExportSummary(summary = {}) {
  const parts = [`动态配置 ${summary.runtime_types || 5} 类`];
  parts.push(formatExportCategory("监控项", summary.monitors, summary.monitors_included ?? summary.monitors));
  parts.push(formatExportCategory("通知渠道", summary.notification_channels, summary.notification_channels_included ?? summary.notification_channels));
  parts.push(formatExportCategory("状态看板", summary.status_board_items, summary.status_board_items_included ?? summary.status_board_items));
  if (summary.admin_key) parts.push("管理员密钥已包含");
  else parts.push("管理员密钥未包含");
  parts.push(summary.encrypted ? "整包已加密" : "配置包未加密");
  return parts.join(" · ");
}

function formatExportCategory(label, count, included) {
  if (!included) return `${label}未包含`;
  return typeof count === "number" ? `${label} ${count}` : `${label}已包含`;
}

function formatFileSize(size) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
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
