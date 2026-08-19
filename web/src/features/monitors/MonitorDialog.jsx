import React, { useEffect, useState } from "react";
import { Bell, Check, LoaderCircle, PlayCircle, Plus, Radio } from "lucide-react";
import { findMissingRequiredParameters, getDefaultValues, getParameters, sanitizeValues } from "../../lib/parameterSchema";
import { api } from "../../lib/api";
import { Button } from "../../components/ui/Button";
import { Checkbox } from "../../components/ui/Checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { IconButton } from "../../components/ui/IconButton";
import { Input, Label } from "../../components/ui/Input";
import { ConditionEditor } from "../../components/forms/ConditionEditor";
import { DynamicFields } from "../../components/forms/DynamicFields";
import { ModulePicker } from "../../components/forms/ModulePicker";
import { getResultFields, findUnsupportedPlaceholders } from "../../lib/resultSchema";
import { previewSchedule } from "../../lib/schedules";
import { CronScheduleRow } from "./CronScheduleRow";

export function MonitorDialog({ monitor, modules, channels, onClose, onSaved, onError, onTest }) {
  const isEditing = Boolean(monitor?.id);
  const isDuplicate = Boolean(monitor?.__duplicate);
  const [moduleType, setModuleType] = useState(monitor?.module_type || modules[0]?.type || "");
  const descriptor = modules.find((item) => item.type === moduleType);
  const parameters = getParameters(descriptor);
  const [name, setName] = useState(monitor?.name || "");
  const [schedules, setSchedules] = useState(() => monitor?.schedules || ["*/5 * * * *"]);
  const [config, setConfig] = useState(() => monitor?.module_config || {});
  const [conditionConfig, setConditionConfig] = useState(() => monitor?.condition_config || { logic: "ALL", rules: [] });
  const [channelIDs, setChannelIDs] = useState(monitor?.notification_channel_ids || []);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    if (!monitor && !descriptor) setModuleType(modules[0]?.type || "");
  }, [monitor, modules, descriptor]);

  useEffect(() => {
    if (!monitor && descriptor) setConfig(getDefaultValues(descriptor));
  }, [moduleType, monitor, descriptor]);

  if (!descriptor) return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent>
      <DialogHeader>
        <div><span className="eyebrow">MONITOR MODULE</span><DialogTitle>采集模块不可用</DialogTitle><DialogDescription>{monitor ? `当前未注册类型为 ${monitor.module_type} 的采集模块，暂时无法编辑此监控项。` : "当前没有可用于创建监控项的采集模块。"}</DialogDescription></div>
      </DialogHeader>
      <DialogFooter><Button type="button" onClick={onClose}>关闭</Button></DialogFooter>
    </DialogContent>
  </Dialog>;

  const updateConfig = (key, value) => setConfig((current) => ({ ...current, [key]: value }));
  const updateSchedule = (index, value) => setSchedules((current) => current.map((schedule, itemIndex) => itemIndex === index ? value : schedule));
  const addSchedule = () => setSchedules((current) => [...current, "*/5 * * * *"]);
  const removeSchedule = (index) => setSchedules((current) => current.length <= 1 ? current : current.filter((_, itemIndex) => itemIndex !== index));
  const addRule = () => { const field = getResultFields(descriptor)[0]; setConditionConfig((current) => ({ ...current, rules: [...(current.rules || []), { id: crypto.randomUUID(), field: field?.name || "", source: "current", operator: field?.operators?.[0] || "equals", value_source: "literal", value: "" }] })); };

  const submit = async (event) => {
    event.preventDefault();
    if (!name.trim()) { onError("请输入监控名称"); return; }
    const missing = findMissingRequiredParameters(parameters, config);
    if (missing.length) { onError(`请填写必填参数：${missing.map((parameter) => parameter.label || parameter.key).join("、")}`); return; }
    setSaving(true);
    try {
      const normalizedSchedules = schedules.map((schedule) => schedule.trim());
      await Promise.all(normalizedSchedules.map(previewSchedule));
      const payload = { name, module_type: moduleType, schedules: normalizedSchedules, enabled: monitor?.enabled ?? true, module_config: sanitizeValues(parameters, config), condition_config: conditionConfig, notification_channel_ids: channelIDs };
      await api(isEditing ? `/api/v1/monitors/${monitor.id}` : "/api/v1/monitors", { method: isEditing ? "PATCH" : "POST", body: JSON.stringify(payload) });
      onSaved();
    } catch (error) {
      onError(error.message);
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    if (!onTest || testing || saving) return;
    const missing = findMissingRequiredParameters(parameters, config);
    if (missing.length) { onError(`请填写必填参数：${missing.map((parameter) => parameter.label || parameter.key).join("、")}`); return; }
    setTesting(true);
    try {
      await onTest({ module_type: moduleType, module_config: sanitizeValues(parameters, config) });
    } finally {
      setTesting(false);
    }
  };

  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="modal-wide">
      <DialogHeader>
        <div><span className="eyebrow">{isEditing ? "EDIT MONITOR" : isDuplicate ? "DUPLICATE MONITOR" : "NEW MONITOR"}</span><DialogTitle>{isEditing ? "编辑监控项" : isDuplicate ? "复制监控项" : "创建监控项"}</DialogTitle><DialogDescription>{isDuplicate ? "基于现有监控配置创建副本，可在保存前调整差异。" : "使用采集模块定期获取结果并评估触发条件。"}</DialogDescription></div>
      </DialogHeader>
      <form noValidate onSubmit={submit}>
        <div className="modal-body">
          <div className="form-grid">
            <label className="field"><Label>名称</Label><Input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：生产 API 响应" /></label>
            <div className="field"><Label id="monitor-module-label">采集模块</Label><ModulePicker id="monitor-module-picker" aria-labelledby="monitor-module-label" modules={modules} value={moduleType} onValueChange={setModuleType} /></div>
          </div>

          <div className="form-section">
            <div className="form-section-title"><div><h3>执行计划</h3><p>可添加多个 cron 表达式，统一使用系统配置中的时区。</p></div><IconButton className="section-add" size="sm" title="添加 cron 表达式" aria-label="添加 cron 表达式" onClick={addSchedule}><Plus size={14} /></IconButton></div>
            <div className="schedule-list">
              {schedules.map((schedule, index) => <CronScheduleRow key={index} schedule={schedule} index={index} removable={schedules.length > 1} onChange={(value) => updateSchedule(index, value)} onRemove={() => removeSchedule(index)} />)}
            </div>
          </div>

          <div className="form-section"><div className="form-section-title"><div><h3>{descriptor.name} 参数</h3><p>由采集模块声明，字段类型和依赖关系会自动生效。</p></div><Radio size={17} /></div><DynamicFields parameters={parameters} values={config} onChange={updateConfig} /></div>
          <div className="form-section"><div className="form-section-title"><div><h3>触发条件</h3><p>第一次执行只建立变化检测基线。</p></div><IconButton className="section-add" size="sm" title="添加条件" aria-label="添加条件" onClick={addRule}><Plus size={14} /></IconButton></div><ConditionEditor descriptor={descriptor} value={conditionConfig} onChange={setConditionConfig} /></div>
          <div className="form-section"><div className="form-section-title"><div><h3>通知渠道</h3><p>可选择多个渠道，触发和恢复时异步发送。</p></div><Bell size={17} /></div><div className="channel-checks">{channels.length ? channels.map((channel) => { const unsupported = findUnsupportedPlaceholders(channel.config, descriptor); return <label key={channel.id} className={`check-card${unsupported.length ? " check-card-warning" : ""}`}><Checkbox checked={channelIDs.includes(channel.id)} onCheckedChange={(checked) => setChannelIDs((current) => checked ? [...new Set([...current, channel.id])] : current.filter((id) => id !== channel.id))} /><span><strong>{channel.name}</strong><small>{channel.notifier_type.toUpperCase()}</small>{unsupported.length > 0 && <em title={unsupported.join(", ")}>包含当前模块无法提供的占位符：{unsupported.join(", ")}</em>}</span></label>; }) : <span className="muted-text">暂无通知渠道，可稍后在通知渠道页面添加。</span>}</div></div>
        </div>
        <DialogFooter className="dialog-footer-split dialog-footer-compact-mobile"><Button className="dialog-test-button" type="button" variant="outline" onClick={test} disabled={saving || testing}>{testing ? "测试中..." : <><PlayCircle size={15} />测试调用</>}</Button><IconButton className="dialog-test-icon" type="button" variant="outline" size="default" title={testing ? "正在测试调用" : "测试调用"} aria-label={testing ? "正在测试调用" : "测试调用"} onClick={test} disabled={saving || testing}>{testing ? <LoaderCircle className="spin" size={15} /> : <PlayCircle size={15} />}</IconButton><div className="dialog-footer-actions"><Button type="button" variant="ghost" onClick={onClose}>取消</Button><Button type="submit" disabled={saving || testing}>{saving ? "保存中..." : <><Check size={15} />{isDuplicate ? "创建副本" : "保存监控"}</>}</Button></div></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}
