import React, { useCallback, useEffect, useMemo, useState } from "react";
import { ArrowDown, ArrowUp, Camera, Check, Clipboard, Code2, Eraser, FileCode2, Globe2, GripVertical, Group, Keyboard, ListChecks, LoaderCircle, MousePointerClick, Network, PanelTopOpen, Play, Plus, RefreshCw, Search, Settings2, SquareTerminal, Timer, Trash2, Workflow, X } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { IconButton } from "../components/ui/IconButton";
import { Input } from "../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/Select";
import { Switch } from "../components/ui/Switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/Tabs";
import { api } from "../lib/api";

const actionIcons = {
  "panel-top-open": PanelTopOpen,
  globe: Globe2,
  group: Group,
  "trash-2": Trash2,
  timer: Timer,
  camera: Camera,
  "file-code-2": FileCode2,
  search: Search,
  "mouse-pointer-click": MousePointerClick,
  keyboard: Keyboard,
  "code-2": Code2,
  network: Network
};

let actionSequence = 0;

export function BrowserDebugPage() {
  const [status, setStatus] = useState(null);
  const [catalog, setCatalog] = useState(null);
  const [loadError, setLoadError] = useState("");
  const [runError, setRunError] = useState("");
  const [agentID, setAgentID] = useState("");
  const [tabID, setTabID] = useState("");
  const [windowID, setWindowID] = useState("");
  const [timeoutMS, setTimeoutMS] = useState(60000);
  const [keepTab, setKeepTab] = useState(true);
  const [workflow, setWorkflow] = useState([]);
  const [selectedID, setSelectedID] = useState("");
  const [dropIndex, setDropIndex] = useState(-1);
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState(null);
  const [resultTab, setResultTab] = useState("actions");
  const [copied, setCopied] = useState(false);

  const loadStatus = useCallback(async () => {
    try {
      const value = await api("/api/v1/browser");
      const agents = value?.connected_agents || [];
      setStatus(value);
      setLoadError("");
      setAgentID((current) => agents.some((agent) => agent.id === current) ? current : agents[0]?.id || "");
    } catch (error) {
      setLoadError(error.message);
    }
  }, []);

  useEffect(() => {
    let active = true;
    Promise.all([api("/api/v1/browser"), api("/api/v1/browser/actions")])
      .then(([statusValue, catalogValue]) => {
        if (!active) return;
        const agents = statusValue?.connected_agents || [];
        const definitions = catalogValue?.actions || [];
        const starter = (catalogValue?.starter_flow || []).map((action) => hydrateAction(action, definitions));
        setStatus(statusValue);
        setCatalog(catalogValue);
        setAgentID(agents[0]?.id || "");
        setWorkflow(starter);
        setSelectedID(starter[0]?.id || "");
        setLoadError("");
      })
      .catch((error) => { if (active) setLoadError(error.message); });
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void loadStatus();
    }, 5000);
    return () => { active = false; window.clearInterval(timer); };
  }, [loadStatus]);

  const definitions = catalog?.actions || [];
  const definitionMap = useMemo(() => new Map(definitions.map((definition) => [definition.type, definition])), [definitions]);
  const categories = useMemo(() => definitions.reduce((items, definition) => {
    let category = items.find((item) => item.key === definition.category);
    if (!category) {
      category = { key: definition.category, label: definition.category_label, actions: [] };
      items.push(category);
    }
    category.actions.push(definition);
    return items;
  }, []), [definitions]);
  const agents = status?.connected_agents || [];
  const selectedAgent = agents.find((agent) => agent.id === agentID);
  const capabilities = useMemo(() => new Set(selectedAgent?.capabilities || []), [selectedAgent]);
  const missingCapabilities = useMemo(() => [...new Set(workflow.map((action) => definitionMap.get(action.type)?.capability || action.type).filter((capability) => selectedAgent && !capabilities.has(capability)))], [capabilities, definitionMap, selectedAgent, workflow]);
  const selectedIndex = workflow.findIndex((action) => action.id === selectedID);
  const selectedAction = selectedIndex >= 0 ? workflow[selectedIndex] : null;
  const selectedDefinition = selectedAction ? definitionMap.get(selectedAction.type) : null;

  const addAction = (definition, index = workflow.length) => {
    if (!definition) return;
    const action = createAction(definition);
    setWorkflow((current) => [...current.slice(0, index), action, ...current.slice(index)]);
    setSelectedID(action.id);
  };

  const updateAction = (id, updater) => setWorkflow((current) => current.map((action) => action.id === id ? updater(action) : action));

  const removeAction = (index) => {
    setWorkflow((current) => {
      const next = current.filter((_, itemIndex) => itemIndex !== index);
      if (current[index]?.id === selectedID) setSelectedID(next[Math.min(index, next.length - 1)]?.id || "");
      return next;
    });
  };

  const moveAction = (from, to) => {
    if (from === to || from < 0 || to < 0 || from >= workflow.length || to >= workflow.length) return;
    setWorkflow((current) => {
      const next = [...current];
      const [action] = next.splice(from, 1);
      next.splice(to, 0, action);
      return next;
    });
  };

  const handleDrop = (event, index) => {
    event.preventDefault();
    setDropIndex(-1);
    const actionType = event.dataTransfer.getData("application/x-meerkit-action");
    const sourceValue = event.dataTransfer.getData("application/x-meerkit-workflow-index");
    const source = sourceValue === "" ? -1 : Number(sourceValue);
    if (actionType) addAction(definitionMap.get(actionType), index);
    else if (Number.isInteger(source)) moveAction(source, source < index ? index - 1 : index);
  };

  const execute = async () => {
    setRunError("");
    setRunning(true);
    try {
      const request = {
        agent_id: agentID,
        ...(positiveNumber(tabID) ? { tab_id: positiveNumber(tabID) } : {}),
        ...(positiveNumber(windowID) ? { window_id: positiveNumber(windowID) } : {}),
        timeout_ms: Number(timeoutMS) || 60000,
        keep_tab: keepTab,
        actions: workflow
      };
      const value = await api("/api/v1/browser/run", { method: "POST", body: JSON.stringify(request) });
      setResult(value);
      setResultTab(value?.network?.length ? "network" : "actions");
    } catch (error) {
      setRunError(error.message);
    } finally {
      setRunning(false);
      void loadStatus();
    }
  };

  const copyResult = async () => {
    if (!result) return;
    try {
      await navigator.clipboard.writeText(JSON.stringify(result, null, 2));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      setRunError("浏览器未允许复制调试结果");
    }
  };

  const canRun = Boolean(selectedAgent && workflow.length && !running && missingCapabilities.length === 0);
  return <div className="page-stack browser-debug-page">
    <PageHeader eyebrow="BROWSER TOOLS" title="浏览器操作调试" description="" />
    {(loadError || runError) && <div className="browser-debug-error" role="alert"><X size={15} /><span>{runError || loadError}</span></div>}
    <div className="browser-debug-workbench">
      <Card className="browser-debug-builder">
        <div className="browser-debug-panel-header"><div><Workflow size={16} /><strong>操作流程</strong><Badge tone={agents.length ? "success" : "muted"}><span className="status-dot" />{agents.length ? `${agents.length} 个节点在线` : "无可用节点"}</Badge></div><div className="browser-debug-heading-actions"><Badge tone={missingCapabilities.length ? "warning" : "muted"}>{workflow.length} 个 action</Badge><IconButton variant="ghost" size="sm" title="刷新执行节点" aria-label="刷新执行节点" onClick={() => void loadStatus()}><RefreshCw size={14} /></IconButton></div></div>
        <div className="browser-flow-context">
          <FlowField label="执行节点"><Select value={agentID || undefined} onValueChange={setAgentID}><SelectTrigger><SelectValue placeholder="选择在线节点" /></SelectTrigger><SelectContent>{agents.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name || agent.id}</SelectItem>)}</SelectContent></Select></FlowField>
          <FlowField label="Tab ID"><Input type="number" min="1" value={tabID} onChange={(event) => setTabID(event.target.value)} placeholder="自动" /></FlowField>
          <FlowField label="Window ID"><Input type="number" min="1" value={windowID} onChange={(event) => setWindowID(event.target.value)} placeholder="自动" /></FlowField>
          <FlowField label="超时 (ms)"><Input type="number" min="1000" max="300000" step="1000" value={timeoutMS} onChange={(event) => setTimeoutMS(event.target.value)} /></FlowField>
          <label className="browser-flow-context-switch"><span>保留标签页</span><Switch checked={keepTab} onCheckedChange={setKeepTab} aria-label="执行后保留标签页" /></label>
        </div>
        <div className="browser-flow-composer">
          <aside className="browser-action-palette">
            {categories.map((category) => <section key={category.key}><h3>{category.label}</h3>{category.actions.map((definition) => <PaletteAction key={definition.type} definition={definition} disabled={capabilities.size > 0 && !capabilities.has(definition.capability)} onAdd={() => addAction(definition)} />)}</section>)}
          </aside>
          <div className="browser-flow-canvas">
            <div className="browser-flow-list" onDragOver={(event) => event.preventDefault()} onDrop={(event) => handleDrop(event, workflow.length)}>
              {workflow.length ? <>{workflow.map((action, index) => <WorkflowAction key={action.id} action={action} definition={definitionMap.get(action.type)} index={index} total={workflow.length} selected={action.id === selectedID} dropActive={dropIndex === index} onDropTarget={setDropIndex} onSelect={() => setSelectedID(action.id)} onMove={moveAction} onRemove={removeAction} onDrop={handleDrop} />)}<div className="browser-flow-drop-end" data-active={dropIndex === workflow.length} onDragEnter={(event) => { event.preventDefault(); setDropIndex(workflow.length); }} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.stopPropagation(); handleDrop(event, workflow.length); }} /></> : <div className="browser-flow-empty"><Workflow size={20} /><span>暂无操作</span></div>}
            </div>
            <ActionEditor action={selectedAction} definition={selectedDefinition} onChange={(updater) => updateAction(selectedAction.id, updater)} />
          </div>
        </div>
        <div className="browser-debug-runbar"><div>{missingCapabilities.length ? <><X size={14} /><span>节点缺少：{missingCapabilities.join("、")}</span></> : <><ListChecks size={14} /><span>{workflow.length ? `按顺序执行 ${workflow.length} 个 action` : "流程为空"}</span></>}</div><Button disabled={!canRun} onClick={() => void execute()}>{running ? <LoaderCircle className="spin" size={15} /> : <Play size={15} />}{running ? "执行中" : "执行流程"}</Button></div>
      </Card>

      <Card className="browser-debug-output">
        <div className="browser-debug-panel-header"><div><SquareTerminal size={16} /><strong>执行结果</strong></div><div className="browser-debug-output-actions">{result && <Badge tone={result.actions?.every((action) => action.success) ? "success" : "warning"}>{result.actions?.every((action) => action.success) ? "完成" : "部分失败"}</Badge>}<IconButton title={copied ? "已复制" : "复制结果"} aria-label={copied ? "已复制" : "复制结果"} disabled={!result} onClick={() => void copyResult()}>{copied ? <Check size={14} /> : <Clipboard size={14} />}</IconButton><IconButton title="清空结果" aria-label="清空结果" disabled={!result && !runError} onClick={() => { setResult(null); setRunError(""); }}><Eraser size={14} /></IconButton></div></div>
        {!result ? <div className="browser-debug-empty"><SquareTerminal size={22} /><span>{running ? "等待浏览器节点返回结果..." : "尚未执行调试流程"}</span></div> : <>
          <div className="browser-debug-result-meta"><span><small>节点</small><strong>{result.agent_id || "-"}</strong></span><span><small>标签页</small><strong>{result.tab_id || "已关闭"}</strong></span><span><small>窗口</small><strong>{result.window_id || "-"}</strong></span><span><small>耗时</small><strong>{result.duration_ms || 0} ms</strong></span></div>
          <Tabs value={resultTab} onValueChange={setResultTab} className="browser-debug-result-tabs"><TabsList><TabsTrigger value="actions">动作 {result.actions?.length || 0}</TabsTrigger><TabsTrigger value="network">网络 {result.network?.length || 0}</TabsTrigger><TabsTrigger value="json">JSON</TabsTrigger></TabsList><TabsContent value="actions"><ActionResults actions={result.actions || []} definitionMap={definitionMap} /></TabsContent><TabsContent value="network"><NetworkResults items={result.network || []} /></TabsContent><TabsContent value="json"><pre className="browser-debug-json">{JSON.stringify(result, null, 2)}</pre></TabsContent></Tabs>
        </>}
      </Card>
    </div>
  </div>;
}

