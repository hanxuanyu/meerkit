import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Activity, ChevronDown, Copy, Edit3, Plus, RadioTower, Trash2 } from "lucide-react";
import { api } from "../lib/api";
import { duplicateStatusBoardDraft } from "../lib/duplicates";
import { formatDate } from "../lib/formatters";
import { PageHeader } from "../components/layout/PageHeader";
import { ActionMenu } from "../components/ui/ActionMenu";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { DeleteConfirmDialog } from "../components/ui/DeleteConfirmDialog";
import { EmptyState } from "../components/ui/EmptyState";
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
    const message = editing?.id ? "看板项已更新" : editing?.__duplicate ? "看板项副本已创建" : "看板项已创建";
    setEditing(undefined);
    notify?.(message);
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
    {loading ? <div className="records-empty">正在加载状态看板...</div> : error ? <div className="records-empty field-error">{error}</div> : !snapshot.groups.length ? <Card className="section-card"><EmptyState icon={RadioTower} title="暂无看板项" description="添加一个监控结果或条件状态。" action={<Button onClick={() => setEditing(null)} disabled={!monitors.length}><Plus size={15} />添加看板项</Button>} /></Card> : <div className="status-board-groups">{snapshot.groups.map((group) => <StatusBoardGroup key={group.monitor.id} group={group} onEdit={setEditing} onDuplicate={(item) => setEditing(duplicateStatusBoardDraft(item))} onDelete={setDeleteTarget} onOpenExecution={onOpenExecution} />)}</div>}
    {editing !== undefined && <StatusBoardDialog item={editing || null} monitors={monitors} channels={channels} onClose={() => setEditing(undefined)} onSaved={saved} onError={reportError} />}
    <DeleteConfirmDialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && !deleting && setDeleteTarget(null)} title="删除看板项" description={deleteTarget ? `将删除“${deleteTarget.name}”的看板配置和趋势状态，现有执行记录不会被删除。` : ""} busy={deleting} onConfirm={confirmDelete} />
  </div>;
}

function StatusBoardGroup({ group, onEdit, onDuplicate, onDelete, onOpenExecution }) {
  const [expanded, setExpanded] = useState(true);
  return <details className="status-board-group" open={expanded} onToggle={(event) => setExpanded(event.currentTarget.open)}>
    <summary className="status-group-header">
      <div><span className="status-group-icon"><Activity size={15} /></span><span><strong>{group.monitor.name}</strong><small>{group.monitor.module_type} · {group.items.length} 个看板项</small></span></div>
      <span className="status-group-controls"><Badge tone={group.monitor.enabled ? "success" : "muted"}>{group.monitor.enabled ? "监控中" : "已停用"}</Badge><ChevronDown className="status-group-chevron" size={16} /></span>
    </summary>
    <div className="status-item-list">{group.items.map((item) => <StatusItemRow key={item.id} item={item} monitor={group.monitor} onEdit={() => onEdit(item)} onDuplicate={() => onDuplicate(item)} onDelete={() => onDelete(item)} onOpenExecution={onOpenExecution} />)}</div>
  </details>;
}

function StatusItemRow({ item, monitor, onEdit, onDuplicate, onDelete, onOpenExecution }) {
  const current = item.current;
  const activeRules = Object.values(item.runtime_state?.rules || {}).filter((rule) => rule.active).length;
  return <article className="status-item-row">
    <div className="status-item-identity"><span className={`status-current-indicator ${sampleColorClass(current)}`} /><span><strong>{item.name}</strong><span className="status-item-subline"><small title={item.source_label}>{item.source_label}</small>{!item.enabled && <Badge tone="muted">已停用</Badge>}{activeRules > 0 && <Badge tone="warning">趋势告警 {activeRules}</Badge>}</span></span></div>
    <div className="status-item-current"><span>当前值</span><div><strong title={current?.display || "未知"}>{current?.display || "未知"}</strong><Badge tone={toneForLevel(current?.level)}>{current?.label || levelLabels[current?.level] || "未知"}</Badge></div></div>
    <StatusBars item={item} monitorID={monitor.id} onOpenExecution={onOpenExecution} />
    <div className="status-item-actions"><ActionMenu label={`${item.name} 操作`} items={[{ label: "编辑", icon: Edit3, onSelect: onEdit }, { label: "复制", icon: Copy, onSelect: onDuplicate }, { label: "删除", icon: Trash2, destructive: true, onSelect: onDelete }]} /></div>
  </article>;
}

