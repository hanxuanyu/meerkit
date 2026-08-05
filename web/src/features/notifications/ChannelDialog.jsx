import React, { useEffect, useState } from "react";
import { Bell, Check } from "lucide-react";
import { getDefaultValues, getParameters, sanitizeValues } from "../../lib/parameterSchema";
import { api } from "../../lib/api";
import { Button } from "../../components/ui/Button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { Input, Label } from "../../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { DynamicFields } from "../../components/forms/DynamicFields";

export function ChannelDialog({ notifiers, onClose, onSaved, onError }) {
  const [type, setType] = useState(notifiers[0]?.type || "webhook");
  const descriptor = notifiers.find((item) => item.type === type) || notifiers[0];
  const parameters = getParameters(descriptor);
  const [name, setName] = useState("");
  const [config, setConfig] = useState({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (descriptor) setConfig(getDefaultValues(descriptor));
  }, [type, descriptor]);

  if (!descriptor) return null;
  const submit = async (event) => {
    event.preventDefault();
    setSaving(true);
    try {
      await api("/api/v1/notification-channels", { method: "POST", body: JSON.stringify({ name, notifier_type: type, enabled: true, config: sanitizeValues(parameters, config) }) });
      onSaved();
    } catch (error) {
      onError(error.message);
    } finally {
      setSaving(false);
    }
  };

  return <Dialog open onOpenChange={(open) => !open && onClose()}><DialogContent><DialogHeader><div><span className="eyebrow">DELIVERY CHANNEL</span><DialogTitle>添加通知渠道</DialogTitle><DialogDescription>配置可供多个监控项复用的通知出口。</DialogDescription></div></DialogHeader><form onSubmit={submit}><div className="modal-body"><div className="form-grid"><label className="field"><Label>名称</Label><Input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：团队 Webhook" /></label><label className="field"><Label>类型</Label><Select value={type} onValueChange={setType}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{notifiers.map((item) => <SelectItem key={item.type} value={item.type}>{item.name}</SelectItem>)}</SelectContent></Select></label></div><div className="form-section"><div className="form-section-title"><div><h3>{descriptor.name} 配置</h3><p>{descriptor.description}</p></div><Bell size={17} /></div><DynamicFields parameters={parameters} values={config} onChange={(key, value) => setConfig((current) => ({ ...current, [key]: value }))} /></div></div><DialogFooter><Button type="button" variant="ghost" onClick={onClose}>取消</Button><Button type="submit" disabled={saving}>{saving ? "保存中..." : <><Check size={15} />保存渠道</>}</Button></DialogFooter></form></DialogContent></Dialog>;
}
