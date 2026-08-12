import React, { useEffect, useMemo, useRef, useState } from "react";
import { Braces, ClipboardPaste, Copy, Plus, Trash2 } from "lucide-react";
import { getParameterOptions, isParameterEnabled, isParameterVisible } from "../../lib/parameterSchema";
import { IconButton } from "../ui/IconButton";
import { Input, Label } from "../ui/Input";
import { PasswordInput } from "../ui/PasswordInput";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../ui/Select";
import { Switch } from "../ui/Switch";

function FieldHint({ parameter }) {
  return parameter.description ? <small title={parameter.description}>{parameter.description}</small> : null;
}

function FieldLabel({ parameter }) {
  return <Label>{parameter.label}{parameter.required && <span className="field-required">*</span>}</Label>;
}

function FieldHeading({ parameter }) {
  return <span className="parameter-field-heading"><FieldLabel parameter={parameter} /><FieldHint parameter={parameter} /></span>;
}

function MapField({ value, onChange, disabled, parameter }) {
  const idRef = useRef(0);
  const toEntries = (source) => Object.entries(source && typeof source === "object" ? source : {}).map(([key, item]) => ({ id: idRef.current++, key, value: String(item ?? "") }));
  const [entries, setEntries] = useState(() => toEntries(value));
  const serializedValue = JSON.stringify(value && typeof value === "object" ? value : {});

  useEffect(() => {
    const next = toEntries(value);
    const current = Object.fromEntries(entries.filter((item) => item.key.trim()).map((item) => [item.key, item.value]));
    if (JSON.stringify(current) !== serializedValue) setEntries(next);
  }, [serializedValue]);

  const emit = (next) => {
    setEntries(next);
    onChange(Object.fromEntries(next.filter((item) => item.key.trim()).map((item) => [item.key.trim(), item.value])));
  };
  const update = (index, patch) => emit(entries.map((item, itemIndex) => itemIndex === index ? { ...item, ...patch } : item));

  const addEntry = () => setEntries((current) => [...current, { id: idRef.current++, key: "", value: "" }]);
  return <div className="map-editor"><div className="map-editor-title"><FieldHeading parameter={parameter} /><IconButton disabled={disabled} className="map-editor-add" size="sm" title="添加一行" aria-label="添加一行" onClick={addEntry}><Plus size={14} /></IconButton></div>{entries.length ? <div className="map-editor-table"><div className="map-editor-head"><span>键</span><span>值</span><span /></div>{entries.map((entry, index) => <div className="map-editor-row" key={entry.id}><Input disabled={disabled} aria-label={`第 ${index + 1} 行键`} value={entry.key} onChange={(event) => update(index, { key: event.target.value })} placeholder="名称" /><Input disabled={disabled} aria-label={`第 ${index + 1} 行值`} value={entry.value} onChange={(event) => update(index, { value: event.target.value })} placeholder="值" /><IconButton disabled={disabled} className="map-editor-remove" size="sm" title="删除此行" aria-label="删除此行" onClick={() => emit(entries.filter((_, itemIndex) => itemIndex !== index))}><Trash2 size={14} /></IconButton></div>)}</div> : <div className="map-editor-empty">点击右上角 + 添加项目</div>}</div>;
}

