import React, { useCallback, useEffect, useState } from "react";
import { Activity, Edit3, Plus, RadioTower, Trash2 } from "lucide-react";
import { api } from "../lib/api";
import { formatDate } from "../lib/formatters";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { DeleteConfirmDialog } from "../components/ui/DeleteConfirmDialog";
import { EmptyState } from "../components/ui/EmptyState";
import { IconButton } from "../components/ui/IconButton";
import { StatusBoardDialog } from "../features/statusboard/StatusBoardDialog";

const levelLabels = { success: "正常", warning: "警告", failure: "失败", unknown: "未知" };

export function StatusBoardPage({ monitors, channels, refreshVersion = 0, onOpenExecution, notify }) {
  const [snapshot, setSnapshot] = useState({ groups: [] });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState(undefined);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async ({ quiet = false } = {}) => {
    if (!quiet) setLoading(true);
    setError("");
    try { setSnapshot(await api("/api/v1/status-board")); }
    catch (loadError) { setError(loadError.message); }
    finally { if (!quiet) setLoading(false); }
  }, []);

  useEffect(() => { void load(); }, [load, refreshVersion]);
	const reportError = useCallback((message) => notify?.(message, "error"), [notify]);
  useEffect(() => {
    let active = true;
    let socket;
    let retryTimer;
    let delay = 1000;
    const connect = () => {
      if (!active) return;
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      socket = new WebSocket(`${protocol}//${window.location.host}/api/v1/status-board/ws`);
      socket.onopen = () => { delay = 1000; void load({ quiet: true }); };
      socket.onmessage = (message) => {
        try {
          const event = JSON.parse(message.data);
          if (event.type === "record_created") {
            setSnapshot((current) => appendStreamSamples(current, event));
          } else if (event.type !== "connected") {
            void load({ quiet: true });
          }
        } catch { void load({ quiet: true }); }
      };
      socket.onerror = () => socket?.close();
      socket.onclose = () => {
        if (!active) return;
        retryTimer = window.setTimeout(connect, delay);
        delay = Math.min(delay * 2, 15000);
      };
    };
    connect();
    return () => { active = false; window.clearTimeout(retryTimer); socket?.close(); };
  }, [load]);

  const saved = () => {
    setEditing(undefined);
    notify?.("看板项已保存");
    void load({ quiet: true });
  };
  const confirmDelete = async (event) => {
    event.preventDefault();
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    try {
      await api(`/api/v1/status-board/items/${deleteTarget.id}`, { method: "DELETE" });
      setDeleteTarget(null);
      notify?.("看板项已删除");
      void load({ quiet: true });
    } catch (deleteError) { notify?.(deleteError.message, "error"); }
    finally { setDeleting(false); }
  };

  const itemCount = snapshot.groups.reduce((total, group) => total + group.items.length, 0);
  return <div className="page-stack status-board-page"><PageHeader eyebrow="STATUS BOARD" title="状态看板" description={`${snapshot.groups.length} 个监控分组 · ${itemCount} 个看板项`} actions={<Button onClick={() => setEditing(null)} disabled={!monitors.length}><Plus size={15} />添加看板项</Button>} />
    {loading ? <div className="records-empty">正在加载状态看板...</div> : error ? <div className="records-empty field-error">{error}</div> : !snapshot.groups.length ? <Card className="section-card"><EmptyState icon={RadioTower} title="暂无看板项" description="添加一个监控结果或条件状态。" action={<Button onClick={() => setEditing(null)} disabled={!monitors.length}><Plus size={15} />添加看板项</Button>} /></Card> : <div className="status-board-groups">{snapshot.groups.map((group) => <Card className="status-board-group" key={group.monitor.id}><header className="status-group-header"><div><span className="status-group-icon"><Activity size={16} /></span><span><strong>{group.monitor.name}</strong><small>{group.monitor.module_type} · {group.items.length} 个看板项</small></span></div><Badge tone={group.monitor.enabled ? "success" : "muted"}>{group.monitor.enabled ? "监控中" : "已停用"}</Badge></header><div className="status-item-list">{group.items.map((item) => <StatusItemRow key={item.id} item={item} monitor={group.monitor} onEdit={() => setEditing(item)} onDelete={() => setDeleteTarget(item)} onOpenExecution={onOpenExecution} />)}</div></Card>)}</div>}
    {editing !== undefined && <StatusBoardDialog item={editing || null} monitors={monitors} channels={channels} onClose={() => setEditing(undefined)} onSaved={saved} onError={reportError} />}
    <DeleteConfirmDialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && !deleting && setDeleteTarget(null)} title="删除看板项" description={deleteTarget ? `将删除“${deleteTarget.name}”的看板配置和趋势状态，现有执行记录不会被删除。` : ""} busy={deleting} onConfirm={confirmDelete} />
  </div>;
}