function PaletteAction({ definition, disabled, onAdd }) {
  const Icon = actionIcons[definition.icon] || Settings2;
  return <button type="button" draggable={!disabled} disabled={disabled} className="browser-palette-action" title={definition.label} onDragStart={(event) => { event.dataTransfer.effectAllowed = "copy"; event.dataTransfer.setData("application/x-meerkit-action", definition.type); }} onClick={onAdd}><Icon size={15} /><span>{definition.label}</span><Plus size={12} /></button>;
}

function WorkflowAction({ action, definition, index, total, selected, dropActive, onDropTarget, onSelect, onMove, onRemove, onDrop }) {
  const Icon = actionIcons[definition?.icon] || Settings2;
  return <div className="browser-flow-drop-target" data-active={dropActive} onDragEnter={(event) => { event.preventDefault(); event.stopPropagation(); onDropTarget(index); }} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.stopPropagation(); onDrop(event, index); }}>
    <article className="browser-flow-action" data-selected={selected} data-destructive={definition?.destructive} draggable onDragStart={(event) => { event.dataTransfer.effectAllowed = "move"; event.dataTransfer.setData("application/x-meerkit-workflow-index", String(index)); }} onClick={onSelect}>
      <GripVertical className="browser-flow-grip" size={15} />
      <span className="browser-flow-index">{index + 1}</span>
      <span className="browser-flow-action-icon"><Icon size={15} /></span>
      <span className="browser-flow-action-copy"><strong>{definition?.label || action.type}</strong><small>{actionSummary(action, definition)}</small></span>
      <span className="browser-flow-action-controls"><IconButton title="上移" aria-label={`上移 ${definition?.label || action.type}`} disabled={index === 0} onClick={(event) => { event.stopPropagation(); onMove(index, index - 1); }}><ArrowUp size={13} /></IconButton><IconButton title="下移" aria-label={`下移 ${definition?.label || action.type}`} disabled={index === total - 1} onClick={(event) => { event.stopPropagation(); onMove(index, index + 1); }}><ArrowDown size={13} /></IconButton><IconButton className="is-destructive" title="删除" aria-label={`删除 ${definition?.label || action.type}`} onClick={(event) => { event.stopPropagation(); onRemove(index); }}><Trash2 size={13} /></IconButton></span>
    </article>
  </div>;
}