function JsonField({ value, onChange, parameter, asObject, disabled }) {
  const initial = typeof value === "string" ? value : JSON.stringify(value ?? {}, null, 2);
  const [draft, setDraft] = useState(initial);
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    const next = typeof value === "string" ? value : JSON.stringify(value ?? {}, null, 2);
    if (!error) setDraft(next);
  }, [value]);

  const update = (next) => {
    setDraft(next);
    try {
      const parsed = JSON.parse(next);
      onChange(asObject ? parsed : next);
      setError("");
    } catch {
      if (!asObject) onChange(next);
      setError("JSON 格式尚未完成");
    }
  };
  const format = () => {
    try {
      const formatted = JSON.stringify(JSON.parse(draft), null, 2);
      setDraft(formatted);
      onChange(asObject ? JSON.parse(formatted) : formatted);
      setError("");
    } catch {
      setError("JSON 格式错误，暂时无法格式化");
    }
  };
  const paste = async () => {
    try { update(await navigator.clipboard.readText()); } catch { setError("无法读取剪贴板内容"); }
  };
  const copy = async () => {
    try { await navigator.clipboard.writeText(draft); setCopied(true); window.setTimeout(() => setCopied(false), 1200); } catch { setError("无法写入剪贴板"); }
  };

  return <><div className="json-editor"><textarea className="field-textarea" rows={parameter.rows || 7} value={draft} onChange={(event) => update(event.target.value)} disabled={disabled} placeholder={parameter.placeholder} /><div className="json-editor-toolbar"><IconButton disabled={disabled} size="sm" title="格式化 JSON" aria-label="格式化 JSON" onClick={format}><Braces size={14} /></IconButton><IconButton disabled={disabled} size="sm" title="从剪贴板粘贴" aria-label="从剪贴板粘贴" onClick={paste}><ClipboardPaste size={14} /></IconButton><IconButton disabled={disabled} size="sm" title={copied ? "已复制" : "复制内容"} aria-label={copied ? "已复制" : "复制内容"} onClick={copy}><Copy size={14} /></IconButton></div></div>{error && <small className="field-error">{error}</small>}</>;
}

function TimeField({ value, onChange, parameter, disabled }) {
  const [hour = "", minute = ""] = String(value || "").split(":");
  const minuteStep = Math.max(1, Math.round(parameter.step || 1));
  const minutes = useMemo(() => Array.from({ length: Math.ceil(60 / minuteStep) }, (_, index) => index * minuteStep).filter((item) => item < 60), [minuteStep]);
  const update = (nextHour, nextMinute) => onChange(nextHour && nextMinute ? `${nextHour}:${nextMinute}` : "");
  return <div className="time-field"><Select value={hour || undefined} onValueChange={(next) => update(next, minute)} disabled={disabled}><SelectTrigger><SelectValue placeholder="小时" /></SelectTrigger><SelectContent>{Array.from({ length: 24 }, (_, index) => String(index).padStart(2, "0")).map((item) => <SelectItem key={item} value={item}>{item} 时</SelectItem>)}</SelectContent></Select><span>:</span><Select value={minute || undefined} onValueChange={(next) => update(hour, next)} disabled={disabled}><SelectTrigger><SelectValue placeholder="分钟" /></SelectTrigger><SelectContent>{minutes.map((item) => { const option = String(item).padStart(2, "0"); return <SelectItem key={option} value={option}>{option} 分</SelectItem>; })}</SelectContent></Select></div>;
}

const emptyBrowserTarget = "__meerkit_browser_target_none__";

function BrowserTargetField({ parameter, value, onChange, disabled, browserTargets }) {
  const isWindow = parameter.type === "browser_window";
  const windows = browserTargets?.windows || [];
  const tabs = browserTargets?.tabs || [];
  const options = isWindow
    ? windows.map((window) => ({ id: window.id, label: `窗口 ${window.id}${window.focused ? " · 当前" : ""}` }))
    : tabs.map((tab) => ({ id: tab.id, label: `#${tab.id} · 窗口 ${tab.window_id} · ${tab.title || tab.url || "未命名"}` }));
  const selected = value === "" || value == null ? (parameter.required ? undefined : emptyBrowserTarget) : String(value);
  const unavailable = !browserTargets;
  if (unavailable) return <label className="field"><FieldHeading parameter={parameter} /><Input disabled={disabled} required={parameter.required} type="number" min="1" value={value ?? ""} onChange={(event) => onChange(event.target.value === "" ? "" : Number(event.target.value))} placeholder={parameter.placeholder} /></label>;
  return <label className="field"><FieldHeading parameter={parameter} /><Select value={selected} onValueChange={(next) => onChange(next === emptyBrowserTarget ? "" : Number(next))} disabled={disabled}><SelectTrigger><SelectValue placeholder={isWindow ? "选择窗口" : "选择标签页"} /></SelectTrigger><SelectContent>{!parameter.required && <SelectItem value={emptyBrowserTarget}>{parameter.placeholder || (isWindow ? "当前窗口" : "不指定标签页")}</SelectItem>}{options.map((option) => <SelectItem key={option.id} value={String(option.id)}>{option.label}</SelectItem>)}</SelectContent></Select></label>;
}

