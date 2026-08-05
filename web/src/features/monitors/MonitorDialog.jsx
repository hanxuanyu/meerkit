import React, { useEffect, useState } from "react";
import { Bell, Check, Clock3, Plus, Radio } from "lucide-react";
import { defaultModuleConfig } from "../../lib/constants";
import { getDefaultValues, getParameters, sanitizeValues } from "../../lib/parameterSchema";
import { api } from "../../lib/api";
import { Button } from "../../components/ui/Button";
import { Checkbox } from "../../components/ui/Checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { IconButton } from "../../components/ui/IconButton";
import { Input, Label } from "../../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { Switch } from "../../components/ui/Switch";
import { ConditionEditor } from "../../components/forms/ConditionEditor";
import { DynamicFields } from "../../components/forms/DynamicFields";

export function MonitorDialog({ monitor, modules, channels, defaultTimezone, onClose, onSaved, onError }) {
  const [moduleType, setModuleType] = useState(monitor?.module_type || modules[0]?.type || "http");
  const descriptor = modules.find((item) => item.type === moduleType) || modules[0];
  const parameters = getParameters(descriptor);
  const [name, setName] = useState(monitor?.name || "");
  const [schedule, setSchedule] = useState(monitor?.schedule || "*/5 * * * *");
  const [timezone, setTimezone] = useState(monitor?.timezone || defaultTimezone);
  const [enabled, setEnabled] = useState(monitor?.enabled ?? true);
  const [config, setConfig] = useState(() => monitor?.module_config || {});
  const [conditionConfig, setConditionConfig] = useState(() => monitor?.condition_config || { logic: "ALL", rules: [] });
  const [channelIDs, setChannelIDs] = useState(monitor?.notification_channel_ids || []);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!monitor && descriptor) setConfig(getDefaultValues(descriptor, defaultModuleConfig[moduleType] || {}));
  }, [moduleType, monitor, descriptor]);

  if (!descriptor) return null;
  const updateConfig = (key, value) => setConfig((current) => ({ ...current, [key]: value }));
  const addRule = () => setConditionConfig((current) => ({ ...current, rules: [...(current.rules || []), { field: descriptor.fields?.[0]?.name || "success", operator: descriptor.fields?.[0]?.operators?.[0] || "equals", value: "" }] }));
  const submit = async (event) => {
    event.preventDefault();
    setSaving(true);
    try {
      const payload = { name, module_type: moduleType, schedule, timezone, enabled, module_config: sanitizeValues(parameters, config), condition_config: conditionConfig, notification_channel_ids: channelIDs };
      await api(monitor ? `/api/v1/monitors/${monitor.id}` : "/api/v1/monitors", { method: monitor ? "PATCH" : "POST", body: JSON.stringify(payload) });
      onSaved();
    } catch (error) {
      onError(error.message);
    } finally {
      setSaving(false);
    }
  };

  return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent className="modal-wide"><DialogHeader><div><span className="eyebrow">{monitor ? "EDIT MONITOR" : "NEW MONITOR"}</span><DialogTitle>{monitor ? "编辑监控项" : "创建监控项"}</DialogTitle><DialogDescription>使用独立采集模块观察响应内容和连接状态。</DialogDescription></div></DialogHeader><form onSubmit={submit}><div className="modal-body"><div className="form-grid"><label className="field"><Label>名称</Label><Input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：生产 API 响应" /></label><label className="field"><Label>采集模块</Label><Select value={moduleType} onValueChange={setModuleType}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{modules.map((item) => <SelectItem key={item.type} value={item.type}>{item.name}</SelectItem>)}</SelectContent></Select></label></div><div className="form-section"><div className="form-section-title"><div><h3>执行计划</h3><p>使用 cron 表达式，支持 5 段或 6 段格式。</p></div><Clock3 size={17} /></div><div className="form-grid"><label className="field"><Label>Cron 表达式</Label><Input required value={schedule} onChange={(event) => setSchedule(event.target.value)} placeholder="*/5 * * * *" /><small>例如每 5 分钟执行一次</small></label><label className="field"><Label>时区</Label><Input value={timezone} onChange={(event) => setTimezone(event.target.value)} placeholder="Asia/Shanghai" /></label></div></div><div className="form-section"><div className="form-section-title"><div><h3>{descriptor.name} 参数</h3><p>由采集模块声明，字段类型和依赖关系会自动生效。</p></div><Radio size={17} /></div><DynamicFields parameters={parameters} values={config} onChange={updateConfig} /></div><div className="form-section"><div className="form-section-title"><div><h3>触发条件</h3><p>第一次执行只建立变化检测基线。</p></div><IconButton className="section-add" size="sm" title="添加条件" aria-label="添加条件" onClick={addRule}><Plus size={14} /></IconButton></div><ConditionEditor descriptor={descriptor} value={conditionConfig} onChange={setConditionConfig} /></div><div className="form-section"><div className="form-section-title"><div><h3>通知渠道</h3><p>可选择多个渠道，触发和恢复时异步发送。</p></div><Bell size={17} /></div><div className="channel-checks">{channels.length ? channels.map((channel) => <label key={channel.id} className="check-card"><Checkbox checked={channelIDs.includes(channel.id)} onCheckedChange={(checked) => setChannelIDs((current) => checked ? [...new Set([...current, channel.id])] : current.filter((id) => id !== channel.id))} /><span><strong>{channel.name}</strong><small>{channel.notifier_type.toUpperCase()}</small></span></label>) : <span className="muted-text">暂无通知渠道，可稍后在通知渠道页面添加。</span>}</div></div><Switch checked={enabled} onCheckedChange={setEnabled} label="启用监控" description="保存后按下一个 cron 时间执行" /></div><DialogFooter><Button type="button" variant="ghost" onClick={onClose}>取消</Button><Button type="submit" disabled={saving}>{saving ? "保存中..." : <><Check size={15} />保存监控</>}</Button></DialogFooter></form></DialogContent></Dialog>;
}