function ActionEditor({ action, definition, onChange }) {
  if (!action || !definition) return <div className="browser-action-editor is-empty"><Settings2 size={16} /><span>选择一个 action</span></div>;
  const updateParam = (key, value) => onChange((current) => ({ ...current, params: { ...(current.params || {}), [key]: value } }));
  return <section className="browser-action-editor">
    <div className="browser-action-editor-header"><div><strong>{definition.label}</strong><code>{definition.type}</code></div><label><span>失败后继续</span><Switch checked={Boolean(action.continue_on_error)} onCheckedChange={(checked) => onChange((current) => ({ ...current, continue_on_error: Boolean(checked) }))} aria-label="失败后继续执行" /></label></div>
    <div className="browser-action-fields">{(definition.parameters || []).filter((parameter) => parameterVisible(parameter, action.params || {})).map((parameter) => <ActionParameter key={parameter.key} parameter={parameter} value={action.params?.[parameter.key]} onChange={(value) => updateParam(parameter.key, value)} />)}</div>
  </section>;
}

function ActionParameter({ parameter, value, onChange }) {
  const className = `browser-action-field${parameter.wide ? " is-wide" : ""}`;
  if (parameter.type === "boolean") return <label className={`${className} is-switch`}><span>{parameter.label}</span><Switch checked={Boolean(value)} onCheckedChange={onChange} aria-label={parameter.label} /></label>;
  if (parameter.type === "select") {
    const selected = value === "" || value == null ? "__empty__" : String(value);
    return <label className={className}><span>{parameter.label}</span><Select value={selected} onValueChange={(next) => onChange(next === "__empty__" ? "" : next)}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{(parameter.options || []).map((option) => <SelectItem key={option.value || "__empty__"} value={option.value || "__empty__"}>{option.label}</SelectItem>)}</SelectContent></Select></label>;
  }
  if (parameter.type === "textarea" || parameter.type === "code") return <label className={className}><span>{parameter.label}</span><textarea className={`browser-debug-textarea${parameter.type === "code" ? " is-code" : ""}`} value={value ?? ""} placeholder={parameter.placeholder} onChange={(event) => onChange(event.target.value)} /></label>;
  return <label className={className}><span>{parameter.label}</span><Input type={parameter.type === "number" ? "number" : parameter.type === "url" ? "text" : "text"} min={parameter.min} max={parameter.max} step={parameter.step} value={value ?? ""} placeholder={parameter.placeholder} onChange={(event) => onChange(parameter.type === "number" ? (event.target.value === "" ? "" : Number(event.target.value)) : event.target.value)} /></label>;
}