export function ParameterField({ parameter, value, values, onChange, browserTargets }) {
  if (!isParameterVisible(parameter, values)) return null;
  const enabled = isParameterEnabled(parameter, values);
  const common = { disabled: !enabled, required: parameter.required };
  const numberType = ["integer", "number", "duration"].includes(parameter.type);
  const wide = parameter.full_width || ["text", "json", "map"].includes(parameter.type) || parameter.format === "json";
  const fieldClass = `field${wide ? " parameter-field-wide" : ""}`;
  const inputType = parameter.type === "url" || parameter.type === "email" ? parameter.type : parameter.secret ? "password" : numberType ? "number" : parameter.type === "date" ? "date" : parameter.type === "datetime" ? "datetime-local" : "text";

  if (["browser_window", "browser_tab"].includes(parameter.type)) return <BrowserTargetField parameter={parameter} value={value} onChange={onChange} disabled={!enabled} browserTargets={browserTargets} />;
  if (parameter.type === "boolean") return <div className={`${fieldClass} parameter-boolean-field`}><span className="parameter-boolean-copy"><FieldLabel parameter={parameter} /><FieldHint parameter={parameter} /></span><Switch checked={Boolean(value)} onCheckedChange={onChange} disabled={!enabled} aria-label={parameter.label} /></div>;
  if (parameter.type === "map") return <div className={fieldClass}><MapField parameter={parameter} disabled={!enabled} value={value} onChange={onChange} /></div>;
  if (parameter.type === "list") return <label className={fieldClass}><FieldHeading parameter={parameter} /><Select value={value === "" || value == null ? undefined : String(value)} onValueChange={onChange} disabled={!enabled}><SelectTrigger><SelectValue placeholder={parameter.placeholder || "请选择"} /></SelectTrigger><SelectContent>{getParameterOptions(parameter, values).map((option) => <SelectItem key={String(option.value)} value={String(option.value)}>{option.label || option.value}</SelectItem>)}</SelectContent></Select></label>;
  if (parameter.type === "time") return <label className={fieldClass}><FieldHeading parameter={parameter} /><TimeField disabled={!enabled} value={value} onChange={onChange} parameter={parameter} /></label>;
  if (parameter.type === "text" || parameter.type === "json" || parameter.format === "json") return <label className={fieldClass}><FieldHeading parameter={parameter} />{parameter.type === "json" || parameter.format === "json" ? <JsonField value={value} onChange={onChange} parameter={parameter} asObject={parameter.type === "json"} disabled={!enabled} /> : <textarea className={`field-textarea${parameter.format === "code" ? " is-code" : ""}`} rows={parameter.rows || 5} value={value ?? ""} onChange={(event) => onChange(event.target.value)} {...common} placeholder={parameter.placeholder} />}</label>;

  const ValueInput = parameter.secret ? PasswordInput : Input;
  return <label className={fieldClass}><FieldHeading parameter={parameter} /><div className={parameter.unit ? "input-with-suffix" : ""}><ValueInput {...common} type={inputType} min={parameter.minimum} max={parameter.maximum} step={parameter.step || (numberType ? 1 : undefined)} value={value ?? ""} onChange={(event) => onChange(numberType ? (event.target.value === "" ? "" : Number(event.target.value)) : event.target.value)} placeholder={parameter.placeholder} />{parameter.unit && <span>{parameter.unit}</span>}</div></label>;
}