function StatusBars({ item, monitorID, onOpenExecution }) {
  const viewportRef = useRef(null);
  const knownSampleIDsRef = useRef(null);
  const [activeSample, setActiveSample] = useState(null);
  const [viewportWidth, setViewportWidth] = useState(0);
  const activeSampleIndex = activeSample ? item.samples.findIndex((sample) => sample.record_id === activeSample.record_id) : -1;
  const layout = statusBarLayout(viewportWidth, item.history_limit, item.samples.length);

  if (knownSampleIDsRef.current === null) {
    knownSampleIDsRef.current = new Set(item.samples.map((sample) => sample.record_id));
  }
  const enteringSampleIDs = new Set(item.samples.filter((sample) => !knownSampleIDsRef.current.has(sample.record_id)).map((sample) => sample.record_id));
  const isShifting = layout.reachedEnd && enteringSampleIDs.size > 0;

  useEffect(() => {
    knownSampleIDsRef.current = new Set(item.samples.map((sample) => sample.record_id));
  }, [item.samples]);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) return undefined;
    const updateWidth = () => setViewportWidth((current) => current === viewport.clientWidth ? current : viewport.clientWidth);
    updateWidth();
    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", updateWidth);
      return () => window.removeEventListener("resize", updateWidth);
    }
    const observer = new ResizeObserver(updateWidth);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, []);

  const hideSummary = () => setActiveSample(null);
  return <div className="status-bars-panel" onPointerLeave={hideSummary}>
    {activeSample && <div className="status-bar-summary">
      <span className={`status-bar-summary-dot ${sampleColorClass(activeSample)}`} />
      <strong title={activeSample.display}>{activeSample.display || "未知"}</strong>
      <span>{activeSample.label || levelLabels[activeSample.level] || "未知"} · {formatDate(activeSample.started_at)}</span>
    </div>}
    <div className="status-bars-viewport" ref={viewportRef}>
      <div className={`status-bars${isShifting ? " is-shifting" : ""}`} style={{ "--status-bar-width": `${layout.barWidth}px`, "--status-bar-gap": `${layout.gap}px`, "--status-bars-shift": `${layout.shift}px`, "--status-bar-step": `${layout.barWidth + layout.gap}px` }}>
        {item.samples.length ? item.samples.map((sample, index) => <span className={`status-bar-slot${barProximityClass(index, activeSampleIndex)}${enteringSampleIDs.has(sample.record_id) ? " is-entering" : ""}`} key={sample.record_id} onPointerEnter={() => setActiveSample(sample)} onFocus={() => setActiveSample(sample)} onBlur={hideSummary}>
          <button type="button" className={`status-bar ${sampleColorClass(sample)}`} style={{ height: `${sample.height}%` }} title={`${formatDate(sample.started_at)} · ${sample.display} · ${sample.label || levelLabels[sample.level]}`} aria-label={`${formatDate(sample.started_at)}，${sample.display}，${sample.label || levelLabels[sample.level]}`} onClick={() => onOpenExecution?.(monitorID, sample.record_id)} />
        </span>) : <span className="status-no-samples">暂无执行数据</span>}
      </div>
    </div>
  </div>;
}

function barProximityClass(index, activeIndex) {
  if (activeIndex < 0) return "";
  const distance = Math.abs(index - activeIndex);
  if (distance === 0) return " is-active";
  return distance <= 3 ? ` is-near-${distance}` : "";
}

function statusBarLayout(viewportWidth, historyLimit, sampleCount) {
  if (!viewportWidth) return { barWidth: 5, gap: 3, shift: 0, reachedEnd: false };
  const limit = Math.max(Number(historyLimit) || 0, 20);
  const available = Math.max(0, viewportWidth - 4);
  const idealCell = available / limit;
  const barWidth = Math.min(7, Math.max(3, idealCell * .56));
  const gap = limit > 1 ? Math.max(2, (available - limit * barWidth) / (limit - 1)) : 2;
  const contentWidth = sampleCount > 0 ? sampleCount * barWidth + Math.max(0, sampleCount - 1) * gap + 4 : 0;
  const reachedEnd = contentWidth >= viewportWidth - .5;
  return { barWidth, gap, shift: Math.max(0, contentWidth - viewportWidth), reachedEnd };
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
