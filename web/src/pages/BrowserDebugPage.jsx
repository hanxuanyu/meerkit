import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Camera, Check, Clipboard, Code2, FileText, Globe2, Group, Image, Keyboard, LoaderCircle, MousePointerClick, Network, PanelTopOpen, Play, RefreshCw, Search, Settings2, SquareTerminal, Timer, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "../components/layout/PageHeader";
import { DynamicFields } from "../components/forms/DynamicFields";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { IconButton } from "../components/ui/IconButton";
import { Input } from "../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/Select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/Tabs";
import { api } from "../lib/api";
import { getDefaultValues, sanitizeValues } from "../lib/parameterSchema";

const icons = { "panel-top-open": PanelTopOpen, globe: Globe2, group: Group, "trash-2": Trash2, timer: Timer, camera: Camera, "file-code-2": Code2, search: Search, "mouse-pointer-click": MousePointerClick, keyboard: Keyboard, "code-2": Code2 };

export function BrowserDebugPage() {
  const [status, setStatus] = useState(null);
  const [catalog, setCatalog] = useState(null);
  const [targets, setTargets] = useState(null);
  const [agentID, setAgentID] = useState("");
  const [windowID, setWindowID] = useState("");
  const [tabID, setTabID] = useState("");
  const [actionType, setActionType] = useState("");
  const [params, setParams] = useState({});
  const [timeoutMS, setTimeoutMS] = useState(60000);
  const [result, setResult] = useState(null);
  const [lastRequest, setLastRequest] = useState(null);
  const [running, setRunning] = useState(false);
  const [capture, setCapture] = useState(null);
  const [network, setNetwork] = useState([]);
  const [networkFilter, setNetworkFilter] = useState("");
  const [captureRules, setCaptureRules] = useState({ url_contains: "", resource_type: "", max_body_bytes: 262144 });
  const [overrideTarget, setOverrideTarget] = useState(false);
  const [overrideWindowID, setOverrideWindowID] = useState("");
  const [overrideTabID, setOverrideTabID] = useState("");
  const [selectedNetwork, setSelectedNetwork] = useState(null);
  const agentRef = useRef(agentID);
  const captureIDRef = useRef("");

  const agents = status?.connected_agents || [];
  const definitions = catalog?.actions || [];
  const definition = definitions.find((item) => item.type === actionType) || definitions[0];
  const windows = targets?.windows || [];
  const selectedWindow = windows.find((item) => String(item.id) === String(windowID));
  const tabs = selectedWindow?.tabs || [];
  const selectedTab = tabs.find((item) => String(item.id) === String(tabID));
  const allTabs = useMemo(() => windows.flatMap((window) => (window.tabs || []).map((tab) => ({ ...tab, window }))), [windows]);
  const categories = useMemo(() => definitions.reduce((list, item) => { let category = list.find((entry) => entry.key === item.category); if (!category) { category = { key: item.category, label: item.category_label, actions: [] }; list.push(category); } category.actions.push(item); return list; }, []), [definitions]);

  const loadTargets = useCallback(async (id) => {
    if (!id) { setTargets(null); return; }
    try { const value = await api(`/api/v1/browser/targets?agent_id=${encodeURIComponent(id)}`); setTargets(value); } catch (err) { notifyBrowserError(err); }
  }, []);

  const loadStatus = useCallback(async () => {
    const value = await api("/api/v1/browser");
    setStatus(value);
    return value;
  }, []);

  useEffect(() => {
    let active = true;
    Promise.all([loadStatus(), api("/api/v1/browser/actions")]).then(([statusValue, catalogValue]) => {
      if (!active) return;
      setStatus(statusValue); setCatalog(catalogValue);
      const id = statusValue?.connected_agents?.[0]?.id || "";
      setAgentID(id); setActionType(catalogValue?.actions?.[0]?.type || "");
    }).catch((err) => { if (active) notifyBrowserError(err); });
    return () => { active = false; };
  }, [loadStatus]);

  useEffect(() => { void loadTargets(agentID); }, [agentID, loadTargets]);
  useEffect(() => {
    if (!agents.some((agent) => agent.id === agentID)) setAgentID(agents[0]?.id || "");
  }, [agentID, agents]);
  useEffect(() => { agentRef.current = agentID; }, [agentID]);
  useEffect(() => { captureIDRef.current = capture?.id || ""; }, [capture]);
  useEffect(() => {
    if (!windows.length) { setWindowID(""); setTabID(""); return; }
    if (!windows.some((item) => String(item.id) === String(windowID))) setWindowID(String(windows.find((item) => item.focused)?.id || windows[0].id));
  }, [targets, windowID, windows]);
  useEffect(() => {
    if (!selectedWindow) return;
    if (!tabs.some((item) => String(item.id) === String(tabID))) setTabID(tabs.length ? String(tabs.find((item) => item.active)?.id || tabs[0].id) : "");
  }, [selectedWindow, tabID, tabs]);
  useEffect(() => {
    if (!definition) return;
    setParams(defaultParams(definition));
    setOverrideTarget(false); setOverrideWindowID(""); setOverrideTabID("");
  }, [actionType, definition]);
  useEffect(() => {
    let stopped = false; let socket; let retry;
    const connect = () => {
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      socket = new WebSocket(`${protocol}//${window.location.host}/api/v1/browser/debug/ws`);
      socket.onmessage = (event) => {
        try {
          const value = JSON.parse(event.data);
          if (value.type === "browser.network" && value.payload?.session_id === captureIDRef.current) setNetwork((items) => [...items.slice(-999), value.payload]);
          if (value.type === "browser.network.status" && value.session_id === captureIDRef.current && value.payload?.status === "stopped") {
            captureIDRef.current = ""; setCapture(null); if (value.payload.error) notifyBrowserError(value.payload.error);
          }
          if (value.type === "browser.targets.changed") { void loadStatus(); void loadTargets(agentRef.current); }
        } catch { /* ignore malformed debug events */ }
      };
      socket.onclose = () => { if (!stopped) retry = window.setTimeout(connect, 1500); };
    };
    connect();
    return () => { stopped = true; window.clearTimeout(retry); socket?.close(); };
  }, [loadStatus, loadTargets]);

  const chooseAction = (item) => { setActionType(item.type); setResult(null); setLastRequest(null); setParams(defaultParams(item)); };
  const updateParam = (key, value) => setParams((current) => ({ ...current, [key]: value }));
  const actionNeedsTab = definition?.target_mode === "tab_required";
  const actionNeedsWindow = definition?.target_mode === "window_optional";
  const effectiveWindowID = overrideTarget ? overrideWindowID : windowID;
  const effectiveTabID = overrideTarget ? overrideTabID : tabID;
  const canRun = Boolean(definition && agentID && !running && (!actionNeedsTab || effectiveTabID));

  const execute = async () => {
    if (!definition) return;
    setRunning(true);
    try {
      const target = { agent_id: agentID };
      if (definition.target_mode === "window_optional" && effectiveWindowID) target.window_id = Number(effectiveWindowID);
      if (definition.target_mode === "tab_required") {
        target.tab_id = Number(effectiveTabID);
        const tab = allTabs.find((item) => String(item.id) === String(effectiveTabID));
        if (tab) target.window_id = tab.window_id;
      }
      const request = { timeout_ms: Number(timeoutMS) || 60000, target, action: { id: actionType.replaceAll(".", "-"), type: actionType, params: sanitizeValues(definition.parameters || [], params) } };
      setLastRequest(request);
      setResult(null);
      const value = await api("/api/v1/browser/action", { method: "POST", body: JSON.stringify(request) });
      setResult(value);
      if (actionType === "tab.open" && value?.data?.tab_id) { await loadTargets(agentID); setWindowID(String(value.data.window_id || "")); setTabID(String(value.data.tab_id)); }
    } catch (err) {
      notifyBrowserError(err);
      setResult({ type: definition.type, success: false, duration_ms: 0, error: err.message, data: {} });
    } finally { setRunning(false); }
  };

  const startCapture = async () => {
    if (!tabID) { notifyBrowserError("请先选择标签页"); return; }
    try {
      const value = await api("/api/v1/browser/network-captures", { method: "POST", body: JSON.stringify({ target: { agent_id: agentID, window_id: Number(windowID), tab_id: Number(tabID) }, rules: [{ id: "debug", ...captureRules }] }) });
      captureIDRef.current = value.id;
      setCapture(value); setNetwork([]); setSelectedNetwork(null);
    } catch (err) { notifyBrowserError(err); }
  };
  const stopCapture = async () => { if (!capture) return; try { const value = await api(`/api/v1/browser/network-captures/${capture.id}/stop`, { method: "POST" }); captureIDRef.current = ""; setNetwork(value.events || network); setCapture(null); } catch (err) { notifyBrowserError(err); } };
  const filteredNetwork = network.filter((item) => !networkFilter || String(item.url || "").toLowerCase().includes(networkFilter.toLowerCase()));

  return <div className="page-stack browser-debug-page">
    <PageHeader eyebrow="BROWSER TOOLS" title="浏览器操作调试" description="" />
    <Tabs defaultValue="actions" className="browser-debug-mode-tabs">
      <div className="browser-debug-toolbar">
        <TabsList><TabsTrigger value="actions"><Settings2 size={13} />浏览器操作</TabsTrigger><TabsTrigger value="network"><Network size={13} />网络捕获 {capture ? <Badge tone="success">{network.length}</Badge> : null}</TabsTrigger></TabsList>
        <div className="browser-target-toolbar">
          <Field label="执行节点"><Select value={agentID || undefined} onValueChange={setAgentID}><SelectTrigger><SelectValue placeholder="选择在线节点" /></SelectTrigger><SelectContent>{agents.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name || agent.id}</SelectItem>)}</SelectContent></Select></Field>
          <Field label="窗口"><Select value={windowID || undefined} onValueChange={(value) => { setWindowID(value); setTabID(""); }}><SelectTrigger><SelectValue placeholder="选择窗口" /></SelectTrigger><SelectContent>{windows.map((item) => <SelectItem key={item.id} value={String(item.id)}>窗口 {item.id}{item.focused ? " · 当前" : ""}</SelectItem>)}</SelectContent></Select></Field>
          <Field label="标签页"><Select value={tabID || undefined} onValueChange={setTabID}><SelectTrigger><SelectValue placeholder="选择标签页" /></SelectTrigger><SelectContent>{tabs.map((item) => <SelectItem key={item.id} value={String(item.id)}>#{item.id} {item.title || item.url || "未命名"}</SelectItem>)}</SelectContent></Select></Field>
          <IconButton title="刷新窗口和标签页" aria-label="刷新窗口和标签页" onClick={() => void loadTargets(agentID)}><RefreshCw size={14} /></IconButton>
        </div>
      </div>
      <TabsContent value="actions">
        <div className="browser-debug-workbench">
          <div className="browser-mobile-action-select"><Field label="原子 Action"><Select value={definition?.type} onValueChange={(value) => { const item = definitions.find((entry) => entry.type === value); if (item) chooseAction(item); }}><SelectTrigger><SelectValue placeholder="选择操作" /></SelectTrigger><SelectContent>{categories.flatMap((category) => category.actions.map((item) => <SelectItem key={item.type} value={item.type}>{category.label} · {item.label}</SelectItem>))}</SelectContent></Select></Field></div>
          <ActionNavigation categories={categories} selectedType={definition?.type} onSelect={chooseAction} />
          <section className="browser-action-console">
            <ActionConfiguration
              definition={definition}
              params={params}
              onParamChange={updateParam}
              timeoutMS={timeoutMS}
              onTimeoutChange={setTimeoutMS}
              customTarget={overrideTarget}
              onCustomTarget={setOverrideTarget}
              windows={windows}
              tabs={allTabs}
              windowID={effectiveWindowID}
              tabID={effectiveTabID}
              onWindow={setOverrideWindowID}
              onTab={(value) => { setOverrideTabID(value); const tab = allTabs.find((item) => String(item.id) === String(value)); if (tab) setOverrideWindowID(String(tab.window_id)); }}
              defaultTab={selectedTab}
              defaultWindow={selectedWindow}
              statusText={actionNeedsTab && !effectiveTabID ? "请选择目标标签页" : actionNeedsWindow && !effectiveWindowID ? "未指定时使用当前浏览器窗口" : "准备执行当前 Action"}
              canRun={canRun}
              running={running}
              onExecute={execute}
            />
          </section>
          <ResultPanel request={lastRequest} result={result} onClear={() => { setResult(null); setLastRequest(null); }} />
        </div>
      </TabsContent>
      <TabsContent value="network"><Card className="browser-network-card"><div className="browser-network-control"><div><Network size={16} /><strong>持续网络捕获</strong><span>{capture ? `固定绑定标签页 #${capture.target?.tab_id}` : "未启动"}</span></div><Button variant={capture ? "secondary" : "default"} onClick={() => void (capture ? stopCapture() : startCapture())}>{capture ? "停止捕获" : "开始捕获"}</Button></div><div className="browser-network-filters"><Field label="URL 包含"><Input disabled={Boolean(capture)} value={captureRules.url_contains} onChange={(event) => setCaptureRules((value) => ({ ...value, url_contains: event.target.value }))} /></Field><Field label="资源类型"><Input disabled={Boolean(capture)} value={captureRules.resource_type} onChange={(event) => setCaptureRules((value) => ({ ...value, resource_type: event.target.value }))} placeholder="XHR / Fetch" /></Field><Field label="正文上限"><Input disabled={Boolean(capture)} type="number" value={captureRules.max_body_bytes} onChange={(event) => setCaptureRules((value) => ({ ...value, max_body_bytes: Number(event.target.value) }))} /></Field><Field label="筛选"><Input value={networkFilter} onChange={(event) => setNetworkFilter(event.target.value)} placeholder="筛选 URL" /></Field></div><div className="browser-network-workspace"><NetworkList items={filteredNetwork} selected={selectedNetwork} onSelect={setSelectedNetwork} /><NetworkDetail item={selectedNetwork} /></div></Card></TabsContent>
    </Tabs>
  </div>;
}

function Field({ label, children }) { return <label className="browser-debug-field"><span>{label}</span>{children}</label>; }
function ActionNavigation({ categories, selectedType, onSelect }) {
  return <aside className="browser-action-navigation"><div><strong>原子 Action</strong></div><nav>{categories.map((category) => <section key={category.key}><h3>{category.label}</h3>{category.actions.map((item) => { const Icon = icons[item.icon] || Settings2; return <button type="button" key={item.type} data-selected={item.type === selectedType} onClick={() => onSelect(item)}><Icon size={14} /><span><strong>{item.label}</strong><small>{item.type}</small></span></button>; })}</section>)}</nav></aside>;
}
function ActionConfiguration({ definition, params, onParamChange, timeoutMS, onTimeoutChange, customTarget, onCustomTarget, windows, tabs, windowID, tabID, onWindow, onTab, defaultTab, defaultWindow, statusText, canRun, running, onExecute }) {
  if (!definition) return <div className="browser-debug-empty"><Settings2 size={22} /><span>暂无可用 Action</span></div>;
  const Icon = icons[definition.icon] || Settings2;
  const parameters = Array.isArray(definition.parameters) ? definition.parameters : [];
  return <div className="browser-action-config">
    <div className="browser-action-title"><div className="browser-action-title-copy"><span className="browser-action-title-icon"><Icon size={15} /></span><span><strong>{definition.label}</strong><code>{definition.type}</code><small>{definition.description}</small></span></div><TargetEditor definition={definition} custom={customTarget} onCustom={onCustomTarget} windows={windows} tabs={tabs} windowID={windowID} tabID={tabID} onWindow={onWindow} onTab={onTab} defaultTab={defaultTab} defaultWindow={defaultWindow} /></div>
    <div className="browser-debug-form">
      <DynamicFields parameters={parameters} values={params} onChange={onParamChange} />
      <Field label="超时 (ms)"><Input type="number" min="1000" value={timeoutMS} onChange={(event) => onTimeoutChange(event.target.value)} /></Field>
    </div>
    <div className="browser-debug-runbar"><span>{statusText}</span><Button disabled={!canRun} onClick={() => void onExecute()}>{running ? <LoaderCircle className="spin" size={15} /> : <Play size={15} />}{running ? "执行中" : "执行 Action"}</Button></div>
  </div>;
}
function TargetEditor({ definition, custom, onCustom, windows, tabs, windowID, tabID, onWindow, onTab, defaultTab, defaultWindow }) {
  if (definition.target_mode === "none") return null;
  const isTab = definition.target_mode === "tab_required";
  const value = custom ? (isTab ? tabID : windowID) : "none";
  const changeTarget = (next) => {
    if (next === "none") {
      onCustom(false);
      if (isTab) onTab(""); else onWindow("");
      return;
    }
    onCustom(true);
    if (isTab) onTab(next); else onWindow(next);
  };
  const inherited = isTab ? (defaultTab ? `无 · 顶部 #${defaultTab.id}` : "无 · 顶部未选择") : (defaultWindow ? `无 · 顶部窗口 ${defaultWindow.id}` : "无 · 当前窗口");
  return <div className="browser-action-target"><span>{isTab ? "自定义标签页" : "自定义窗口"}</span><Select value={value || "none"} onValueChange={changeTarget}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="none">{inherited}</SelectItem>{isTab ? tabs.map((item) => <SelectItem key={item.id} value={String(item.id)}>#{item.id} · 窗口 {item.window_id} · {item.title || item.url}</SelectItem>) : windows.map((item) => <SelectItem key={item.id} value={String(item.id)}>窗口 {item.id}</SelectItem>)}</SelectContent></Select></div>;
}
function ResultPanel({ request, result, onClear }) {
  const [copied, setCopied] = useState("");
  const [activeTab, setActiveTab] = useState("preview");
  useEffect(() => { if (result) setActiveTab("preview"); }, [result]);
  const copy = async (key, value) => {
    if (!value) return;
    await navigator.clipboard.writeText(JSON.stringify(value, null, 2));
    setCopied(key);
    window.setTimeout(() => setCopied((current) => current === key ? "" : current), 1200);
  };
  return <section className="browser-debug-output">
    <div className="browser-debug-panel-header"><div><SquareTerminal size={16} /><strong>执行结果</strong></div><IconButton title="清空结果" aria-label="清空结果" disabled={!result && !request} onClick={onClear}><Trash2 size={14} /></IconButton></div>
    {result ? <Tabs value={activeTab} onValueChange={setActiveTab} className="browser-result-tabs">
      <TabsList>
        <TabsTrigger value="preview"><Image size={13} />结果预览</TabsTrigger>
        <TabsTrigger value="request"><FileText size={13} />原始请求</TabsTrigger>
        <TabsTrigger value="response"><Code2 size={13} />原始响应</TabsTrigger>
      </TabsList>
      <TabsContent value="preview"><ResultPreview result={result} /></TabsContent>
      <TabsContent value="request"><RawResult value={request} label="复制原始请求" copied={copied === "request"} onCopy={() => void copy("request", request)} /></TabsContent>
      <TabsContent value="response"><RawResult value={result} label="复制原始响应" copied={copied === "response"} onCopy={() => void copy("response", result)} /></TabsContent>
    </Tabs> : <div className="browser-debug-empty"><SquareTerminal size={22} /><span>{request ? "Action 执行中或未返回结果" : "尚未执行 Action"}</span></div>}
  </section>;
}
function RawResult({ value, label, copied, onCopy }) {
  return <div className="browser-result-raw"><div><span>{label.replace("复制", "")}</span><IconButton title={copied ? "已复制" : label} aria-label={label} disabled={!value} onClick={onCopy}>{copied ? <Check size={14} /> : <Clipboard size={14} />}</IconButton></div><pre>{value ? JSON.stringify(value, null, 2) : "暂无数据"}</pre></div>;
}
function ResultPreview({ result }) {
  const images = collectResultImages(result?.data);
  return <div className="browser-result-preview">
    <div className="browser-result-summary">
      <span><small>执行状态</small><Badge tone={result.success ? "success" : "warning"}>{result.success ? "成功" : "失败"}</Badge></span>
      <span><small>Action</small><strong>{result.type || "-"}</strong></span>
      <span><small>执行耗时</small><strong>{result.duration_ms ?? 0} ms</strong></span>
      <span><small>目标</small><strong>{formatTarget(result.target)}</strong></span>
    </div>
    {result.error ? <div className="browser-result-error">{result.error}</div> : null}
    {images.length ? <div className="browser-result-images">{images.map((item, index) => <figure key={`${item.label}-${index}`}><img src={item.src} alt={item.label} loading="lazy" referrerPolicy="no-referrer" /><figcaption>{item.label}</figcaption></figure>)}</div> : null}
    <section className="browser-result-data"><h3>响应数据</h3><PreviewValue value={result.data} /></section>
  </div>;
}
function PreviewValue({ value, depth = 0 }) {
  if (value == null) return <span className="browser-result-null">null</span>;
  if (typeof value === "boolean") return <Badge tone={value ? "success" : "muted"}>{value ? "true" : "false"}</Badge>;
  if (typeof value === "number") return <code>{value}</code>;
  if (typeof value === "string") return <PreviewString value={value} />;
  if (Array.isArray(value)) return value.length ? <div className="browser-result-array">{value.map((item, index) => <div key={index}><span>#{index + 1}</span><PreviewValue value={item} depth={depth + 1} /></div>)}</div> : <span className="browser-result-null">空数组</span>;
  const entries = Object.entries(value).filter(([, item]) => !isRenderedImage(item));
  return entries.length ? <dl className={`browser-result-object${depth ? " is-nested" : ""}`}>{entries.map(([key, item]) => <div key={key}><dt>{key}</dt><dd><PreviewValue value={item} depth={depth + 1} /></dd></div>)}</dl> : <span className="browser-result-null">无数据</span>;
}
function PreviewString({ value }) {
  const trimmed = value.trim();
  if (isRenderedImage(trimmed)) return <span className="browser-result-null">已在图片区域显示</span>;
  if (/^https?:\/\//i.test(trimmed)) return <a href={trimmed} target="_blank" rel="noreferrer">{value}</a>;
  if (trimmed.startsWith("<") || value.includes("\n") || value.length > 180) return <pre>{value}</pre>;
  return <span>{value || "空字符串"}</span>;
}
function collectResultImages(value, path = "image", found = []) {
  if (found.length >= 8 || value == null) return found;
  if (typeof value === "string" && isRenderedImage(value)) found.push({ label: path, src: value });
  else if (Array.isArray(value)) value.forEach((item, index) => collectResultImages(item, `${path}[${index}]`, found));
  else if (typeof value === "object") Object.entries(value).forEach(([key, item]) => collectResultImages(item, key, found));
  return found;
}
function isRenderedImage(value) { return typeof value === "string" && (/^data:image\/(png|jpe?g|webp|gif);base64,/i.test(value) || /^https?:\/\/[^?#]+\.(png|jpe?g|webp|gif)(?:[?#].*)?$/i.test(value)); }
function formatTarget(target) { return target?.tab_id ? `窗口 ${target.window_id || "-"} / 标签页 ${target.tab_id}` : target?.window_id ? `窗口 ${target.window_id}` : "无指定目标"; }
function notifyBrowserError(error) { toast.error(error?.message || String(error || "浏览器操作失败"), { id: "browser-debug-error" }); }
function NetworkList({ items, selected, onSelect }) { return <div className="browser-network-list"><div className="browser-network-list-head"><span>状态</span><span>方法</span><span>地址</span><span>类型</span><span>耗时</span></div>{items.length ? items.map((item, index) => <button type="button" className="browser-network-list-row" data-selected={item === selected} key={`${item.session_id}-${item.url}-${index}`} onClick={() => onSelect(item)}><span>{item.status || "ERR"}</span><span>{item.method || "GET"}</span><strong title={item.url}>{item.url}</strong><span>{item.resource_type || item.mime_type || "-"}</span><span>{item.duration_ms || 0} ms</span></button>) : <div className="browser-debug-empty">暂无捕获请求</div>}</div>; }
function NetworkDetail({ item }) { if (!item) return <div className="browser-network-detail-empty">选择请求查看标头、正文和时序</div>; return <div className="browser-network-detail"><div><strong>{item.method || "GET"} {item.url}</strong><span>{item.status || "ERR"} · {item.mime_type || item.resource_type || "未知类型"} · {item.duration_ms || 0} ms</span></div><Tabs defaultValue="response"><TabsList><TabsTrigger value="response">响应</TabsTrigger><TabsTrigger value="request">请求</TabsTrigger><TabsTrigger value="timing">时序</TabsTrigger></TabsList><TabsContent value="response"><DetailObject title="响应标头" value={item.headers} /><DetailBody value={item.body} error={item.error} truncated={item.truncated} /></TabsContent><TabsContent value="request"><DetailObject title="请求标头" value={item.request_headers} /><DetailBody value={item.request_body} truncated={item.request_body_truncated} /></TabsContent><TabsContent value="timing"><DetailObject title="连接与时序" value={{ protocol: item.protocol, remote_address: [item.remote_ip_address, item.remote_port].filter(Boolean).join(":"), encoded_data_length: item.encoded_data_length, from_disk_cache: item.from_disk_cache, from_service_worker: item.from_service_worker, ...(item.timing || {}) }} /></TabsContent></Tabs></div>; }
function DetailObject({ title, value }) { const entries = Object.entries(value || {}).filter(([, item]) => item !== undefined && item !== ""); return <section className="browser-network-detail-section"><h4>{title}</h4>{entries.length ? <dl>{entries.map(([key, item]) => <div key={key}><dt>{key}</dt><dd>{String(item)}</dd></div>)}</dl> : <div className="browser-network-detail-empty">无数据</div>}</section>; }
function DetailBody({ value, error, truncated }) { return <section className="browser-network-body"><pre className={error ? "is-error" : ""}>{value || error || "无正文"}</pre>{truncated ? <span>正文已按配置上限截断</span> : null}</section>; }
function defaultParams(definition) { return getDefaultValues({ parameters: definition?.parameters || [] }); }