function FlowField({ label, children }) {
  return <label className="browser-flow-context-field"><span>{label}</span>{children}</label>;
}

function ActionResults({ actions, definitionMap }) {
  if (!actions.length) return <div className="browser-debug-tab-empty">没有 action 结果</div>;
  return <div className="browser-debug-action-results">{actions.map((action, index) => {
    const definition = definitionMap.get(action.type);
    return <section key={`${action.id}-${index}`} className="browser-debug-action-result" data-success={action.success}><div><span className="browser-debug-result-icon">{action.success ? <Check size={13} /> : <X size={13} />}</span><strong>{definition?.label || action.id || `action-${index + 1}`}</strong><code>{action.type}</code><small>{action.duration_ms || 0} ms</small></div>{action.error ? <pre className="is-error">{action.error}</pre> : <ActionResultData type={definition?.result_type} data={action.data || {}} />}</section>;
  })}</div>;
}

function ActionResultData({ type, data }) {
  if (!Object.keys(data).length) return null;
  if (type === "screenshot" && data.data_url) return <div className="browser-debug-screenshot"><img src={data.data_url} alt="浏览器调试截图" /></div>;
  if (type === "element") return <div className="browser-action-element-result"><div><Badge tone="muted">{data.tag_name || "element"}</Badge><strong>{data.selector || "-"}</strong></div>{data.text && <pre>{data.text}</pre>}<DetailRows entries={Object.entries(data.attributes || {})} empty="无元素属性" /></div>;
  if (type === "document" && data.html != null) return <pre>{data.html}</pre>;
  if (type === "script") return <pre>{JSON.stringify(data.value, null, 2)}</pre>;
  if (type === "tab") return <DetailRows entries={[["Tab ID", data.tab_id], ["Window ID", data.window_id], ["地址", data.url], ["标题", data.title], ["状态", data.status], ["复用", data.reused == null ? "-" : data.reused ? "是" : "否"]]} />;
  return <pre>{JSON.stringify(withoutImageData(data), null, 2)}</pre>;
}

