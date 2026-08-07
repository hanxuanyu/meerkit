import React, { useEffect, useMemo, useState } from "react";
import { Bell, Check, Plus, Trash2 } from "lucide-react";
import { api } from "../../lib/api";
import { Button } from "../../components/ui/Button";
import { Checkbox } from "../../components/ui/Checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { IconButton } from "../../components/ui/IconButton";
import { Input, Label } from "../../components/ui/Input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { Switch } from "../../components/ui/Switch";
import { NumericThresholdEditor } from "./NumericThresholdEditor";
import { ValueColorRulesEditor } from "./ValueColorRulesEditor";

const valueTypes = { boolean: "布尔", number: "数值", text: "文本" };
const ruleTypes = { consecutive: "连续异常", count: "窗口异常次数", average: "窗口平均值", delta: "首尾变化", slope: "线性斜率" };
const operators = { gt: ">", gte: "≥", lt: "<", lte: "≤" };

const defaultSource = { kind: "condition_overall", value_type: "boolean" };
const defaultThresholds = [{ maximum: null, level: "success", label: "正常", color: "green" }];

function valueStyle(valueType) {
  if (valueType === "text") return { value_mappings: [{ value: "", level: "failure", label: "失败", color: "red" }], default_level: "success", default_label: "正常", default_color: "green" };
  if (valueType === "number") return { value_mappings: [], default_level: "success", default_label: "正常", default_color: "green" };
  return {};
}

