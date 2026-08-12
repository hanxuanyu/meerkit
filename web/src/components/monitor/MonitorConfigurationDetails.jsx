import React, { useEffect, useState } from "react";
import { operators } from "../../lib/constants";
import { getParameterOptions, getParameters, isParameterVisible } from "../../lib/parameterSchema";
import { getResultFields } from "../../lib/resultSchema";
import { previewSchedule, schedulePreviewLabel, schedulePreviewTitle } from "../../lib/schedules";
import { Badge } from "../ui/Badge";

const operatorsWithoutValue = new Set(["changed", "is_true", "is_false", "exists", "not_exists"]);

export function MonitorConfigurationDetails({ monitor, descriptor, channels = [] }) {
  const values = monitor?.module_config || {};
  const parameters = getParameters(descriptor || {}).filter((parameter) => isParameterVisible(parameter, values));
  return <div className="monitor-configuration-details">
    <CompactSection title={`${descriptor?.name || monitor?.module_type?.toUpperCase() || "监控"} 参数`} count={parameters.length}>
      <div className="monitor-compact-values">{parameters.length ? parameters.map((parameter) => <ParameterValue key={parameter.key} parameter={parameter} value={values[parameter.key]} values={values} />) : <EmptyBadge>没有模块参数</EmptyBadge>}</div>
    </CompactSection>
    <ScheduleDetails schedules={monitor?.schedules || []} />
    <ConditionDetails conditionConfig={monitor?.condition_config || {}} descriptor={descriptor} />
    <ChannelDetails channelIDs={monitor?.notification_channel_ids || []} channels={channels} />
  </div>;
}

function CompactSection({ title, count, children }) {
  return <section className="monitor-compact-section"><div className="monitor-compact-section-label"><strong>{title}</strong>{Number.isFinite(count) && <small>{count}</small>}</div><div className="monitor-compact-section-content">{children}</div></section>;
}

function ParameterValue({ parameter, value, values }) {
  const label = parameter.label || parameter.key;
  const type = parameter.type || "string";
  if (parameter.secret && value) return <ValueBadge label={label} value="••••••" />;
  if (type === "map" || (value && typeof value === "object" && !Array.isArray(value))) {
    const entries = Object.entries(value || {});
    return entries.length ? entries.map(([key, item]) => <ValueBadge key={`${parameter.key}-${key}`} label={`${label} · ${key}`} value={parameter.secret ? "••••••" : formatValue(item)} mono />) : <ValueBadge label={label} value="未配置" muted />;
  }
  if (Array.isArray(value)) {
    return value.length ? value.map((item, index) => <ValueBadge key={`${parameter.key}-${index}`} label={`${label} ${index + 1}`} value={formatValue(item)} />) : <ValueBadge label={label} value="未配置" muted />;
  }
  const optionLabel = type === "list" ? getParameterOptions(parameter, values).find((option) => String(option.value) === String(value))?.label : "";
  const display = type === "boolean" ? (value ? "是" : "否") : optionLabel || formatValue(value, parameter.unit);
  return <ValueBadge label={label} value={display} tone={type === "boolean" ? (value ? "success" : "muted") : "neutral"} mono={["json", "text"].includes(type) || parameter.format === "json"} numeric={["number", "integer", "duration"].includes(type)} />;
}

function ScheduleDetails({ schedules }) {
  return <CompactSection title="执行计划" count={schedules.length}><div className="monitor-compact-values">{schedules.length ? schedules.map((schedule, index) => <ScheduleValue key={`${schedule}-${index}`} schedule={schedule} index={index} />) : <EmptyBadge>未配置执行计划</EmptyBadge>}</div></CompactSection>;
}

function ScheduleValue({ schedule, index }) {
  const [preview, setPreview] = useState(null);
  const [error, setError] = useState("");
  useEffect(() => {
    let cancelled = false;
    previewSchedule(schedule).then((result) => { if (!cancelled) { setPreview(result); setError(""); } }).catch((loadError) => { if (!cancelled) { setPreview(null); setError(loadError.message); } });
    return () => { cancelled = true; };
  }, [schedule]);
  return <ValueBadge label={`计划 ${index + 1}`} value={schedule} meta={preview ? schedulePreviewLabel(preview) : error ? "格式无效" : "正在计算"} title={preview ? schedulePreviewTitle(preview) : error} tone={error ? "warning" : "neutral"} mono />;
}