function NetworkResults({ items }) {
  const [filter, setFilter] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [detailTab, setDetailTab] = useState("headers");
  if (!items.length) return <div className="browser-debug-tab-empty"><Network size={18} />未捕获到响应</div>;
  const indexed = items.map((item, index) => ({ item, index }));
  const query = filter.trim().toLowerCase();
  const filtered = query ? indexed.filter(({ item }) => [item.url, item.method, item.status, item.resource_type, item.mime_type, item.protocol].some((value) => String(value || "").toLowerCase().includes(query))) : indexed;
  const selectedEntry = filtered.find((entry) => entry.index === selectedIndex) || filtered[0];
  const selected = selectedEntry?.item;
  const longestDuration = Math.max(1, ...filtered.map(({ item }) => Number(item.duration_ms) || 0));
  return <div className="browser-network-workspace">
    <div className="browser-network-list-pane">
      <div className="browser-network-toolbar"><div><Search size={13} /><Input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder="筛选请求" /></div><span>{filtered.length} / {items.length}</span></div>
      <div className="browser-network-table"><div className="browser-network-head"><span>状态</span><span>名称</span><span>方法</span><span>类型</span><span>大小</span><span>耗时</span></div><div className="browser-network-rows">{filtered.length ? filtered.map(({ item, index }) => <button type="button" key={`${item.capture_id}-${item.url}-${index}`} className="browser-network-row" data-selected={selectedEntry?.index === index} data-error={item.status >= 400 || Boolean(item.error)} onClick={() => setSelectedIndex(index)}><span>{item.status || "ERR"}</span><span><strong title={item.url}>{requestName(item.url)}</strong><small>{requestHost(item.url)}</small></span><span>{item.method || "GET"}</span><span>{item.resource_type || item.mime_type || "-"}</span><span>{formatBytes(item.encoded_data_length || bodyLength(item))}</span><span className="browser-network-waterfall"><i style={{ width: `${Math.max(4, Math.round(((Number(item.duration_ms) || 0) / longestDuration) * 100))}%` }} /><em>{item.duration_ms || 0} ms</em></span></button>) : <div className="browser-network-no-match">没有匹配的请求</div>}</div></div>
    </div>
    <div className="browser-network-detail-pane">{selected ? <><div className="browser-network-detail-title"><div><Badge tone={selected.status >= 400 || selected.error ? "warning" : "success"}>{selected.status || "ERR"}</Badge><strong title={selected.url}>{selected.method || "GET"} {selected.url}</strong></div><span>{selected.protocol || "-"} · {formatBytes(selected.encoded_data_length || bodyLength(selected))} · {selected.duration_ms || 0} ms</span></div><Tabs value={detailTab} onValueChange={setDetailTab} className="browser-network-detail-tabs"><TabsList><TabsTrigger value="headers">标头</TabsTrigger><TabsTrigger value="payload">载荷</TabsTrigger><TabsTrigger value="response">响应</TabsTrigger><TabsTrigger value="timing">时序</TabsTrigger></TabsList><TabsContent value="headers"><NetworkHeaders item={selected} /></TabsContent><TabsContent value="payload"><NetworkPayload item={selected} /></TabsContent><TabsContent value="response"><NetworkResponse item={selected} /></TabsContent><TabsContent value="timing"><NetworkTiming item={selected} /></TabsContent></Tabs> </> : <div className="browser-debug-tab-empty">请选择一个请求</div>}</div>
  </div>;
}