export function StatusBoardDialog({ item, monitors, channels, onClose, onSaved, onError }) {
  const [name, setName] = useState(item?.name || "");
  const [monitorID, setMonitorID] = useState(item?.monitor_id || monitors[0]?.id || "");
  const [enabled, setEnabled] = useState(item?.enabled ?? true);
  const [source, setSource] = useState(item?.source || defaultSource);
  const [invert, setInvert] = useState(item?.invert || false);
  const [thresholds, setThresholds] = useState(item?.thresholds?.length ? item.thresholds : defaultThresholds);
  const [historyLimit, setHistoryLimit] = useState(item?.history_limit || 60);
  const [trendRules, setTrendRules] = useState(item?.trend_rules || []);
  const [channelIDs, setChannelIDs] = useState(item?.notification_channel_ids || []);
  const [sources, setSources] = useState({ conditions: [], results: [] });
  const [loadingSources, setLoadingSources] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!monitorID) return;
    let cancelled = false;
    setLoadingSources(true);
    api(`/api/v1/status-board/sources?monitor_id=${encodeURIComponent(monitorID)}`)
      .then((response) => { if (!cancelled) setSources(response || { conditions: [], results: [] }); })
      .catch((error) => { if (!cancelled) onError(error.message); })
      .finally(() => { if (!cancelled) setLoadingSources(false); });
    return () => { cancelled = true; };
  }, [monitorID, onError]);

  const selectedResult = useMemo(() => sources.results.find((candidate) => candidate.result_set === source.result_set && candidate.field === source.field), [source.field, source.result_set, sources.results]);
  const effectiveType = source.value_type || primitiveType(selectedResult?.type);
  const sourceSelection = source.result_set && source.field ? `${source.result_set}::${source.field}` : "";

  const changeMonitor = (value) => {
    setMonitorID(value);
    setSource(defaultSource);
    setInvert(false);
    setThresholds(defaultThresholds);
    setTrendRules([]);
  };
  const changeSourceKind = (kind) => {
    if (kind === "condition_overall") setSource(defaultSource);
    else if (kind === "condition_rule") setSource({ kind, rule_id: "", value_type: "boolean" });
    else setSource({ kind, result_set: "", field: "", path: "", value_type: "" });
    setInvert(false);
    setTrendRules([]);
  };
  const selectResult = (selection) => {
    const [result_set, field] = selection.split("::");
    const descriptor = sources.results.find((candidate) => candidate.result_set === result_set && candidate.field === field);
    const value_type = primitiveType(descriptor?.type);
    setSource({ kind: "result_field", result_set, field, path: "", value_type, ...valueStyle(value_type) });
    setThresholds(value_type === "number" ? defaultThresholds : []);
    setInvert(false);
    setTrendRules([]);
  };
  const changeValueType = (value_type) => {
    setSource((current) => {
      const { value_mappings: _, default_level: __, default_label: ___, default_color: ____, ...base } = current;
      return { ...base, value_type, ...valueStyle(value_type) };
    });
    setThresholds(value_type === "number" ? defaultThresholds : []);
    setTrendRules((current) => value_type === "number" ? current : current.filter((rule) => rule.type === "consecutive" || rule.type === "count"));
  };
  const addTrendRule = () => setTrendRules((current) => [...current, { id: crypto.randomUUID(), name: `趋势规则 ${current.length + 1}`, type: "consecutive", window: 3, minimum: 2, operator: "gt", threshold: 0, delta_mode: "absolute" }]);
  const updateTrendRule = (index, patch) => setTrendRules((current) => current.map((rule, itemIndex) => itemIndex === index ? { ...rule, ...patch } : rule));
  const removeTrendRule = (index) => setTrendRules((current) => current.filter((_, itemIndex) => itemIndex !== index));

  const submit = async (event) => {
    event.preventDefault();
    setSaving(true);
    try {
      const payload = { name, monitor_id: monitorID, enabled, source, invert, thresholds: effectiveType === "number" ? thresholds : [], history_limit: Number(historyLimit), trend_rules: trendRules, notification_channel_ids: channelIDs };
      await api(item ? `/api/v1/status-board/items/${item.id}` : "/api/v1/status-board/items", { method: item ? "PATCH" : "POST", body: JSON.stringify(payload) });
      onSaved();
    } catch (error) {
      onError(error.message);
    } finally {
      setSaving(false);
    }
  };

  return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent className="modal-wide status-board-dialog"><DialogHeader><div><span className="eyebrow">STATUS BOARD ITEM</span><DialogTitle>{item ? "编辑看板项" : "添加看板项"}</DialogTitle><DialogDescription>选择一次执行中的状态或结果值，并按执行次数观察变化。</DialogDescription></div></DialogHeader><form onSubmit={submit}><div className="modal-body">
    <div className="form-grid"><label className="field"><Label>名称</Label><Input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：API 响应耗时" /></label><label className="field"><Label>监控项</Label><Select value={monitorID} onValueChange={changeMonitor} disabled={Boolean(item)}><SelectTrigger><SelectValue placeholder="选择监控项" /></SelectTrigger><SelectContent className="status-board-select-content">{monitors.map((monitor) => <SelectItem key={monitor.id} value={monitor.id}>{monitor.name}</SelectItem>)}</SelectContent></Select></label></div>
    <div className="form-section"><div className="form-section-title"><div><h3>显示来源</h3><p>条件按布尔状态展示，结果字段按声明或选定类型展示。</p></div><label className="status-dialog-enabled"><Switch checked={enabled} onCheckedChange={setEnabled} /><span>{enabled ? "已启用" : "已停用"}</span></label></div><div className="status-source-grid"><label className="field"><Label>来源类型</Label><Select value={source.kind} onValueChange={changeSourceKind}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent className="status-board-select-content"><SelectItem value="condition_overall">整体条件</SelectItem><SelectItem value="condition_rule">单条条件规则</SelectItem><SelectItem value="result_field">结果集字段</SelectItem></SelectContent></Select></label>
      {source.kind === "condition_rule" && <label className="field"><Label>条件规则</Label><Select value={source.rule_id || ""} onValueChange={(rule_id) => setSource((current) => ({ ...current, rule_id }))} disabled={loadingSources}><SelectTrigger><SelectValue placeholder="选择条件规则" /></SelectTrigger><SelectContent className="status-board-select-content">{sources.conditions.filter((condition) => condition.id).map((condition) => <SelectItem key={condition.id} value={condition.id}>{condition.label}</SelectItem>)}</SelectContent></Select></label>}
      {source.kind === "result_field" && <><label className="field"><Label>结果字段</Label><Select value={sourceSelection} onValueChange={selectResult} disabled={loadingSources}><SelectTrigger><SelectValue placeholder="选择结果字段" /></SelectTrigger><SelectContent className="status-board-select-content">{groupResults(sources.results).map((group) => <SelectGroup key={group.key}><SelectLabel className="status-board-select-label">{group.label}</SelectLabel>{group.items.map((result) => <SelectItem key={`${result.result_set}::${result.field}`} value={`${result.result_set}::${result.field}`}>{result.label}{result.unit ? ` (${result.unit})` : ""}</SelectItem>)}</SelectGroup>)}</SelectContent></Select></label>{selectedResult?.path && <label className="field"><Label>JSON 路径</Label><Input required value={source.path || ""} onChange={(event) => setSource((current) => ({ ...current, path: event.target.value }))} placeholder="例如 data.latency" /></label>}{(!primitiveType(selectedResult?.type) || selectedResult?.path) && <label className="field"><Label>显示类型</Label><Select value={source.value_type || "auto"} onValueChange={(value) => changeValueType(value === "auto" ? "" : value)}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent className="status-board-select-content"><SelectItem value="auto">自动识别</SelectItem>{Object.entries(valueTypes).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent></Select></label>}</>}
    </div>{effectiveType === "boolean" && <label className="status-invert-row"><Switch checked={invert} onCheckedChange={setInvert} /><span><strong>反转成功与失败</strong><small>启用后，条件满足或布尔值为 true 时显示为失败。</small></span></label>}</div>
    <div className="form-section"><div className="form-section-title"><div><h3>历史窗口</h3><p>按执行次数展示，最新执行位于最右侧。</p></div></div><label className="field status-history-field"><Label>展示次数</Label><Input type="number" min="20" max="200" value={historyLimit} onChange={(event) => setHistoryLimit(event.target.value)} /></label></div>
    {effectiveType === "number" && <div className="form-section"><div className="form-section-title"><div><h3>数值区间</h3><p>按递增上界匹配状态，最右侧分段同时覆盖超过可视上限的数值。</p></div></div><NumericThresholdEditor key={`${source.result_set || source.kind}:${source.field || ""}:${source.path || ""}`} thresholds={thresholds} onChange={setThresholds} /><ValueColorRulesEditor type="number" source={source} onChange={setSource} /></div>}
    {effectiveType === "text" && <div className="form-section"><div className="form-section-title"><div><h3>文本颜色</h3><p>为特定文本指定状态，其余内容使用默认状态。</p></div></div><ValueColorRulesEditor type="text" source={source} onChange={setSource} /></div>}
    <div className="form-section"><div className="form-section-title"><div><h3>趋势通知</h3><p>规则进入异常和恢复正常时各发送一次。</p></div><IconButton size="sm" title="添加趋势规则" aria-label="添加趋势规则" onClick={addTrendRule}><Plus size={14} /></IconButton></div><div className="trend-rule-list">{trendRules.length ? trendRules.map((rule, index) => <TrendRuleRow key={rule.id} rule={rule} index={index} numeric={effectiveType === "number"} onChange={(patch) => updateTrendRule(index, patch)} onRemove={() => removeTrendRule(index)} />) : <div className="condition-empty">暂无趋势规则</div>}</div></div>
    <div className="form-section"><div className="form-section-title"><div><h3>通知渠道</h3><p>仅用于该看板项的趋势触发和恢复。</p></div><Bell size={17} /></div><div className="channel-checks">{channels.map((channel) => <label key={channel.id} className="check-card"><Checkbox checked={channelIDs.includes(channel.id)} onCheckedChange={(checked) => setChannelIDs((current) => checked ? [...new Set([...current, channel.id])] : current.filter((id) => id !== channel.id))} /><span><strong>{channel.name}</strong><small>{channel.notifier_type.toUpperCase()}</small></span></label>)}</div></div>
  </div><DialogFooter><Button type="button" variant="ghost" onClick={onClose}>取消</Button><Button type="submit" disabled={saving || loadingSources || !monitorID}>{saving ? "保存中..." : <><Check size={15} />保存看板项</>}</Button></DialogFooter></form></DialogContent></Dialog>;
}

