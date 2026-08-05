import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  Bell,
  Check,
  ChevronRight,
  Clock3,
  Copy,
  Database,
  Edit3,
  ExternalLink,
  Globe2,
  Inbox,
  Layers3,
  MoreHorizontal,
  Play,
  Plus,
  RefreshCw,
  Radio,
  Server,
  Settings2,
  ShieldCheck,
  Trash2,
  TriangleAlert,
  Waves,
  X
} from "lucide-react";
import "./styles.css";

const operators = {
  equals: "等于",
  not_equals: "不等于",
  contains: "包含",
  not_contains: "不包含",
  regex: "正则匹配",
  gt: "大于",
  gte: "大于等于",
  lt: "小于",
  lte: "小于等于",
  is_true: "为真",
  is_false: "为假",
  changed: "发生变化"
};

async function api(path, options) {
  const response = await fetch(path, { headers: { "Content-Type": "application/json" }, ...options });
  const body = response.status === 204 ? null : await response.json();
  if (!response.ok) throw new Error(body?.message || "请求失败");
  return body;
}

function Button({ children, variant = "default", size = "default", className = "", ...props }) {
  return <button className={`button button-${variant} button-${size} ${className}`} {...props}>{children}</button>;
}

function Badge({ children, tone = "neutral" }) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}

function Card({ children, className = "" }) {
  return <section className={`card ${className}`}>{children}</section>;
}

function EmptyState({ icon: Icon, title, description, action }) {
  return <div className="empty-state"><div className="empty-icon"><Icon size={22} /></div><h3>{title}</h3><p>{description}</p>{action}</div>;
}