function NetworkHeaders({ item }) {
  return <div className="browser-network-detail-scroll"><DetailSection title="常规"><DetailRows entries={[["请求 URL", item.url], ["请求方法", item.method || "GET"], ["状态代码", `${item.status || 0} ${item.status_text || ""}`.trim()], ["远程地址", item.remote_ip_address ? `${item.remote_ip_address}${item.remote_port ? `:${item.remote_port}` : ""}` : "-"], ["协议", item.protocol || "-"], ["资源类型", item.resource_type || "-"], ["发起类型", item.initiator_type || "-"], ["缓存", item.from_service_worker ? "Service Worker" : item.from_disk_cache ? "磁盘缓存" : "否"]]} /></DetailSection><DetailSection title={`响应标头 (${Object.keys(item.headers || {}).length})`}><DetailRows entries={Object.entries(item.headers || {})} empty="无响应标头" /></DetailSection><DetailSection title={`请求标头 (${Object.keys(item.request_headers || {}).length})`}><DetailRows entries={Object.entries(item.request_headers || {})} empty="无请求标头" /></DetailSection></div>;
}

function NetworkPayload({ item }) {
  if (!item.request_body) return <div className="browser-network-detail-empty">此请求没有载荷</div>;
  return <div className="browser-network-body"><pre>{formatBody(item.request_body, item.request_headers?.["Content-Type"] || item.request_headers?.["content-type"])}</pre>{item.request_body_truncated && <span>请求载荷已截断</span>}</div>;
}