function TrendRuleRow({ rule, index, numeric, onChange, onRemove }) {
  const numericType = rule.type === "average" || rule.type === "delta" || rule.type === "slope";
  return <div className="trend-rule-row"><div className="trend-rule-heading"><strong>{index + 1}</strong><Input value={rule.name} onChange={(event) => onChange({ name: event.target.value })} aria-label="趋势规则名称" /><IconButton size="sm" title="删除趋势规则" aria-label="删除趋势规则" onClick={onRemove}><Trash2 size={14} /></IconButton></div><div className="trend-rule-fields"><label className="field"><Label>规则类型</Label><Select value={rule.type} onValueChange={(type) => onChange({ type })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent className="status-board-select-content">{Object.entries(ruleTypes).filter(([type]) => numeric || type === "consecutive" || type === "count").map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent></Select></label><label className="field"><Label>窗口次数</Label><Input type="number" min={numericType ? 2 : 1} max="200" value={rule.window} onChange={(event) => onChange({ window: Number(event.target.value) })} /></label>{rule.type === "count" && <label className="field"><Label>至少异常</Label><Input type="number" min="1" max={rule.window} value={rule.minimum} onChange={(event) => onChange({ minimum: Number(event.target.value) })} /></label>}{numericType && <><label className="field"><Label>比较</Label><Select value={rule.operator} onValueChange={(operator) => onChange({ operator })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent className="status-board-select-content">{Object.entries(operators).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent></Select></label><label className="field"><Label>阈值</Label><Input type="number" step="any" value={rule.threshold} onChange={(event) => onChange({ threshold: Number(event.target.value) })} /></label></>}{rule.type === "delta" && <label className="field"><Label>变化方式</Label><Select value={rule.delta_mode} onValueChange={(delta_mode) => onChange({ delta_mode })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent className="status-board-select-content"><SelectItem value="absolute">绝对变化</SelectItem><SelectItem value="percent">百分比变化</SelectItem></SelectContent></Select></label>}</div></div>;
}

function primitiveType(type) {
  if (type === "boolean") return "boolean";
  if (type === "number" || type === "integer") return "number";
  if (type === "string" || type === "text") return "text";
  return "";
}

function groupResults(results) {
  const groups = new Map();
  for (const result of results || []) {
    if (!groups.has(result.result_set)) groups.set(result.result_set, { key: result.result_set, label: result.result_label, items: [] });
    groups.get(result.result_set).items.push(result);
  }
  return [...groups.values()];
}
