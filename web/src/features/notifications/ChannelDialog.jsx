import React, { useEffect, useState } from "react";
import { Bell, Check, LoaderCircle, PlayCircle } from "lucide-react";
import { getDefaultValues, getParameters, sanitizeValues } from "../../lib/parameterSchema";
import { api } from "../../lib/api";
import { Button } from "../../components/ui/Button";
import { IconButton } from "../../components/ui/IconButton";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { Input, Label } from "../../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { DynamicFields } from "../../components/forms/DynamicFields";
import { PlaceholderHelper } from "../../components/forms/PlaceholderHelper";

export function ChannelDialog({ channel, notifiers, monitors, modules, onClose, onSaved, onError, onTest }) {
  const isEditing = Boolean(channel?.id);
  const isDuplicate = Boolean(channel?.__duplicate);
  const availableNotifiers = isEditing && channel?.built_in ? notifiers : notifiers.filter((item) => item.type !== "inapp");
  const [type, setType] = useState(channel?.notifier_type || availableNotifiers[0]?.type || "webhook");
  const descriptor = availableNotifiers.find((item) => item.type === type) || availableNotifiers[0];
  const parameters = getParameters(descriptor);
  const [name, setName] = useState(channel?.name || "");
  const [config, setConfig] = useState({});
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    if (descriptor) setConfig(getDefaultValues(descriptor, channel?.notifier_type === type ? channel.config : {}));
  }, [channel, type, descriptor]);

  useEffect(() => {
    setName(channel?.name || "");
    const firstType = notifiers.find((item) => item.type !== "inapp")?.type || "webhook";
    setType(channel?.notifier_type || firstType);
  }, [channel, notifiers]);

  if (!descriptor) return null;
  const submit = async (event) => {
    event.preventDefault();
    setSaving(true);
    try {
      await api(isEditing ? `/api/v1/notification-channels/${channel.id}` : "/api/v1/notification-channels", { method: isEditing ? "PATCH" : "POST", body: JSON.stringify({ name, notifier_type: type, enabled: channel?.enabled ?? true, config: sanitizeValues(parameters, config) }) });
      onSaved();
    } catch (error) {
      onError(error.message);
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    if (!onTest || saving || testing) return;
    setTesting(true);
    try {
      await onTest({ notifier_type: type, config: sanitizeValues(parameters, config) });
    } finally {
      setTesting(false);
    }
  };

  return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent><DialogHeader><div><span className="eyebrow">{isDuplicate ? "DUPLICATE CHANNEL" : "DELIVERY CHANNEL"}</span><DialogTitle>{isEditing ? "编辑通知渠道" : isDuplicate ? "复制通知渠道" : "添加通知渠道"}</DialogTitle><DialogDescription>{isDuplicate ? "基于现有渠道配置创建副本，可在保存前调整差异。" : "配置可供多个监控项复用的通知出口。"}</DialogDescription></div></DialogHeader><form onSubmit={submit}><div className="modal-body"><div className="form-grid"><label className="field"><Label>名称</Label><Input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：团队 Webhook" /></label><label className="field"><Label>类型</Label><Select value={type} onValueChange={setType}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{availableNotifiers.map((item) => <SelectItem key={item.type} value={item.type}>{item.name}</SelectItem>)}</SelectContent></Select></label></div><div className="form-section"><div className="form-section-title"><div><h3>{descriptor.name} 配置</h3><p>{descriptor.description}</p></div><div className="form-section-actions"><PlaceholderHelper monitors={monitors} modules={modules} /><Bell size={17} /></div></div><DynamicFields parameters={parameters} values={config} onChange={(key, value) => setConfig((current) => ({ ...current, [key]: value }))} /></div></div><DialogFooter className="dialog-footer-split dialog-footer-compact-mobile"><Button className="dialog-test-button" type="button" variant="outline" onClick={test} disabled={saving || testing}>{testing ? "测试中..." : <><PlayCircle size={15} />测试通知</>}</Button><IconButton className="dialog-test-icon" type="button" variant="outline" size="default" title={testing ? "正在测试通知" : "测试通知"} aria-label={testing ? "正在测试通知" : "测试通知"} onClick={test} disabled={saving || testing}>{testing ? <LoaderCircle className="spin" size={15} /> : <PlayCircle size={15} />}</IconButton><div className="dialog-footer-actions"><Button type="button" variant="ghost" onClick={onClose}>取消</Button><Button type="submit" disabled={saving || testing}>{saving ? "保存中..." : <><Check size={15} />{isEditing ? "保存修改" : isDuplicate ? "创建副本" : "保存渠道"}</>}</Button></div></DialogFooter></form></DialogContent></Dialog>;
}