function NetworkResponse({ item }) {
  if (item.error) return <div className="browser-network-body"><pre className="is-error">{item.error}</pre></div>;
  if (item.body == null || item.body === "") return <div className="browser-network-detail-empty">响应正文为空</div>;
  return <div className="browser-network-body"><pre>{item.body_base64 ? `[Base64]\n${item.body}` : formatBody(item.body, item.mime_type)}</pre>{item.truncated && <span>响应正文已截断</span>}</div>;
}

function NetworkTiming({ item }) {
  const timing = Object.entries(item.timing || {}).filter(([, value]) => Number(value) >= 0);
  return <div className="browser-network-detail-scroll"><DetailSection title="汇总"><DetailRows entries={[["总耗时", `${item.duration_ms || 0} ms`], ["传输大小", formatBytes(item.encoded_data_length || bodyLength(item))], ["响应正文", formatBytes(bodyLength(item))]]} /></DetailSection><DetailSection title="CDP Timing"><DetailRows entries={timing.map(([key, value]) => [key, key === "requestTime" ? `${value} s` : `${value} ms`])} empty="当前响应没有 Timing 数据" /></DetailSection></div>;
}

function DetailSection({ title, children }) { return <section className="browser-network-detail-section"><h4>{title}</h4>{children}</section>; }

function DetailRows({ entries, empty = "" }) {
  const values = entries.filter(([, value]) => value !== undefined && value !== null && value !== "");
  if (!values.length) return empty ? <div className="browser-network-detail-empty">{empty}</div> : null;
  return <dl className="browser-result-detail-rows">{values.map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{String(value)}</dd></div>)}</dl>;
}

function hydrateAction(action, definitions) {
  const definition = definitions.find((item) => item.type === action.type);
  return { ...createAction(definition, action.id), ...action, params: { ...defaultParams(definition), ...(action.params || {}) } };
}

function createAction(definition, preferredID = "") {
  actionSequence += 1;
  return { id: preferredID || `${definition?.type?.replaceAll(".", "-") || "action"}-${actionSequence}`, type: definition?.type || "", params: defaultParams(definition), continue_on_error: Boolean(definition?.default_continue_on_error) };
}

function defaultParams(definition) {
  return Object.fromEntries((definition?.parameters || []).filter((parameter) => Object.prototype.hasOwnProperty.call(parameter, "default")).map((parameter) => [parameter.key, parameter.default]));
}

function parameterVisible(parameter, params) {
  return !parameter.visible_when || Object.entries(parameter.visible_when).every(([key, value]) => params[key] === value);
}

function actionSummary(action, definition) {
  const parts = (definition?.parameters || []).filter((parameter) => parameterVisible(parameter, action.params || {}) && !["boolean", "textarea", "code"].includes(parameter.type)).map((parameter) => {
    const value = action.params?.[parameter.key];
    return value === "" || value == null ? "" : `${parameter.label}: ${value}`;
  }).filter(Boolean);
  return parts.slice(0, 2).join(" · ") || action.type;
}

function positiveNumber(value) { const number = Math.trunc(Number(value) || 0); return number > 0 ? number : 0; }
function requestName(value) { try { const url = new URL(value); return url.pathname.split("/").filter(Boolean).pop() || url.hostname; } catch { return value || "-"; } }
function requestHost(value) { try { return new URL(value).host; } catch { return ""; } }
function bodyLength(item) { return new TextEncoder().encode(String(item?.body || "")).length; }
function formatBytes(value) { const bytes = Number(value) || 0; if (bytes < 1024) return `${bytes} B`; if (bytes < 1048576) return `${(bytes / 1024).toFixed(bytes < 10240 ? 1 : 0)} KB`; return `${(bytes / 1048576).toFixed(1)} MB`; }
function formatBody(value, contentType = "") { if (!String(contentType).toLowerCase().includes("json")) return value; try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; } }
function withoutImageData(data) { if (!data?.data_url) return data; return { ...data, data_url: `[image data: ${data.data_url.length} characters]` }; }