function App() {
  const [activePage, setActivePage] = useState("overview");
  const [modules, setModules] = useState([]);
  const [notifiers, setNotifiers] = useState([]);
  const [monitors, setMonitors] = useState([]);
  const [channels, setChannels] = useState([]);
  const [selectedMonitor, setSelectedMonitor] = useState(null);
  const [showMonitorDialog, setShowMonitorDialog] = useState(false);
  const [showChannelDialog, setShowChannelDialog] = useState(false);
  const [toast, setToast] = useState(null);
  const [loading, setLoading] = useState(true);

  const refresh = async () => {
    setLoading(true);
    try {
      const [moduleResponse, notifierResponse, monitorResponse, channelResponse] = await Promise.all([
        api("/api/v1/modules"), api("/api/v1/notifiers"), api("/api/v1/monitors"), api("/api/v1/notification-channels")
      ]);
      setModules(moduleResponse.items || []);
      setNotifiers(notifierResponse.items || []);
      setMonitors(monitorResponse.items || []);
      setChannels(channelResponse.items || []);
    } catch (error) {
      notify(error.message, "error");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { refresh(); }, []);
  useEffect(() => {
    if (!toast) return undefined;
    const timer = window.setTimeout(() => setToast(null), 3200);
    return () => window.clearTimeout(timer);
  }, [toast]);

  function notify(message, tone = "success") { setToast({ message, tone }); }

  const openCreateMonitor = () => { setSelectedMonitor(null); setShowMonitorDialog(true); };
  const openEditMonitor = (monitor) => { setSelectedMonitor(monitor); setShowMonitorDialog(true); };

  const page = activePage === "overview"
    ? <Overview monitors={monitors} loading={loading} onCreate={openCreateMonitor} onOpen={(monitor) => { setSelectedMonitor(monitor); setActivePage("monitors"); }} onRun={async (monitor) => { try { await api(`/api/v1/monitors/${monitor.id}/run`, { method: "POST" }); notify("监控已执行"); refresh(); } catch (error) { notify(error.message, "error"); } }} />
    : activePage === "monitors"
      ? <Monitors monitors={monitors} loading={loading} onCreate={openCreateMonitor} onEdit={openEditMonitor} onRun={async (monitor) => { try { await api(`/api/v1/monitors/${monitor.id}/run`, { method: "POST" }); notify("监控已执行"); refresh(); } catch (error) { notify(error.message, "error"); } }} onDelete={async (monitor) => { if (!window.confirm(`删除监控“${monitor.name}”？`)) return; try { await api(`/api/v1/monitors/${monitor.id}`, { method: "DELETE" }); notify("监控已删除"); refresh(); } catch (error) { notify(error.message, "error"); } }} />
      : activePage === "notifications"
        ? <Notifications channels={channels} notifiers={notifiers} onCreate={() => setShowChannelDialog(true)} onRefresh={refresh} notify={notify} />
        : <SettingsPage />;

  return <div className="app-shell">
    <Sidebar activePage={activePage} setActivePage={setActivePage} />
    <main className="main-content">
      <header className="topbar"><div className="breadcrumb"><span>Meerkit</span><ChevronRight size={15} /><strong>{pageTitle(activePage)}</strong></div><div className="topbar-actions"><Button variant="ghost" size="icon" title="刷新数据" onClick={refresh}><RefreshCw size={16} /></Button><div className="avatar">MK</div></div></header>
      <div className="page-content">{page}</div>
    </main>
    {showMonitorDialog && <MonitorDialog monitor={selectedMonitor} modules={modules} channels={channels} defaultTimezone="Asia/Shanghai" onClose={() => setShowMonitorDialog(false)} onSaved={() => { setShowMonitorDialog(false); notify(selectedMonitor ? "监控已更新" : "监控已创建"); refresh(); }} onError={(message) => notify(message, "error")} />}
    {showChannelDialog && <ChannelDialog notifiers={notifiers} onClose={() => setShowChannelDialog(false)} onSaved={() => { setShowChannelDialog(false); notify("通知渠道已创建"); refresh(); }} onError={(message) => notify(message, "error")} />}
    {toast && <div className={`toast toast-${toast.tone}`}><span className="toast-dot">{toast.tone === "error" ? <TriangleAlert size={14} /> : <Check size={14} />}</span>{toast.message}</div>}
  </div>;
}

function Sidebar({ activePage, setActivePage }) {
  const navigation = [{ id: "overview", label: "总览", icon: Activity }, { id: "monitors", label: "监控项", icon: Layers3 }, { id: "notifications", label: "通知渠道", icon: Bell }];
  return <aside className="sidebar"><div className="brand"><div className="brand-mark"><Waves size={18} /></div><div><strong>Meerkit</strong><span>observability</span></div></div><div className="workspace-label">工作区</div><nav>{navigation.map(({ id, label, icon: Icon }) => <button key={id} className={`nav-item ${activePage === id ? "active" : ""}`} onClick={() => setActivePage(id)}><Icon size={17} /><span>{label}</span>{id === "notifications" && <span className="nav-count">·</span>}</button>)}</nav><div className="sidebar-spacer" /><button className={`nav-item ${activePage === "settings" ? "active" : ""}`} onClick={() => setActivePage("settings")}><Settings2 size={17} /><span>系统设置</span></button><div className="sidebar-footer"><div className="status-indicator"><span />服务运行中</div><span className="version">v0.1.0</span></div></aside>;
}

function Overview({ monitors, loading, onCreate, onOpen, onRun }) {
  const stats = { total: monitors.length, active: monitors.filter((item) => item.enabled).length, alert: monitors.filter((item) => item.runtime_state?.condition_active).length, healthy: monitors.filter((item) => item.runtime_state?.last_success).length };
  return <div className="page-stack"><div className="page-heading"><div><div className="eyebrow">SYSTEM OVERVIEW</div><h1>监控总览</h1><p>跟踪响应内容、连接状态和条件变化。</p></div><Button onClick={onCreate}><Plus size={16} />新建监控</Button></div><div className="stat-grid"><StatCard label="监控项" value={stats.total} hint={`${stats.active} 个正在运行`} icon={Layers3} /><StatCard label="当前正常" value={stats.healthy} hint="最近一次执行成功" icon={ShieldCheck} /><StatCard label="活跃触发" value={stats.alert} hint={stats.alert ? "需要关注" : "暂无待处理事件"} icon={TriangleAlert} tone={stats.alert ? "warning" : "neutral"} /><StatCard label="采集模块" value="2" hint="HTTP · TCP" icon={Radio} /></div><Card className="section-card"><div className="section-header"><div><h2>最近监控项</h2><p>所有已配置的数据采集任务</p></div><Button variant="ghost" onClick={() => onOpen(monitors[0])} disabled={!monitors.length}>查看全部<ChevronRight size={15} /></Button></div>{loading ? <div className="loading-list"><span /><span /><span /></div> : monitors.length ? <MonitorTable monitors={monitors.slice(0, 6)} onOpen={onOpen} onRun={onRun} /> : <EmptyState icon={Inbox} title="还没有监控项" description="创建第一个 HTTP 或 TCP 监控，开始观察数据变化。" action={<Button onClick={onCreate}><Plus size={16} />创建监控</Button>} />}</Card><div className="lower-grid"><Card className="info-card"><div className="info-icon"><Database size={18} /></div><div><h3>本地历史数据</h3><p>执行结果保存于 SQLite，可在监控详情中查看完整变化过程。</p></div><ChevronRight size={16} className="muted-icon" /></Card><Card className="info-card"><div className="info-icon"><Clock3 size={18} /></div><div><h3>Cron 调度</h3><p>使用标准 5 段或 6 段表达式灵活安排执行时间。</p></div><ChevronRight size={16} className="muted-icon" /></Card></div></div>;
}

function StatCard({ label, value, hint, icon: Icon, tone = "neutral" }) { return <Card className="stat-card"><div className="stat-top"><span>{label}</span><div className={`stat-icon stat-icon-${tone}`}><Icon size={17} /></div></div><strong>{value}</strong><small>{hint}</small></Card>; }

function Monitors({ monitors, loading, onCreate, onEdit, onRun, onDelete }) { return <div className="page-stack"><div className="page-heading"><div><div className="eyebrow">COLLECTION MODULES</div><h1>监控项</h1><p>配置采集模块、条件和执行计划。</p></div><Button onClick={onCreate}><Plus size={16} />新建监控</Button></div><Card className="section-card"><div className="toolbar"><div className="search-placeholder"><Activity size={15} />{monitors.length} 个监控项</div><Button variant="outline" size="icon" title="刷新"><RefreshCw size={15} /></Button></div>{loading ? <div className="loading-list"><span /><span /><span /></div> : monitors.length ? <MonitorTable monitors={monitors} onOpen={onEdit} onRun={onRun} onEdit={onEdit} onDelete={onDelete} /> : <EmptyState icon={Radio} title="监控列表为空" description="使用独立模块采集 HTTP 或 TCP 数据。" action={<Button onClick={onCreate}><Plus size={16} />新建监控</Button>} />}</Card></div>; }

function MonitorTable({ monitors, onOpen, onRun, onEdit, onDelete }) { return <div className="table-wrap"><table><thead><tr><th>监控项</th><th>模块</th><th>调度</th><th>最近执行</th><th>状态</th><th className="action-cell">操作</th></tr></thead><tbody>{monitors.map((monitor) => <tr key={monitor.id} onClick={() => onOpen(monitor)}><td><div className="monitor-name"><div className={`module-mark module-${monitor.module_type}`} >{monitor.module_type === "http" ? <Globe2 size={15} /> : <Server size={15} />}</div><div><strong>{monitor.name}</strong><span>{monitor.module_config?.url || `${monitor.module_config?.host || "-"}:${monitor.module_config?.port || "-"}`}</span></div></div></td><td><Badge>{monitor.module_type.toUpperCase()}</Badge></td><td><code>{monitor.schedule}</code></td><td><span className="last-run">{formatDate(monitor.runtime_state?.last_run_at)}</span></td><td><StatusBadge monitor={monitor} /></td><td className="action-cell" onClick={(event) => event.stopPropagation()}><div className="row-actions"><Button variant="ghost" size="icon" title="立即执行" onClick={() => onRun(monitor)}><Play size={15} /></Button>{onEdit && <Button variant="ghost" size="icon" title="编辑" onClick={() => onEdit(monitor)}><Edit3 size={15} /></Button>}{onDelete && <Button variant="ghost" size="icon" title="删除" onClick={() => onDelete(monitor)}><Trash2 size={15} /></Button>}<Button variant="ghost" size="icon" title="更多"><MoreHorizontal size={15} /></Button></div></td></tr>)}</tbody></table></div>; }

function StatusBadge({ monitor }) { if (!monitor.enabled) return <Badge tone="muted">已停用</Badge>; if (monitor.runtime_state?.condition_active) return <Badge tone="warning"><span className="status-dot" />已触发</Badge>; if (monitor.runtime_state?.last_success) return <Badge tone="success"><span className="status-dot" />正常</Badge>; return <Badge tone="muted"><span className="status-dot" />等待执行</Badge>; }

function Notifications({ channels, notifiers, onCreate, onRefresh, notify }) { return <div className="page-stack"><div className="page-heading"><div><div className="eyebrow">DELIVERY CHANNELS</div><h1>通知渠道</h1><p>在条件边沿触发时发送告警和恢复通知。</p></div><Button onClick={onCreate}><Plus size={16} />添加渠道</Button></div><Card className="section-card"><div className="section-header"><div><h2>已配置渠道</h2><p>Webhook 和 SMTP 渠道可被多个监控项复用。</p></div><Button variant="outline" size="icon" title="刷新" onClick={onRefresh}><RefreshCw size={15} /></Button></div>{channels.length ? <div className="channel-list">{channels.map((channel) => <div className="channel-row" key={channel.id}><div className="channel-icon">{channel.notifier_type === "smtp" ? <Inbox size={17} /> : <ExternalLink size={17} />}</div><div className="channel-main"><strong>{channel.name}</strong><span>{channel.notifier_type.toUpperCase()} · {channel.enabled ? "已启用" : "已停用"}</span></div><Badge tone={channel.enabled ? "success" : "muted"}>{channel.enabled ? "运行中" : "已停用"}</Badge><Button variant="outline" onClick={async () => { try { await api(`/api/v1/notification-channels/${channel.id}/test`, { method: "POST" }); notify("测试通知已发送"); } catch (error) { notify(error.message, "error"); } }}><Bell size={14} />测试</Button></div>)}</div> : <EmptyState icon={Bell} title="还没有通知渠道" description="添加 Webhook 或 SMTP，让变化及时抵达。" action={<Button onClick={onCreate}><Plus size={16} />添加渠道</Button>} />}</Card></div>; }

function SettingsPage() { return <div className="page-stack"><div className="page-heading"><div><div className="eyebrow">RUNTIME CONFIGURATION</div><h1>系统设置</h1><p>服务配置通过 config.yaml、环境变量和命令行管理。</p></div></div><div className="settings-grid"><Card className="settings-card"><div className="settings-icon"><Server size={18} /></div><div><h2>单二进制服务</h2><p>Web 资源已嵌入 Go 服务，默认监听 127.0.0.1:8080。</p><code>MEERKIT_SERVER__PORT</code></div></Card><Card className="settings-card"><div className="settings-icon"><Database size={18} /></div><div><h2>SQLite 存储</h2><p>监控配置、执行结果和通知渠道使用轻量化本地数据库保存。</p><code>MEERKIT_STORAGE__DATA_DIR</code></div></Card></div></div>; }

function MonitorDialog({ monitor, modules, channels, defaultTimezone, onClose, onSaved, onError }) {
  const [moduleType, setModuleType] = useState(monitor?.module_type || modules[0]?.type || "http");
  const descriptor = modules.find((item) => item.type === moduleType) || modules[0];
  const [name, setName] = useState(monitor?.name || "");
  const [schedule, setSchedule] = useState(monitor?.schedule || "*/5 * * * *");
  const [timezone, setTimezone] = useState(monitor?.timezone || defaultTimezone);
  const [enabled, setEnabled] = useState(monitor?.enabled ?? true);
  const [config, setConfig] = useState(() => monitor?.module_config || (moduleType === "http" ? { method: "GET", response_mode: "auto", normalize: "trim", verify_tls: true, timeout_seconds: 30 } : { timeout_seconds: 10, read_response: false, read_timeout_seconds: 3 }));
  const [conditionConfig, setConditionConfig] = useState(() => monitor?.condition_config || { logic: "ALL", rules: [] });
  const [channelIDs, setChannelIDs] = useState(monitor?.notification_channel_ids || []);
  const [saving, setSaving] = useState(false);

  useEffect(() => { if (!monitor) setConfig(moduleType === "http" ? { method: "GET", response_mode: "auto", normalize: "trim", verify_tls: true, timeout_seconds: 30 } : { timeout_seconds: 10, read_response: false, read_timeout_seconds: 3 }); }, [moduleType, monitor]);
  if (!descriptor) return null;
  const properties = descriptor.config_schema?.properties || {};
  const updateConfig = (key, value) => setConfig((current) => ({ ...current, [key]: value }));
  const addRule = () => setConditionConfig((current) => ({ ...current, rules: [...(current.rules || []), { field: descriptor.fields?.[0]?.name || "success", operator: descriptor.fields?.[0]?.operators?.[0] || "equals", value: "" }] }));
  const updateRule = (index, patch) => setConditionConfig((current) => ({ ...current, rules: current.rules.map((rule, itemIndex) => itemIndex === index ? { ...rule, ...patch } : rule) }));
  const removeRule = (index) => setConditionConfig((current) => ({ ...current, rules: current.rules.filter((_, itemIndex) => itemIndex !== index) }));
  const submit = async (event) => { event.preventDefault(); setSaving(true); try { const payload = { name, module_type: moduleType, schedule, timezone, enabled, module_config: config, condition_config: conditionConfig, notification_channel_ids: channelIDs }; await api(monitor ? `/api/v1/monitors/${monitor.id}` : "/api/v1/monitors", { method: monitor ? "PATCH" : "POST", body: JSON.stringify(payload) }); onSaved(); } catch (error) { onError(error.message); } finally { setSaving(false); } };
  return <div className="modal-backdrop" onMouseDown={(event) => event.target === event.currentTarget && onClose()}><div className="modal modal-wide"><div className="modal-header"><div><span className="eyebrow">{monitor ? "EDIT MONITOR" : "NEW MONITOR"}</span><h2>{monitor ? "编辑监控项" : "创建监控项"}</h2></div><Button variant="ghost" size="icon" onClick={onClose}><X size={17} /></Button></div><form onSubmit={submit}><div className="modal-body"><div className="form-grid"><label className="field"><span>名称</span><input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：生产 API 响应" /></label><label className="field"><span>采集模块</span><select value={moduleType} onChange={(event) => setModuleType(event.target.value)}>{modules.map((item) => <option key={item.type} value={item.type}>{item.name}</option>)}</select></label></div><div className="form-section"><div className="form-section-title"><div><h3>执行计划</h3><p>使用 cron 表达式，支持 5 段或 6 段格式。</p></div><Clock3 size={17} /></div><div className="form-grid"><label className="field"><span>Cron 表达式</span><input required value={schedule} onChange={(event) => setSchedule(event.target.value)} placeholder="*/5 * * * *" /><small>例如每 5 分钟执行一次</small></label><label className="field"><span>时区</span><input value={timezone} onChange={(event) => setTimezone(event.target.value)} placeholder="Asia/Shanghai" /></label></div></div><div className="form-section"><div className="form-section-title"><div><h3>{descriptor.name} 参数</h3><p>由采集模块声明并动态生成。</p></div><Radio size={17} /></div><DynamicFields properties={properties} values={config} onChange={updateConfig} /></div><div className="form-section"><div className="form-section-title"><div><h3>触发条件</h3><p>第一次执行只建立变化检测基线。</p></div><Button type="button" variant="outline" onClick={addRule}><Plus size={14} />添加条件</Button></div><div className="condition-toolbar"><span>满足</span><select value={conditionConfig.logic || "ALL"} onChange={(event) => setConditionConfig((current) => ({ ...current, logic: event.target.value }))}><option value="ALL">全部条件</option><option value="ANY">任一条件</option></select><span>时触发通知</span></div>{conditionConfig.rules?.length ? <div className="condition-list">{conditionConfig.rules.map((rule, index) => <div className="condition-row" key={`${rule.field}-${index}`}><select value={rule.field} onChange={(event) => updateRule(index, { field: event.target.value, operator: descriptor.fields.find((field) => field.name === event.target.value)?.operators?.[0] || "equals" })}>{descriptor.fields?.map((field) => <option key={field.name} value={field.name}>{field.label}</option>)}</select>{descriptor.fields?.find((field) => field.name === rule.field)?.path && <input value={rule.path || ""} onChange={(event) => updateRule(index, { path: event.target.value })} placeholder="JSON 路径，如 data.status" />}<select value={rule.operator} onChange={(event) => updateRule(index, { operator: event.target.value })}>{(descriptor.fields?.find((field) => field.name === rule.field)?.operators || []).map((operator) => <option key={operator} value={operator}>{operators[operator] || operator}</option>)}</select>{rule.operator !== "changed" && !["is_true", "is_false"].includes(rule.operator) && <input value={rule.value ?? ""} onChange={(event) => updateRule(index, { value: event.target.value })} placeholder="比较值" />}<Button type="button" variant="ghost" size="icon" onClick={() => removeRule(index)}><Trash2 size={15} /></Button></div>)}</div> : <div className="condition-empty">未设置条件，仅保存采集结果。</div>}</div><div className="form-section"><div className="form-section-title"><div><h3>通知渠道</h3><p>可选择多个渠道，触发和恢复时异步发送。</p></div><Bell size={17} /></div><div className="channel-checks">{channels.length ? channels.map((channel) => <label key={channel.id} className="check-card"><input type="checkbox" checked={channelIDs.includes(channel.id)} onChange={(event) => setChannelIDs((current) => event.target.checked ? [...current, channel.id] : current.filter((id) => id !== channel.id))} /><span><strong>{channel.name}</strong><small>{channel.notifier_type.toUpperCase()}</small></span></label>) : <span className="muted-text">暂无通知渠道，可稍后在通知渠道页面添加。</span>}</div></div><label className="switch-row"><input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} /><span className="switch-ui" /><span><strong>启用监控</strong><small>保存后按下一个 cron 时间执行</small></span></label></div><div className="modal-footer"><Button type="button" variant="ghost" onClick={onClose}>取消</Button><Button type="submit" disabled={saving}>{saving ? "保存中..." : <><Check size={15} />保存监控</>}</Button></div></form></div></div>;
}

function DynamicFields({ properties, values, onChange }) { const entries = Object.entries(properties).filter(([key]) => !["headers"].includes(key)); return <div className="dynamic-fields">{entries.map(([key, schema]) => { const type = schema.type || "string"; const value = values[key] ?? schema.default ?? (type === "boolean" ? false : ""); if (type === "boolean") return <label className="switch-row compact" key={key}><input type="checkbox" checked={Boolean(value)} onChange={(event) => onChange(key, event.target.checked)} /><span className="switch-ui" /><span><strong>{labelFor(key, schema)}</strong></span></label>; if (schema.enum) return <label className="field" key={key}><span>{labelFor(key, schema)}</span><select value={value} onChange={(event) => onChange(key, event.target.value)}>{schema.enum.map((option) => <option key={option}>{option}</option>)}</select></label>; return <label className="field" key={key}><span>{labelFor(key, schema)}</span><input type={schema.secret ? "password" : type === "integer" || type === "number" ? "number" : "text"} value={value} onChange={(event) => onChange(key, type === "integer" || type === "number" ? Number(event.target.value) : event.target.value)} placeholder={placeholderFor(key)} /></label>; })}</div>; }

function ChannelDialog({ notifiers, onClose, onSaved, onError }) { const [type, setType] = useState(notifiers[0]?.type || "webhook"); const descriptor = notifiers.find((item) => item.type === type) || notifiers[0]; const [name, setName] = useState(""); const [config, setConfig] = useState({}); const [saving, setSaving] = useState(false); if (!descriptor) return null; const properties = descriptor.config_schema?.properties || {}; const submit = async (event) => { event.preventDefault(); setSaving(true); try { await api("/api/v1/notification-channels", { method: "POST", body: JSON.stringify({ name, notifier_type: type, enabled: true, config }) }); onSaved(); } catch (error) { onError(error.message); } finally { setSaving(false); } }; return <div className="modal-backdrop" onMouseDown={(event) => event.target === event.currentTarget && onClose()}><div className="modal"><div className="modal-header"><div><span className="eyebrow">DELIVERY CHANNEL</span><h2>添加通知渠道</h2></div><Button variant="ghost" size="icon" onClick={onClose}><X size={17} /></Button></div><form onSubmit={submit}><div className="modal-body"><div className="form-grid"><label className="field"><span>名称</span><input required value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：团队 Webhook" /></label><label className="field"><span>类型</span><select value={type} onChange={(event) => { setType(event.target.value); setConfig({}); }}>{notifiers.map((item) => <option key={item.type} value={item.type}>{item.name}</option>)}</select></label></div><div className="form-section"><div className="form-section-title"><div><h3>{descriptor.name} 配置</h3><p>{descriptor.description}</p></div><Bell size={17} /></div><DynamicFields properties={properties} values={config} onChange={(key, value) => setConfig((current) => ({ ...current, [key]: value }))} /></div></div><div className="modal-footer"><Button type="button" variant="ghost" onClick={onClose}>取消</Button><Button type="submit" disabled={saving}>{saving ? "保存中..." : <><Check size={15} />保存渠道</>}</Button></div></form></div></div>; }

function labelFor(key, schema) { return schema.title || ({ url: "URL", method: "请求方法", headers: "请求头", body: "请求体", host: "主机", port: "端口", from: "发件人", to: "收件人", username: "用户名", password: "密码", token: "Token", timeout_seconds: "超时(秒)", response_mode: "响应模式", normalize: "内容规范化", verify_tls: "校验证书", read_response: "读取响应", read_timeout_seconds: "读取超时(秒)", max_read_bytes: "最大读取字节数", max_body_bytes: "最大正文字节数", subject_prefix: "主题前缀" }[key] || key); }
function placeholderFor(key) { return { url: "https://example.com/api", host: "127.0.0.1", port: "8080", headers: "JSON 请求头" }[key] || ""; }
function pageTitle(page) { return { overview: "总览", monitors: "监控项", notifications: "通知渠道", settings: "系统设置" }[page]; }
function formatDate(value) { if (!value) return "尚未执行"; return new Date(value).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }); }

createRoot(document.getElementById("root")).render(<App />);
