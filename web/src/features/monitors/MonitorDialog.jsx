import React, { useEffect, useState } from "react";
import { Bell, Check, Clock3, PlayCircle, Plus, Radio, Trash2 } from "lucide-react";
import { cronPresets, defaultModuleConfig } from "../../lib/constants";
import { getDefaultValues, getParameters, sanitizeValues } from "../../lib/parameterSchema";
import { api } from "../../lib/api";
import { Button } from "../../components/ui/Button";
import { Checkbox } from "../../components/ui/Checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { IconButton } from "../../components/ui/IconButton";
import { Input, Label } from "../../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { ConditionEditor } from "../../components/forms/ConditionEditor";
import { DynamicFields } from "../../components/forms/DynamicFields";
import { getResultFields, findUnsupportedPlaceholders } from "../../lib/resultSchema";

export function MonitorDialog({ monitor, modules, channels, onClose, onSaved, onError, onTest }) {
  const [moduleType, setModuleType] = useState(monitor?.module_type || modules[0]?.type || "http");
  const descriptor = modules.find((item) => item.type === moduleType) || modules[0];
  const parameters = getParameters(descriptor);
  const [name, setName] = useState(monitor?.name || "");
  const [schedules, setSchedules] = useState(() => monitor?.schedules || ["*/5 * * * *"]);
  const [config, setConfig] = useState(() => monitor?.module_config || {});
  const [conditionConfig, setConditionConfig] = useState(() => monitor?.condition_config || { logic: "ALL", rules: [] });
  const [channelIDs, setChannelIDs] = useState(monitor?.notification_channel_ids || []);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    if (!monitor && descriptor) setConfig(getDefaultValues(descriptor, defaultModuleConfig[moduleType] || {}));
  }, [moduleType, monitor, descriptor]);

  if (!descriptor) return null;

  const updateConfig = (key, value) => setConfig((current) => ({ ...current, [key]: value }));
  const updateSchedule = (index, value) => setSchedules((current) => current.map((schedule, itemIndex) => itemIndex === index ? value : schedule));
  const addSchedule = () => setSchedules((current) => [...current, "*/5 * * * *"]);
  const removeSchedule = (index) => setSchedules((current) => current.length <= 1 ? current : current.filter((_, itemIndex) => itemIndex !== index));
  const addRule = () => { const field = getResultFields(descriptor)[0]; setConditionConfig((current) => ({ ...current, rules: [...(current.rules || []), { field: field?.name || "", source: "current", operator: field?.operators?.[0] || "equals", value_source: "literal", value: "" }] })); };

  const submit = async (event) => {
    event.preventDefault();
    setSaving(true);
    try {
      const payload = { name, module_type: moduleType, schedules, module_config: sanitizeValues(parameters, config), condition_config: conditionConfig, notification_channel_ids: channelIDs };
      await api(monitor ? `/api/v1/monitors/${monitor.id}` : "/api/v1/monitors", { method: monitor ? "PATCH" : "POST", body: JSON.stringify(payload) });
      onSaved();
    } catch (error) {
      onError(error.message);
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    if (!onTest || testing || saving) return;
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
        <div><span className="eyebrow">{monitor ? "EDIT MONITOR" : "NEW MONITOR"}</span><DialogTitle>{monitor ? "编辑监控项" : "创建监控项"}</DialogTitle><DialogDescription>使用独立采集模块观察响应内容和连接状态。</DialogDescription></div>
      </DialogHeader>
      <form onSubmit={submit}>
        <div className="modal-body">
          <div className="form-grid">
            <label className="field"><Label>名称</Label><Input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：生产 API 响应" /></label>
            <label className="field"><Label>采集模块</Label><Select value={moduleType} onValueChange={setModuleType}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{modules.map((item) => <SelectItem key={item.type} value={item.type}>{item.name}</SelectItem>)}</SelectContent></Select></label>
          </div>

          <div className="form-section">
            <div className="form-section-title"><div><h3>执行计划</h3><p>可添加多个 cron 表达式，统一使用系统配置中的时区。</p></div><IconButton className="section-add" size="sm" title="添加 cron 表达式" aria-label="添加 cron 表达式" onClick={addSchedule}><Plus size={14} /></IconButton></div>
            <div className="schedule-list">
              <div className="schedule-head"><span>Cron 表达式</span><span>快速预设</span><span /></div>
              {schedules.map((schedule, index) => <div className="schedule-row" key={index}>
                <div className="schedule-control"><Input required aria-label={`Cron 表达式 ${index + 1}`} value={schedule} onChange={(event) => updateSchedule(index, event.target.value)} placeholder="*/5 * * * *" /></div>
                <div className="schedule-control"><Select value={cronPresets.some((preset) => preset.value === schedule) ? schedule : undefined} onValueChange={(value) => updateSchedule(index, value)}><SelectTrigger aria-label={`快速预设 ${index + 1}`}><SelectValue placeholder="快速预设" /></SelectTrigger><SelectContent className="schedule-preset-content">{cronPresets.map((preset) => <SelectItem className="schedule-preset-item" key={preset.value} value={preset.value}>{preset.label}</SelectItem>)}</SelectContent></Select></div>
                <IconButton className="schedule-remove" size="sm" title="删除表达式" aria-label={`删除第 ${index + 1} 个表达式`} disabled={schedules.length <= 1} onClick={() => removeSchedule(index)}><Trash2 size={14} /></IconButton>
              </div>)}
            </div>
          </div>

          <div className="form-section"><div className="form-section-title"><div><h3>{descriptor.name} 参数</h3><p>由采集模块声明，字段类型和依赖关系会自动生效。</p></div><Radio size={17} /></div><DynamicFields parameters={parameters} values={config} onChange={updateConfig} /></div>
          <div className="form-section"><div className="form-section-title"><div><h3>触发条件</h3><p>第一次执行只建立变化检测基线。</p></div><IconButton className="section-add" size="sm" title="添加条件" aria-label="添加条件" onClick={addRule}><Plus size={14} /></IconButton></div><ConditionEditor descriptor={descriptor} value={conditionConfig} onChange={setConditionConfig} /></div>
          <div className="form-section"><div className="form-section-title"><div><h3>通知渠道</h3><p>可选择多个渠道，触发和恢复时异步发送。</p></div><Bell size={17} /></div><div className="channel-checks">{channels.length ? channels.map((channel) => { const unsupported = findUnsupportedPlaceholders(channel.config, descriptor); return <label key={channel.id} className={`check-card${unsupported.length ? " check-card-warning" : ""}`}><Checkbox checked={channelIDs.includes(channel.id)} onCheckedChange={(checked) => setChannelIDs((current) => checked ? [...new Set([...current, channel.id])] : current.filter((id) => id !== channel.id))} /><span><strong>{channel.name}</strong><small>{channel.notifier_type.toUpperCase()}</small>{unsupported.length > 0 && <em title={unsupported.join(", ")}>包含当前模块无法提供的占位符：{unsupported.join(", ")}</em>}</span></label>; }) : <span className="muted-text">暂无通知渠道，可稍后在通知渠道页面添加。</span>}</div></div>
        </div>
        <DialogFooter className="dialog-footer-split"><Button type="button" variant="outline" onClick={test} disabled={saving || testing}>{testing ? "测试中..." : <><PlayCircle size={15} />测试调用</>}</Button><div className="dialog-footer-actions"><Button type="button" variant="ghost" onClick={onClose}>取消</Button><Button type="submit" disabled={saving || testing}>{saving ? "保存中..." : <><Check size={15} />保存监控</>}</Button></div></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}