function StatusItemRow({ item, monitor, onEdit, onDelete, onOpenExecution }) {
  const current = item.current;
  const activeRules = Object.values(item.runtime_state?.rules || {}).filter((rule) => rule.active).length;
  return <article className="status-item-row"><div className="status-item-identity"><span className={`status-current-indicator ${sampleColorClass(current)}`} /><span><strong>{item.name}</strong><small>{item.source_label}</small></span></div><div className="status-item-current"><span>当前</span><strong title={current?.display || "未知"}>{current?.display || "未知"}</strong><Badge tone={toneForLevel(current?.level)}>{current?.label || levelLabels[current?.level] || "未知"}</Badge></div><div className="status-bars-viewport"><div className="status-bars" style={{ minWidth: `${Math.max(item.history_limit, 20) * 8}px` }}>{item.samples.length ? item.samples.map((sample) => <button type="button" className={`status-bar ${sampleColorClass(sample)}`} key={sample.record_id} style={{ height: `${sample.height}%` }} title={`${formatDate(sample.started_at)} · ${sample.display} · ${sample.label || levelLabels[sample.level]}`} aria-label={`${formatDate(sample.started_at)}，${sample.display}，${sample.label || levelLabels[sample.level]}`} onClick={() => onOpenExecution?.(monitor.id, sample.record_id)} />) : <span className="status-no-samples">暂无执行数据</span>}</div></div><div className="status-item-trend"><span>趋势</span>{!item.enabled ? <Badge tone="muted">已停用</Badge> : activeRules ? <Badge tone="warning">{activeRules} 条触发</Badge> : <Badge tone="success">正常</Badge>}</div><div className="status-item-actions"><IconButton size="sm" title="编辑看板项" aria-label="编辑看板项" onClick={onEdit}><Edit3 size={14} /></IconButton><IconButton size="sm" title="删除看板项" aria-label="删除看板项" onClick={onDelete}><Trash2 size={14} /></IconButton></div></article>;
}

function appendStreamSamples(snapshot, event) {
  const updates = new Map((event.items || []).map((item) => [item.item_id, item.sample]));
  return { ...snapshot, groups: snapshot.groups.map((group) => group.monitor.id !== event.monitor_id ? group : { ...group, items: group.items.map((item) => {
    const sample = updates.get(item.id);
    if (!sample) return item;
    const samples = normalizeSamples([...item.samples.filter((value) => value.record_id !== sample.record_id), sample].slice(-item.history_limit));
    return { ...item, samples, current: samples.at(-1) };
  }) }) };
}

function normalizeSamples(samples) {
  const numeric = samples.filter((sample) => typeof sample.numeric === "number").map((sample) => sample.numeric);
  if (!numeric.length) return samples;
  const min = Math.min(...numeric);
  const max = Math.max(...numeric);
  return samples.map((sample) => typeof sample.numeric !== "number" ? sample : { ...sample, height: min === max ? 100 : 10 + ((sample.numeric - min) / (max - min)) * 90 });
}

function toneForLevel(level) {
  return level === "success" ? "success" : level === "warning" ? "warning" : level === "failure" ? "danger" : "muted";
}

function sampleColorClass(sample) {
  return sample?.color ? `color-${sample.color}` : `level-${sample?.level || "unknown"}`;
}