function ConditionDetails({ conditionConfig, descriptor }) {
  const rules = conditionConfig.rules || [];
  const fields = getResultFields(descriptor);
  const logic = conditionConfig.logic === "ANY" ? "任一条件" : "全部条件";
  const policy = conditionConfig.notification_policy === "every" ? "每次满足都通知" : "同一场景仅通知一次";
  return <CompactSection title="触发条件" count={rules.length}><div className="monitor-compact-values monitor-condition-values"><Badge variant="outline" tone="muted">{logic}</Badge><Badge variant="outline" tone="muted">{policy}</Badge>{rules.length ? rules.map((rule, index) => <ConditionRule key={`${rule.field}-${index}`} rule={rule} index={index} fields={fields} />) : <EmptyBadge>未配置触发条件</EmptyBadge>}</div></CompactSection>;
}

function ConditionRule({ rule, index, fields }) {
  const field = findField(rule.field, fields);
  const comparesExecutions = rule.source === "previous" && rule.value_source === "current";
  const left = `${rule.source === "previous" ? "上次 · " : ""}${fieldLabel(field, rule.field, rule.path)}`;
  const operator = operators[rule.operator] || rule.operator || "判断";
  const right = operatorsWithoutValue.has(rule.operator) ? "" : comparesExecutions
    ? `当前 · ${fieldLabel(findField(rule.value_field || rule.field, fields), rule.value_field || rule.field, rule.value_path || rule.path)}`
    : rule.value_source === "current" || rule.value_source === "previous"
      ? `${rule.value_source === "previous" ? "上次" : "当前"} · ${fieldLabel(findField(rule.value_field, fields), rule.value_field, rule.value_path)}`
      : formatValue(rule.value);
  return <ValueBadge className="compact-condition-badge" label={`条件 ${index + 1}`} value={`${left} ${operator}${right ? ` ${right}` : ""}`} />;
}

function ChannelDetails({ channelIDs, channels }) {
  return <CompactSection title="通知渠道" count={channelIDs.length}><div className="monitor-compact-values">{channelIDs.length ? channelIDs.map((id) => {
    const channel = channels.find((item) => item.id === id);
    const status = !channel ? "配置缺失" : channel.enabled ? "已启用" : "已停用";
    return <ValueBadge key={id} label={channel?.notifier_type?.toUpperCase() || "未知类型"} value={channel?.name || id} meta={status} tone={!channel ? "warning" : channel.enabled ? "success" : "muted"} />;
  }) : <EmptyBadge>未选择通知渠道</EmptyBadge>}</div></CompactSection>;
}

function ValueBadge({ label, value, meta = "", title = "", tone = "neutral", mono = false, numeric = false, muted = false, className = "" }) {
  const visibleValue = value === undefined || value === null || value === "" ? "未配置" : String(value).replace(/\s*\n\s*/g, " ");
  const classes = ["compact-detail-badge", mono ? "is-mono" : "", numeric ? "is-numeric" : "", className].filter(Boolean).join(" ");
  return <Badge variant="outline" tone={muted ? "muted" : tone} className={classes} title={title || `${label}：${visibleValue}${meta ? ` · ${meta}` : ""}`}><span>{label}</span><strong>{visibleValue}</strong>{meta && <em>{meta}</em>}</Badge>;
}

function EmptyBadge({ children }) {
  return <Badge variant="outline" tone="muted" className="monitor-compact-empty">{children}</Badge>;
}

function findField(name, fields) {
  const normalized = String(name || "").replace(/^(result|current|previous)\./, "");
  return fields.find((field) => field.name === normalized);
}

function fieldLabel(field, fallback, path) {
  const base = field?.label || fallback || "未知字段";
  return path ? `${base} · ${path}` : base;
}

function formatValue(value, unit = "") {
  if (value === undefined || value === null || value === "") return "未配置";
  if (Array.isArray(value)) return value.map((item) => formatValue(item)).join(" · ");
  if (typeof value === "object") return Object.entries(value).map(([key, item]) => `${key}: ${formatValue(item)}`).join(" · ");
  return `${String(value)}${unit ? ` ${unit}` : ""}`;
}
