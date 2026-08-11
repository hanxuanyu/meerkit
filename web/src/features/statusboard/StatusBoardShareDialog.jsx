import React, { useEffect, useMemo, useState } from "react";
import { Copy, ExternalLink, EyeOff, Link2, LoaderCircle, Plus, RotateCcw, Share2, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { api } from "../../lib/api";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "../../components/ui/AlertDialog";
import { Button } from "../../components/ui/Button";
import { Checkbox } from "../../components/ui/Checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { IconButton } from "../../components/ui/IconButton";
import { Input, Label } from "../../components/ui/Input";

export function StatusBoardShareDialog({ open, snapshot, onOpenChange, notify }) {
  const [name, setName] = useState("状态看板分享");
  const [monitorIDs, setMonitorIDs] = useState([]);
  const [itemIDs, setItemIDs] = useState([]);
  const [shares, setShares] = useState([]);
  const [createdURL, setCreatedURL] = useState("");
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [changeTarget, setChangeTarget] = useState(null);

  const selectedMonitorIDs = useMemo(() => new Set(monitorIDs), [monitorIDs]);
  const selectedItemIDs = useMemo(() => new Set(itemIDs), [itemIDs]);
  const selectionCount = monitorIDs.length + itemIDs.length;

  useEffect(() => {
    if (!open) return;
    let active = true;
    setLoading(true);
    api("/api/v1/status-board/shares")
      .then((result) => { if (active) setShares(result || []); })
      .catch((error) => { if (active) notify?.(error.message, "error"); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [notify, open]);

  const toggleGroup = (group, checked) => {
    setMonitorIDs((current) => checked ? [...new Set([...current, group.monitor.id])] : current.filter((id) => id !== group.monitor.id));
    if (checked) setItemIDs((current) => current.filter((id) => !group.items.some((item) => item.id === id)));
  };
  const toggleItem = (id, checked) => setItemIDs((current) => checked ? [...new Set([...current, id])] : current.filter((value) => value !== id));

  const createShare = async (event) => {
    event.preventDefault();
    if (!name.trim()) { toast.error("请输入分享名称"); return; }
    if (!selectionCount) { toast.error("请至少选择一个分组或看板项"); return; }
    setBusy(true);
    try {
      const result = await api("/api/v1/status-board/shares", { method: "POST", body: JSON.stringify({ name: name.trim(), monitor_ids: monitorIDs, item_ids: itemIDs }) });
      const url = absoluteShareURL(result.url);
      setCreatedURL(url);
      setShares((current) => [result, ...current]);
      setMonitorIDs([]);
      setItemIDs([]);
      setName("状态看板分享");
      try { await navigator.clipboard.writeText(url); toast.success("共享链接已创建并复制"); }
      catch { toast.success("共享链接已创建"); }
    } catch (error) { notify?.(error.message, "error"); }
    finally { setBusy(false); }
  };

  const copyCreatedURL = async () => {
    try { await navigator.clipboard.writeText(createdURL); toast.success("共享链接已复制"); }
    catch { toast.error("无法写入剪贴板"); }
  };

  const copyManagedURL = async (url) => {
    try { await navigator.clipboard.writeText(absoluteShareURL(url)); toast.success("共享链接已复制"); }
    catch { toast.error("无法写入剪贴板"); }
  };

  const applyShareChange = async () => {
    if (!changeTarget) return;
    const { share, action } = changeTarget;
    setBusy(true);
    try {
      const path = action === "restore" ? `/api/v1/status-board/shares/${share.id}/restore` : action === "delete" ? `/api/v1/status-board/shares/${share.id}/permanent` : `/api/v1/status-board/shares/${share.id}`;
      await api(path, { method: action === "restore" ? "POST" : "DELETE" });
      if (action === "delete") setShares((current) => current.filter((item) => item.id !== share.id));
      else setShares((current) => current.map((item) => item.id === share.id ? { ...item, active: action === "restore" } : item));
      setChangeTarget(null);
      toast.success(action === "restore" ? "共享链接已恢复" : action === "delete" ? "共享链接已永久删除" : "共享链接已停用");
    } catch (error) { notify?.(error.message, "error"); }
    finally { setBusy(false); }
  };

  const confirmation = shareChangeConfirmation(changeTarget);

  return <><Dialog open={open} onOpenChange={(nextOpen) => !busy && onOpenChange(nextOpen)}>
    <DialogContent className="status-share-dialog">
      <DialogHeader><div className="status-share-heading"><Share2 size={20} /><div><DialogTitle>分享状态看板</DialogTitle><DialogDescription>创建只读公共页面，并管理已经生成的共享链接。</DialogDescription></div></div></DialogHeader>
      <form onSubmit={createShare}>
        <div className="modal-body status-share-body">
          {createdURL && <div className="status-share-created"><div><Link2 size={15} /><span><strong>共享链接已生成</strong><small>可在下方管理列表中再次复制。</small></span></div><div><Input readOnly value={createdURL} aria-label="新创建的共享链接" /><IconButton type="button" size="default" title="复制共享链接" aria-label="复制共享链接" onClick={() => void copyCreatedURL()}><Copy size={14} /></IconButton><IconButton type="button" size="default" title="打开共享页面" aria-label="打开共享页面" onClick={() => window.open(createdURL, "_blank", "noopener,noreferrer")}><ExternalLink size={14} /></IconButton></div></div>}
          <section className="status-share-create"><div className="status-share-section-title"><div><strong>新建分享</strong><span>选择整个分组可自动包含该分组后续新增的看板项。</span></div><span>{selectionCount} 项选择</span></div><label className="field"><Label>分享名称</Label><Input value={name} maxLength={80} onChange={(event) => setName(event.target.value)} placeholder="例如：公开服务状态" /></label><div className="status-share-selection">{(snapshot.groups || []).map((group) => { const groupSelected = selectedMonitorIDs.has(group.monitor.id); return <div className="status-share-group" key={group.monitor.id}><label><Checkbox checked={groupSelected} onCheckedChange={(checked) => toggleGroup(group, Boolean(checked))} /><span><strong>{group.monitor.name}</strong><small>整个分组 · {group.items.length} 个看板项</small></span></label><div>{group.items.map((item) => <label key={item.id}><Checkbox checked={groupSelected || selectedItemIDs.has(item.id)} disabled={groupSelected} onCheckedChange={(checked) => toggleItem(item.id, Boolean(checked))} /><span>{item.name}</span></label>)}</div></div>; })}</div></section>
          <section className="status-share-management"><div className="status-share-section-title"><div><strong>共享链接管理</strong><span>停用后公共页面立即失效，可恢复或永久删除。</span></div><span>{shares.filter((share) => share.active).length} 个有效</span></div>{loading ? <div className="status-share-empty"><LoaderCircle className="spin" size={16} />正在加载...</div> : shares.length ? <div className="status-share-list">{shares.map((share) => <div className={`status-share-row${share.active ? "" : " is-inactive"}`} key={share.id}><span><strong>{share.name}<em>{share.active ? "有效" : "已停用"}</em></strong><small>{share.monitor_ids?.length || 0} 个分组 · {share.item_ids?.length || 0} 个单项 · {formatShareDate(share.created_at)}</small></span><div className="status-share-row-actions">{share.active ? <><IconButton type="button" size="sm" variant="ghost" title="复制共享链接" aria-label={`复制共享链接 ${share.name}`} disabled={busy} onClick={() => void copyManagedURL(share.url)}><Copy size={14} /></IconButton><IconButton type="button" size="sm" variant="ghost" title="打开共享页面" aria-label={`打开共享页面 ${share.name}`} disabled={busy} onClick={() => window.open(absoluteShareURL(share.url), "_blank", "noopener,noreferrer")}><ExternalLink size={14} /></IconButton><IconButton type="button" size="sm" variant="ghost" title="停用共享链接" aria-label={`停用共享链接 ${share.name}`} disabled={busy} onClick={() => setChangeTarget({ share, action: "disable" })}><EyeOff size={14} /></IconButton></> : <><IconButton type="button" size="sm" variant="ghost" title="恢复共享链接" aria-label={`恢复共享链接 ${share.name}`} disabled={busy} onClick={() => setChangeTarget({ share, action: "restore" })}><RotateCcw size={14} /></IconButton><IconButton className="is-destructive" type="button" size="sm" variant="ghost" title="永久删除共享链接" aria-label={`永久删除共享链接 ${share.name}`} disabled={busy} onClick={() => setChangeTarget({ share, action: "delete" })}><Trash2 size={14} /></IconButton></>}</div></div>)}</div> : <div className="status-share-empty">尚未创建共享链接</div>}</section>
        </div>
        <DialogFooter><Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>关闭</Button><Button type="submit" disabled={busy || !selectionCount || !name.trim()}>{busy ? <><LoaderCircle className="spin" size={15} />处理中...</> : <><Plus size={15} />创建共享链接</>}</Button></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
  <AlertDialog open={Boolean(changeTarget)} onOpenChange={(nextOpen) => !nextOpen && !busy && setChangeTarget(null)}><AlertDialogContent><AlertDialogHeader><div className="alert-dialog-icon">{confirmation.icon}</div><AlertDialogTitle>{confirmation.title}</AlertDialogTitle><AlertDialogDescription>{confirmation.description}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={busy}>取消</AlertDialogCancel><AlertDialogAction variant={confirmation.variant} disabled={busy} onClick={(event) => { event.preventDefault(); void applyShareChange(); }}>{busy ? "处理中..." : confirmation.action}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog></>;
}

function shareChangeConfirmation(target) {
  if (!target) return { icon: null, title: "", description: "", action: "", variant: "default" };
  if (target.action === "restore") return { icon: <RotateCcw size={18} />, title: "恢复共享链接？", description: `“${target.share.name}”原有的公共链接将重新生效。`, action: "确认恢复", variant: "default" };
  if (target.action === "delete") return { icon: <Trash2 size={18} />, title: "永久删除共享链接？", description: `“${target.share.name}”的令牌记录将被删除，原链接无法再次恢复。`, action: "永久删除", variant: "destructive" };
  return { icon: <EyeOff size={18} />, title: "停用共享链接？", description: `“${target.share.name}”对应的公共页面将立即失效，之后仍可从管理列表恢复。`, action: "确认停用", variant: "destructive" };
}

function formatShareDate(value) {
  if (!value) return "创建时间未知";
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}

function absoluteShareURL(value) {
  return new URL(value, window.location.origin).href;
}
