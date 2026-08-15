import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, AppWindow, ArrowLeft, ArrowRight, Camera, Check, CircleStop, Clipboard, Code2, Cookie, Copy, Database, FileText, Focus, Gauge, Globe2, Group, Image, Info, Keyboard, Languages, ListTree, LoaderCircle, LocateFixed, MemoryStick, Mouse, MousePointer2, MousePointerClick, MoveDiagonal2, MoveHorizontal, MoveVertical, Network, PanelTopOpen, PanelsTopLeft, Pin, Play, RefreshCw, Scan, Search, Send, Settings2, SquareCheckBig, SquareTerminal, Tags, Timer, Trash2, Ungroup, Unlink, VolumeX, X, Zap, ZoomIn } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "../components/layout/PageHeader";
import { DynamicFields } from "../components/forms/DynamicFields";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { IconButton } from "../components/ui/IconButton";
import { Input } from "../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/Select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/Tabs";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "../components/ui/AlertDialog";
import { NetworkPanel } from "../features/browser/NetworkPanel";
import { api } from "../lib/api";
import { findMissingRequiredParameters, getDefaultValues, sanitizeValues } from "../lib/parameterSchema";

const icons = { "app-window": AppWindow, scan: Scan, "panels-top-left": PanelsTopLeft, "move-diagonal-2": MoveDiagonal2, x: X, "panel-top-open": PanelTopOpen, "mouse-pointer-2": MousePointer2, globe: Globe2, "refresh-cw": RefreshCw, "arrow-left": ArrowLeft, "arrow-right": ArrowRight, copy: Copy, "move-horizontal": MoveHorizontal, pin: Pin, "volume-x": VolumeX, "memory-stick": MemoryStick, languages: Languages, group: Group, ungroup: Ungroup, "zoom-in": ZoomIn, "trash-2": Trash2, info: Info, timer: Timer, "move-vertical": MoveVertical, "circle-stop": CircleStop, gauge: Gauge, camera: Camera, "file-code-2": Code2, search: Search, "list-tree": ListTree, focus: Focus, unlink: Unlink, "mouse-pointer-click": MousePointerClick, keyboard: Keyboard, "square-check-big": SquareCheckBig, send: Send, tags: Tags, zap: Zap, "locate-fixed": LocateFixed, mouse: Mouse, cookie: Cookie, "cookie-off": Cookie, database: Database, "database-zap": Database, "database-x": Database, "database-backup": Database, "code-2": Code2 };

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
  const [overrideTarget, setOverrideTarget] = useState(false);
  const [overrideWindowID, setOverrideWindowID] = useState("");
  const [overrideTabID, setOverrideTabID] = useState("");
  const [selectedNetwork, setSelectedNetwork] = useState(null);
  const [confirmAction, setConfirmAction] = useState(false);
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
  const effectiveWindowID = overrideTarget ? overrideWindowID : windowID;
  const effectiveTabID = overrideTarget ? overrideTabID : tabID;
  const effectiveTab = allTabs.find((item) => String(item.id) === String(effectiveTabID));
  const effectiveWindow = windows.find((item) => String(item.id) === String(effectiveWindowID));
  const targetIssue = actionNeedsTab && !effectiveTab ? "请选择有效标签页" : definition?.target_mode === "window_required" && !effectiveWindow ? "请选择有效窗口" : "";
  const canRun = Boolean(definition && agentID && !running && !targetIssue);

  const loadSelectorCandidates = useCallback(async ({ queries, limit }) => {
    if (!agentID || !effectiveTab) throw new Error("请先选择有效的目标标签页");
    return api("/api/v1/browser/selector-candidates", { method: "POST", body: JSON.stringify({ target: { agent_id: agentID, window_id: Number(effectiveTab.window_id), tab_id: Number(effectiveTab.id) }, queries, ...(limit ? { limit } : {}) }) });
  }, [agentID, effectiveTab]);

  const execute = async (confirmed = false) => {
    if (!definition) return;
    const missing = findMissingRequiredParameters(definition.parameters || [], params);
    if (missing.length) {
      toast.error(`请填写必填参数：${missing.map((parameter) => parameter.label || parameter.key).join("、")}`, { id: "browser-action-required" });
      return;
    }
    if (definition.destructive && !confirmed) { setConfirmAction(true); return; }
    setConfirmAction(false);
    setRunning(true);
    try {
      const target = { agent_id: agentID };
      if (["window_optional", "window_required"].includes(definition.target_mode) && effectiveWindowID) target.window_id = Number(effectiveWindowID);
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
      if (actionType === "window.open" && value?.data?.window_id) { await loadTargets(agentID); setWindowID(String(value.data.window_id)); setTabID(String(value.data.tabs?.[0]?.tab_id || "")); }
    } catch (err) {
      notifyBrowserError(err);
      setResult({ type: definition.type, success: false, duration_ms: 0, error: err.message, data: {} });
    } finally { setRunning(false); }
  };

  const startCapture = async ({ preserveLog, disableCache, maxBodyBytes }) => {
    if (!tabID) { notifyBrowserError("请先选择标签页"); return; }
    try {
      const value = await api("/api/v1/browser/network-captures", { method: "POST", body: JSON.stringify({ target: { agent_id: agentID, window_id: Number(windowID), tab_id: Number(tabID) }, disable_cache: disableCache, rules: [{ id: "debug", max_body_bytes: maxBodyBytes }] }) });
      captureIDRef.current = value.id;
      setCapture(value); if (!preserveLog) { setNetwork([]); setSelectedNetwork(null); }
    } catch (err) { notifyBrowserError(err); }
  };
  const stopCapture = async () => { if (!capture) return; try { const value = await api(`/api/v1/browser/network-captures/${capture.id}/stop`, { method: "POST" }); captureIDRef.current = ""; setNetwork((items) => { const received = items.filter((item) => item.session_id === capture.id).length; const missing = (value.events || []).slice(received); return [...items, ...missing].slice(-1000); }); setCapture(null); } catch (err) { notifyBrowserError(err); } };

  return <div className="page-stack browser-debug-page">
    <PageHeader eyebrow="LAB / BROWSER" title="浏览器控制" description="" />
    <Tabs defaultValue="actions" className="browser-debug-mode-tabs">
      <div className="browser-debug-toolbar">
        <TabsList><TabsTrigger value="actions"><Settings2 size={13} />浏览器操作</TabsTrigger><TabsTrigger value="network"><Network size={13} />网络捕获 {capture ? <Badge tone="success">{network.length}</Badge> : null}</TabsTrigger></TabsList>
        <div className="browser-target-toolbar">
          <Field label="执行节点"><Select value={agentID} onValueChange={setAgentID}><SelectTrigger><SelectValue placeholder="选择在线节点" /></SelectTrigger><SelectContent>{agents.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name || agent.id}</SelectItem>)}</SelectContent></Select></Field>
          <Field label="窗口"><Select value={windowID} onValueChange={(value) => { setWindowID(value); setTabID(""); }}><SelectTrigger><SelectValue placeholder="选择窗口" /></SelectTrigger><SelectContent>{windows.map((item) => <SelectItem key={item.id} value={String(item.id)}>窗口 {item.id}{item.focused ? " · 当前" : ""}</SelectItem>)}</SelectContent></Select></Field>
          <Field label="标签页"><Select value={tabID} onValueChange={setTabID}><SelectTrigger><SelectValue placeholder="选择标签页" /></SelectTrigger><SelectContent>{tabs.map((item) => <SelectItem key={item.id} value={String(item.id)}>#{item.id} {item.title || item.url || "未命名"}</SelectItem>)}</SelectContent></Select></Field>
          <IconButton title="刷新窗口和标签页" aria-label="刷新窗口和标签页" onClick={() => void loadTargets(agentID)}><RefreshCw size={14} /></IconButton>
        </div>
      </div>
      <TabsContent value="actions">
        <div className="browser-debug-workbench">
          <div className="browser-mobile-action-select"><Field label="原子 Action"><Select value={definition?.type || ""} onValueChange={(value) => { const item = definitions.find((entry) => entry.type === value); if (item) chooseAction(item); }}><SelectTrigger><SelectValue placeholder="选择操作" /></SelectTrigger><SelectContent>{categories.flatMap((category) => category.actions.map((item) => <SelectItem key={item.type} value={item.type}>{category.label} · {item.label}</SelectItem>))}</SelectContent></Select></Field></div>
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
              targetIssue={targetIssue}
              canRun={canRun}
              running={running}
              onExecute={execute}
              loadSelectorCandidates={loadSelectorCandidates}
              selectorTargetKey={`${agentID}:${effectiveWindowID}:${effectiveTabID}`}
            />
          </section>
          <ResultPanel request={lastRequest} result={result} onClear={() => { setResult(null); setLastRequest(null); }} />
        </div>
      </TabsContent>
      <TabsContent value="network"><NetworkPanel capture={capture} items={network} selected={selectedNetwork} onSelect={setSelectedNetwork} onStart={startCapture} onStop={stopCapture} onClear={() => { setNetwork([]); setSelectedNetwork(null); }} canStart={Boolean(agentID && tabID)} targetLabel={capture ? `标签页 #${capture.target?.tab_id}` : selectedTab ? `标签页 #${selectedTab.id} · ${selectedTab.title || selectedTab.url || "未命名"}` : ""} /></TabsContent>
    </Tabs>
    <AlertDialog open={confirmAction} onOpenChange={(open) => !running && setConfirmAction(open)}><AlertDialogContent><AlertDialogHeader><div className="alert-dialog-icon"><AlertTriangle size={18} /></div><AlertDialogTitle>确认执行“{definition?.label}”？</AlertDialogTitle><AlertDialogDescription>{definition?.description} 此操作会修改浏览器状态或页面认证数据，请确认目标窗口和标签页无误。</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel disabled={running}>取消</AlertDialogCancel><AlertDialogAction disabled={running} onClick={(event) => { event.preventDefault(); void execute(true); }}>{running ? "执行中..." : "确认执行"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </div>;
}

function Field({ label, children }) { return <label className="browser-debug-field"><span>{label}</span>{children}</label>; }
function ActionNavigation({ categories, selectedType, onSelect }) {
  const [query, setQuery] = useState("");
  const navigationRef = useRef(null);
  const normalizedQuery = query.trim().toLowerCase();
  const filteredCategories = normalizedQuery
    ? categories.map((category) => ({ ...category, actions: category.actions.filter((item) => [category.label, item.label, item.type].some((value) => String(value || "").toLowerCase().includes(normalizedQuery))) })).filter((category) => category.actions.length)
    : categories;
  useEffect(() => { navigationRef.current?.scrollTo({ top: 0 }); }, [query]);
  return <aside className="browser-action-navigation"><div><strong>原子 Action</strong></div><label className="browser-action-search"><Search size={13} /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索 Action" aria-label="搜索 Action" /></label><nav ref={navigationRef}>{filteredCategories.length ? filteredCategories.map((category) => <section key={category.key}><h3>{category.label}</h3>{category.actions.map((item) => { const Icon = icons[item.icon] || Settings2; return <button type="button" key={item.type} data-selected={item.type === selectedType} onClick={() => onSelect(item)}><Icon size={14} /><span><strong>{item.label}</strong><small>{item.type}</small></span></button>; })}</section>) : <span className="browser-action-search-empty">未找到匹配的 Action</span>}</nav></aside>;
}
function ActionConfiguration({ definition, params, onParamChange, timeoutMS, onTimeoutChange, customTarget, onCustomTarget, windows, tabs, windowID, tabID, onWindow, onTab, defaultTab, defaultWindow, targetIssue, canRun, running, onExecute, loadSelectorCandidates, selectorTargetKey }) {
  if (!definition) return <div className="browser-debug-empty"><Settings2 size={22} /><span>暂无可用 Action</span></div>;
  const Icon = icons[definition.icon] || Settings2;
  const parameters = Array.isArray(definition.parameters) ? definition.parameters : [];
  return <div className="browser-action-config">
    <div className="browser-action-title"><div className="browser-action-title-copy"><span className="browser-action-title-icon"><Icon size={15} /></span><span className="browser-action-title-text"><span className="browser-action-title-line"><strong>{definition.label}</strong><code>{definition.type}</code>{definition.sensitive ? <Badge tone="warning">敏感</Badge> : null}</span><small title={definition.description}>{definition.description}</small></span></div></div>
    <div className="browser-debug-form">
      <TargetEditor definition={definition} custom={customTarget} onCustom={onCustomTarget} windows={windows} tabs={tabs} windowID={windowID} tabID={tabID} onWindow={onWindow} onTab={onTab} defaultTab={defaultTab} defaultWindow={defaultWindow} targetIssue={targetIssue} />
      <DynamicFields parameters={parameters} values={params} onChange={onParamChange} browserTargets={{ windows, tabs, loadSelectorCandidates, selectorTargetKey }} />
      <Field label="超时 (ms)"><Input type="number" min="1000" value={timeoutMS} onChange={(event) => onTimeoutChange(event.target.value)} /></Field>
    </div>
    <div className="browser-debug-runbar"><Button disabled={!canRun} title={!canRun && !running ? targetIssue || "当前无法执行" : undefined} onClick={() => void onExecute()}>{running ? <LoaderCircle className="spin" size={15} /> : <Play size={15} />}{running ? "执行中" : "执行 Action"}</Button></div>
  </div>;
}
function TargetEditor({ definition, custom, onCustom, windows, tabs, windowID, tabID, onWindow, onTab, defaultTab, defaultWindow, targetIssue }) {
  if (definition.target_mode === "none") return null;
  const isTab = definition.target_mode === "tab_required";
  const value = isTab ? tabID : windowID;
  const selectedOverrideTab = isTab ? tabs.find((item) => String(item.id) === String(tabID)) : null;
  const inheritedWindow = defaultWindow ? `跟随全局目标 · 窗口 ${defaultWindow.id}${defaultWindow.focused ? " · 当前窗口" : ""}` : definition.target_mode === "window_required" ? "顶部尚未选择窗口" : "跟随全局目标 · 浏览器当前窗口";
  const selectMode = (override) => {
    onCustom(override);
    if (!override) return;
    if (isTab) onTab(String(defaultTab?.id || tabs[0]?.id || ""));
    else onWindow(String(defaultWindow?.id || windows[0]?.id || ""));
  };
  return <section className="browser-action-target" data-custom={custom} data-invalid={Boolean(targetIssue)}>
    <div className="browser-action-target-main">
      <div className="browser-action-target-copy"><span><LocateFixed size={14} /></span><div><strong>执行目标</strong>{!custom && isTab && defaultTab ? <TabTargetLabel tab={defaultTab} prefix="跟随全局" /> : <small>{custom ? `本次 Action 使用指定${isTab ? "标签页" : "窗口"}` : isTab ? "顶部尚未选择标签页" : inheritedWindow}</small>}</div></div>
      {custom && <Select value={value || ""} onValueChange={isTab ? onTab : onWindow}><SelectTrigger aria-label={`选择执行${isTab ? "标签页" : "窗口"}`} title={isTab ? tabTargetTooltip(selectedOverrideTab) : undefined}><SelectValue placeholder={`选择${isTab ? "标签页" : "窗口"}`}>{selectedOverrideTab ? <TabTargetLabel tab={selectedOverrideTab} /> : undefined}</SelectValue></SelectTrigger><SelectContent className="browser-action-target-select-content">{isTab ? tabs.map((item) => <SelectItem className="browser-action-target-option" key={item.id} value={String(item.id)} textValue={tabTargetText(item)}><TabTargetLabel tab={item} /></SelectItem>) : windows.map((item) => <SelectItem key={item.id} value={String(item.id)}>窗口 {item.id}{item.focused ? " · 当前" : ""}</SelectItem>)}</SelectContent></Select>}
    </div>
    <div className="browser-action-target-controls">
      {targetIssue && <Badge tone="warning" className="browser-action-target-warning"><AlertTriangle size={11} />{targetIssue}</Badge>}
      {custom && <Badge tone="warning" className="browser-action-target-override">已覆盖全局目标</Badge>}
      <div className="browser-action-target-mode" role="group" aria-label="执行目标来源">
        <button type="button" data-active={!custom} aria-pressed={!custom} onClick={() => selectMode(false)}>跟随全局</button>
        <button type="button" data-active={custom} aria-pressed={custom} onClick={() => selectMode(true)}>单次覆盖</button>
      </div>
    </div>
  </section>;
}
function TabTargetLabel({ tab, prefix = "" }) {
  const title = tab?.title || tab?.url || "未命名标签页";
  const windowID = tab?.window_id || tab?.window?.id;
  const url = tab?.url && tab.url !== title ? tab.url : "";
  return <span className="browser-tab-target-label" title={tabTargetTooltip(tab)}><span>{title}</span><small>{[prefix, `#${tab?.id ?? "-"}`, windowID ? `窗口 ${windowID}` : "", url].filter(Boolean).join(" · ")}</small></span>;
}
function tabTargetText(tab) { return [tab?.title, tab?.url, tab?.id ? `标签页 ${tab.id}` : "", tab?.window_id ? `窗口 ${tab.window_id}` : ""].filter(Boolean).join(" "); }
function tabTargetTooltip(tab) { return tab ? [`标签页 #${tab.id}`, tab.title, tab.url].filter(Boolean).join("\n") : undefined; }
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
function defaultParams(definition) { return getDefaultValues({ parameters: definition?.parameters || [] }); }
