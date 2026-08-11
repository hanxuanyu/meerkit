import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Camera, Check, Clipboard, Code2, Eraser, FileCode2, Globe2, Group, Keyboard, ListChecks, LoaderCircle, MousePointerClick, Network, PanelTopOpen, Play, RefreshCw, Search, SquareTerminal, Timer, Trash2, X } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { IconButton } from "../components/ui/IconButton";
import { Input } from "../components/ui/Input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "../components/ui/Select";
import { Switch } from "../components/ui/Switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../components/ui/Tabs";
import { api } from "../lib/api";

const actionOptions = [
  { value: "tab.open", label: "打开或刷新标签页", icon: PanelTopOpen },
  { value: "tab.navigate", label: "导航标签页", icon: Globe2 },
  { value: "tab.group", label: "加入标签页分组", icon: Group },
  { value: "tab.close", label: "关闭标签页", icon: Trash2 },
  { value: "page.wait", label: "等待页面或元素", icon: Timer },
  { value: "page.screenshot", label: "页面截图", icon: Camera },
  { value: "dom.document", label: "获取页面 HTML", icon: FileCode2 },
  { value: "dom.query", label: "查询 CSS 元素", icon: Search },
  { value: "dom.click", label: "点击 CSS 元素", icon: MousePointerClick },
  { value: "dom.input", label: "填写表单控件", icon: Keyboard },
  { value: "runtime.evaluate", label: "执行 JavaScript", icon: Code2 },
  { value: "network.capture", label: "捕获网络响应", icon: Network }
];

const rawRequestTemplate = {
  timeout_ms: 60000,
  keep_tab: true,
  actions: [
    { id: "open", type: "tab.open", params: { url: "https://example.com", active: true, reuse: true, reuse_key: "browser-debug", group_title: "Meerkit Debug" } },
    { id: "group", type: "tab.group", params: { title: "Meerkit Debug", color: "blue", reuse_group: true } },
    { id: "query", type: "dom.query", params: { selector: "body", max_length: 65536 }, continue_on_error: true }
  ]
};

export function BrowserDebugPage() {
  const [mode, setMode] = useState("quick");
  const [status, setStatus] = useState(null);
  const [statusError, setStatusError] = useState("");
  const [agentID, setAgentID] = useState("");
  const [actionType, setActionType] = useState("dom.query");
  const [targetURL, setTargetURL] = useState("https://example.com");
  const [navigateURL, setNavigateURL] = useState("https://example.com");
  const [timeoutMS, setTimeoutMS] = useState(60000);
  const [keepTab, setKeepTab] = useState(true);
  const [reuseTab, setReuseTab] = useState(true);
  const [reuseKey, setReuseKey] = useState("browser-debug");
  const [activeTab, setActiveTab] = useState(true);
  const [groupTitle, setGroupTitle] = useState("Meerkit Debug");
  const [groupColor, setGroupColor] = useState("blue");
  const [groupCollapsed, setGroupCollapsed] = useState(false);
  const [selector, setSelector] = useState("body");
  const [inputValue, setInputValue] = useState("");
  const [expression, setExpression] = useState("({ title: document.title, url: location.href })");
  const [maxLength, setMaxLength] = useState(65536);
  const [waitMode, setWaitMode] = useState("selector");
  const [waitDuration, setWaitDuration] = useState(1000);
  const [screenshotFormat, setScreenshotFormat] = useState("png");
  const [screenshotQuality, setScreenshotQuality] = useState(90);
  const [fullPage, setFullPage] = useState(false);
  const [captureURL, setCaptureURL] = useState("");
  const [resourceType, setResourceType] = useState("");
  const [maxBodyBytes, setMaxBodyBytes] = useState(262144);
  const [rawRequest, setRawRequest] = useState(() => JSON.stringify(rawRequestTemplate, null, 2));
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState(null);
  const [runError, setRunError] = useState("");
  const [resultTab, setResultTab] = useState("actions");
  const [copied, setCopied] = useState(false);

  const loadStatus = useCallback(async () => {
    try {
      const value = await api("/api/v1/browser");
      const agents = value?.connected_agents || [];
      setStatus(value);
      setStatusError("");
      setAgentID((current) => agents.some((agent) => agent.id === current) ? current : agents[0]?.id || "");
    } catch (error) {
      setStatusError(error.message);
    }
  }, []);

  useEffect(() => {
    void loadStatus();
    const timer = window.setInterval(() => void loadStatus(), 5000);
    return () => window.clearInterval(timer);
  }, [loadStatus]);

  const agents = status?.connected_agents || [];
  const selectedAgent = agents.find((agent) => agent.id === agentID);
  const capabilities = new Set(selectedAgent?.capabilities || []);
  const selectedAction = actionOptions.find((item) => item.value === actionType) || actionOptions[0];
  const missingCapabilities = useMemo(() => {
    if (!selectedAgent) return [];
    const required = new Set(["tab.open", actionType === "network.capture" ? "page.wait" : actionType]);
    if (actionType === "network.capture") required.add("network.capture");
    if (groupTitle.trim()) required.add("tab.group");
    return [...required].filter((capability) => !capabilities.has(capability));
  }, [actionType, capabilities, groupTitle, selectedAgent]);

  const buildQuickRequest = () => {
    const openURL = actionType === "tab.navigate" ? "about:blank" : targetURL.trim();
    const actions = [{ id: "open", type: "tab.open", params: { url: openURL, active: activeTab, reuse: actionType === "tab.navigate" ? false : reuseTab, ...(reuseTab && actionType !== "tab.navigate" && reuseKey.trim() ? { reuse_key: reuseKey.trim() } : {}), ...(groupTitle.trim() ? { group_title: groupTitle.trim() } : {}) } }];
    const groupAction = { id: "group", type: "tab.group", params: { title: groupTitle.trim() || "Meerkit Debug", color: groupColor, collapsed: groupCollapsed, reuse_group: true }, continue_on_error: true };
    if (groupTitle.trim() && actionType !== "tab.group") actions.push(groupAction);
    const action = quickAction(actionType, { navigateURL, selector, inputValue, expression, maxLength, waitMode, waitDuration, screenshotFormat, screenshotQuality, fullPage, groupTitle, groupColor, groupCollapsed });
    if (action) actions.push(action);
    return {
      agent_id: agentID,
      timeout_ms: Number(timeoutMS) || 60000,
      keep_tab: keepTab,
      actions,
      ...(actionType === "network.capture" ? { network_captures: [{ id: "network", url_contains: captureURL, ...(resourceType ? { resource_type: resourceType } : {}), max_body_bytes: Number(maxBodyBytes) || 262144 }] } : {})
    };
  };

  const execute = async () => {
    setRunError("");
    let request;
    try {
      request = mode === "raw" ? JSON.parse(rawRequest) : buildQuickRequest();
      if (!request || typeof request !== "object" || Array.isArray(request)) throw new Error("请求必须是 JSON 对象");
      if (!request.agent_id && agentID) request.agent_id = agentID;
    } catch (error) {
      setRunError(`请求格式错误：${error.message}`);
      return;
    }
    setRunning(true);
    try {
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
  const formatRawRequest = () => {
    try { setRawRequest(JSON.stringify(JSON.parse(rawRequest), null, 2)); setRunError(""); }
    catch (error) { setRunError(`请求格式错误：${error.message}`); }
  };

  const ActionIcon = selectedAction.icon;
  const canRun = Boolean(selectedAgent) && !running && (mode === "raw" || missingCapabilities.length === 0);
  return <div className="page-stack browser-debug-page">
    <PageHeader eyebrow="BROWSER TOOLS" title="浏览器操作调试" description="" actions={<div className="browser-debug-heading-actions"><Badge tone={agents.length ? "success" : "muted"}><span className="status-dot" />{agents.length ? `${agents.length} 个节点在线` : "无可用节点"}</Badge><IconButton variant="outline" size="default" title="刷新执行节点" aria-label="刷新执行节点" onClick={() => void loadStatus()}><RefreshCw size={15} /></IconButton></div>} />
    {(statusError || runError) && <div className="browser-debug-error" role="alert"><X size={15} /><span>{runError || statusError}</span></div>}
    <div className="browser-debug-workbench">
      <Card className="browser-debug-console">
        <div className="browser-debug-panel-header"><div><SquareTerminal size={16} /><strong>执行控制台</strong></div>{selectedAgent && <Badge tone="success">{selectedAgent.name || selectedAgent.id}</Badge>}</div>
        <Tabs value={mode} onValueChange={setMode} className="browser-debug-mode-tabs">
          <TabsList><TabsTrigger value="quick">单项调试</TabsTrigger><TabsTrigger value="raw">原始请求</TabsTrigger></TabsList>
          <TabsContent value="quick">
            <div className="browser-debug-form">
              <DebugField label="执行节点" wide><Select value={agentID || undefined} onValueChange={setAgentID}><SelectTrigger><SelectValue placeholder="选择在线节点" /></SelectTrigger><SelectContent>{agents.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name || agent.id} · {agent.version || "unknown"}</SelectItem>)}</SelectContent></Select></DebugField>
              <DebugField label="目标地址" wide><Input type="url" value={targetURL} onChange={(event) => setTargetURL(event.target.value)} placeholder="https://example.com" /></DebugField>
              <DebugField label="调试能力" wide><Select value={actionType} onValueChange={setActionType}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectGroup><SelectLabel>标签页</SelectLabel>{actionOptions.slice(0, 4).map((item) => <CapabilityOption key={item.value} item={item} capabilities={capabilities} />)}</SelectGroup><SelectGroup><SelectLabel>页面与 DOM</SelectLabel>{actionOptions.slice(4, 10).map((item) => <CapabilityOption key={item.value} item={item} capabilities={capabilities} />)}</SelectGroup><SelectGroup><SelectLabel>运行时与网络</SelectLabel>{actionOptions.slice(10).map((item) => <CapabilityOption key={item.value} item={item} capabilities={capabilities} />)}</SelectGroup></SelectContent></Select></DebugField>
              <ActionFields type={actionType} values={{ navigateURL, selector, inputValue, expression, maxLength, waitMode, waitDuration, screenshotFormat, screenshotQuality, fullPage, captureURL, resourceType, maxBodyBytes, groupTitle, groupColor, groupCollapsed }} setters={{ setNavigateURL, setSelector, setInputValue, setExpression, setMaxLength, setWaitMode, setWaitDuration, setScreenshotFormat, setScreenshotQuality, setFullPage, setCaptureURL, setResourceType, setMaxBodyBytes, setGroupTitle, setGroupColor, setGroupCollapsed }} />
              <div className="browser-debug-divider" />
              <DebugField label="超时时间"><Input type="number" min="1000" max="300000" step="1000" value={timeoutMS} onChange={(event) => setTimeoutMS(event.target.value)} /></DebugField>
              <DebugField label="复用标识"><Input disabled={!reuseTab || actionType === "tab.navigate"} value={reuseKey} onChange={(event) => setReuseKey(event.target.value)} placeholder="browser-debug" /></DebugField>
              <DebugSwitch label="复用标签页" checked={reuseTab && actionType !== "tab.navigate"} disabled={actionType === "tab.navigate"} onCheckedChange={setReuseTab} />
              <DebugSwitch label="保留标签页" checked={keepTab} onCheckedChange={setKeepTab} />
              <DebugSwitch label="前台打开" checked={activeTab} onCheckedChange={setActiveTab} />
              {actionType !== "tab.group" && <DebugField label="分组名称"><Input value={groupTitle} onChange={(event) => setGroupTitle(event.target.value)} placeholder="留空则不分组" /></DebugField>}
              {missingCapabilities.length > 0 && <div className="browser-debug-capability-warning">节点缺少能力：{missingCapabilities.join("、")}</div>}
            </div>
          </TabsContent>
          <TabsContent value="raw">
            <div className="browser-debug-raw-editor"><div className="browser-debug-raw-toolbar"><span>BrowserRunRequest</span><IconButton title="格式化 JSON" aria-label="格式化 JSON" onClick={formatRawRequest}><Code2 size={14} /></IconButton></div><textarea spellCheck="false" value={rawRequest} onChange={(event) => setRawRequest(event.target.value)} aria-label="浏览器原始调试请求" /></div>
            <div className="browser-debug-raw-agent"><span>默认执行节点</span><Select value={agentID || undefined} onValueChange={setAgentID}><SelectTrigger><SelectValue placeholder="选择在线节点" /></SelectTrigger><SelectContent>{agents.map((agent) => <SelectItem key={agent.id} value={agent.id}>{agent.name || agent.id}</SelectItem>)}</SelectContent></Select></div>
          </TabsContent>
        </Tabs>
        <div className="browser-debug-runbar"><div><ActionIcon size={15} /><span>{mode === "raw" ? "执行动作序列" : selectedAction.label}</span></div><Button variant={actionType === "tab.close" && mode === "quick" ? "destructive" : "default"} disabled={!canRun} onClick={() => void execute()}>{running ? <LoaderCircle className="spin" size={15} /> : <Play size={15} />}{running ? "执行中" : "执行"}</Button></div>
      </Card>

      <Card className="browser-debug-output">
        <div className="browser-debug-panel-header"><div><ListChecks size={16} /><strong>执行结果</strong></div><div className="browser-debug-output-actions">{result && <Badge tone={result.actions?.every((action) => action.success) ? "success" : "warning"}>{result.actions?.every((action) => action.success) ? "完成" : "部分失败"}</Badge>}<IconButton title={copied ? "已复制" : "复制结果"} aria-label={copied ? "已复制" : "复制结果"} disabled={!result} onClick={() => void copyResult()}>{copied ? <Check size={14} /> : <Clipboard size={14} />}</IconButton><IconButton title="清空结果" aria-label="清空结果" disabled={!result && !runError} onClick={() => { setResult(null); setRunError(""); }}><Eraser size={14} /></IconButton></div></div>
        {!result ? <div className="browser-debug-empty"><SquareTerminal size={22} /><span>{running ? "等待浏览器节点返回结果..." : "尚未执行调试请求"}</span></div> : <>
          <div className="browser-debug-result-meta"><span><small>节点</small><strong>{result.agent_id || "-"}</strong></span><span><small>标签页</small><strong>{result.tab_id || "已关闭"}</strong></span><span><small>耗时</small><strong>{result.duration_ms || 0} ms</strong></span></div>
          <Tabs value={resultTab} onValueChange={setResultTab} className="browser-debug-result-tabs"><TabsList><TabsTrigger value="actions">动作 {result.actions?.length || 0}</TabsTrigger><TabsTrigger value="network">网络 {result.network?.length || 0}</TabsTrigger><TabsTrigger value="json">JSON</TabsTrigger></TabsList><TabsContent value="actions"><ActionResults actions={result.actions || []} /></TabsContent><TabsContent value="network"><NetworkResults items={result.network || []} /></TabsContent><TabsContent value="json"><pre className="browser-debug-json">{JSON.stringify(result, null, 2)}</pre></TabsContent></Tabs>
        </>}
      </Card>
    </div>
  </div>;
}

function CapabilityOption({ item, capabilities }) {
  const Icon = item.icon;
  const capability = item.value === "network.capture" ? "network.capture" : item.value;
  return <SelectItem value={item.value} disabled={capabilities.size > 0 && !capabilities.has(capability)}><span className="browser-debug-option"><Icon size={13} />{item.label}</span></SelectItem>;
}

function DebugField({ label, wide = false, children }) {
  return <label className={`browser-debug-field${wide ? " is-wide" : ""}`}><span>{label}</span>{children}</label>;
}

function DebugSwitch({ label, checked, disabled = false, onCheckedChange }) {
  return <label className="browser-debug-switch"><span>{label}</span><Switch checked={checked} disabled={disabled} onCheckedChange={onCheckedChange} aria-label={label} /></label>;
}

function ActionFields({ type, values, setters }) {
  if (["dom.query", "dom.click", "dom.input"].includes(type)) return <><DebugField label="CSS Selector" wide><Input value={values.selector} onChange={(event) => setters.setSelector(event.target.value)} placeholder="#app, main, [data-id]" /></DebugField>{type === "dom.input" && <DebugField label="输入内容" wide><textarea className="browser-debug-textarea" value={values.inputValue} onChange={(event) => setters.setInputValue(event.target.value)} /></DebugField>}{type === "dom.query" && <DebugField label="最大返回长度"><Input type="number" min="256" max="1048576" value={values.maxLength} onChange={(event) => setters.setMaxLength(event.target.value)} /></DebugField>}</>;
  if (type === "dom.document") return <DebugField label="最大 HTML 长度" wide><Input type="number" min="1024" max="1048576" value={values.maxLength} onChange={(event) => setters.setMaxLength(event.target.value)} /></DebugField>;
  if (type === "runtime.evaluate") return <DebugField label="JavaScript 表达式" wide><textarea className="browser-debug-textarea is-code" value={values.expression} onChange={(event) => setters.setExpression(event.target.value)} /></DebugField>;
  if (type === "tab.navigate") return <DebugField label="导航地址" wide><Input type="url" value={values.navigateURL} onChange={(event) => setters.setNavigateURL(event.target.value)} /></DebugField>;
  if (type === "tab.group") return <><DebugField label="分组名称" wide><Input value={values.groupTitle} onChange={(event) => setters.setGroupTitle(event.target.value)} /></DebugField><DebugField label="分组颜色"><Select value={values.groupColor} onValueChange={setters.setGroupColor}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{["grey", "blue", "red", "yellow", "green", "pink", "purple", "cyan", "orange"].map((color) => <SelectItem key={color} value={color}>{color}</SelectItem>)}</SelectContent></Select></DebugField><DebugSwitch label="折叠分组" checked={values.groupCollapsed} onCheckedChange={setters.setGroupCollapsed} /></>;
  if (type === "page.wait") return <><DebugField label="等待方式"><Select value={values.waitMode} onValueChange={setters.setWaitMode}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="load">页面加载</SelectItem><SelectItem value="selector">CSS 元素</SelectItem><SelectItem value="duration">固定时长</SelectItem></SelectContent></Select></DebugField>{values.waitMode === "selector" && <DebugField label="CSS Selector"><Input value={values.selector} onChange={(event) => setters.setSelector(event.target.value)} /></DebugField>}{values.waitMode === "duration" && <DebugField label="等待毫秒"><Input type="number" min="0" value={values.waitDuration} onChange={(event) => setters.setWaitDuration(event.target.value)} /></DebugField>}</>;
  if (type === "page.screenshot") return <><DebugField label="图片格式"><Select value={values.screenshotFormat} onValueChange={setters.setScreenshotFormat}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="png">PNG</SelectItem><SelectItem value="jpeg">JPEG</SelectItem></SelectContent></Select></DebugField>{values.screenshotFormat === "jpeg" && <DebugField label="JPEG 质量"><Input type="number" min="1" max="100" value={values.screenshotQuality} onChange={(event) => setters.setScreenshotQuality(event.target.value)} /></DebugField>}<DebugSwitch label="完整页面" checked={values.fullPage} onCheckedChange={setters.setFullPage} /></>;
  if (type === "network.capture") return <><DebugField label="URL 包含文本" wide><Input value={values.captureURL} onChange={(event) => setters.setCaptureURL(event.target.value)} placeholder="留空匹配全部请求" /></DebugField><DebugField label="资源类型"><Select value={values.resourceType || "all"} onValueChange={(value) => setters.setResourceType(value === "all" ? "" : value)}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部</SelectItem>{["Document", "XHR", "Fetch", "Script", "Stylesheet", "Image", "Media", "Font", "Other"].map((value) => <SelectItem key={value} value={value}>{value}</SelectItem>)}</SelectContent></Select></DebugField><DebugField label="最大正文长度"><Input type="number" min="1024" max="1048576" value={values.maxBodyBytes} onChange={(event) => setters.setMaxBodyBytes(event.target.value)} /></DebugField><DebugField label="捕获等待毫秒"><Input type="number" min="0" value={values.waitDuration} onChange={(event) => setters.setWaitDuration(event.target.value)} /></DebugField></>;
  return null;
}

function quickAction(type, values) {
  const common = { id: "action", type, continue_on_error: true };
  if (type === "tab.open") return null;
  if (type === "tab.navigate") return { ...common, params: { url: values.navigateURL.trim() } };
  if (type === "tab.group") return { ...common, params: { title: values.groupTitle.trim() || "Meerkit Debug", color: values.groupColor, collapsed: values.groupCollapsed, reuse_group: true } };
  if (type === "tab.close") return common;
  if (type === "page.wait") return { ...common, params: values.waitMode === "selector" ? { selector: values.selector, timeout_ms: Number(values.waitDuration) || 60000 } : values.waitMode === "duration" ? { duration_ms: Number(values.waitDuration) || 0 } : {} };
  if (type === "page.screenshot") return { ...common, params: { format: values.screenshotFormat, quality: Number(values.screenshotQuality) || 90, full_page: values.fullPage } };
  if (type === "dom.document") return { ...common, params: { max_length: Number(values.maxLength) || 262144 } };
  if (type === "dom.query") return { ...common, params: { selector: values.selector, max_length: Number(values.maxLength) || 65536 } };
  if (type === "dom.click") return { ...common, params: { selector: values.selector } };
  if (type === "dom.input") return { ...common, params: { selector: values.selector, value: values.inputValue } };
  if (type === "runtime.evaluate") return { ...common, params: { expression: values.expression } };
  if (type === "network.capture") return { id: "settle", type: "page.wait", params: { duration_ms: Number(values.waitDuration) || 1000 }, continue_on_error: true };
  return null;
}

function ActionResults({ actions }) {
  const screenshot = actions.find((action) => action.data?.data_url)?.data?.data_url;
  return <div className="browser-debug-action-results">{screenshot && <div className="browser-debug-screenshot"><img src={screenshot} alt="浏览器调试截图" /></div>}{actions.map((action, index) => <section key={`${action.id}-${index}`} className="browser-debug-action-result" data-success={action.success}><div><span className="browser-debug-result-icon">{action.success ? <Check size={13} /> : <X size={13} />}</span><strong>{action.id || `action-${index + 1}`}</strong><code>{action.type}</code><small>{action.duration_ms || 0} ms</small></div>{action.error ? <pre className="is-error">{action.error}</pre> : action.data && Object.keys(action.data).length > 0 ? <pre>{JSON.stringify(withoutImageData(action.data), null, 2)}</pre> : null}</section>)}</div>;
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
      <div className="browser-network-table">
        <div className="browser-network-head"><span>状态</span><span>名称</span><span>方法</span><span>类型</span><span>大小</span><span>耗时</span></div>
        <div className="browser-network-rows">{filtered.length ? filtered.map(({ item, index }) => <button type="button" key={`${item.capture_id}-${item.url}-${index}`} className="browser-network-row" data-selected={selectedEntry?.index === index} data-error={item.status >= 400 || Boolean(item.error)} onClick={() => setSelectedIndex(index)}><span>{item.status || "ERR"}</span><span><strong title={item.url}>{requestName(item.url)}</strong><small>{requestHost(item.url)}</small></span><span>{item.method || "GET"}</span><span>{item.resource_type || item.mime_type || "-"}</span><span>{formatBytes(item.encoded_data_length || bodyLength(item))}</span><span className="browser-network-waterfall"><i style={{ width: `${Math.max(4, Math.round(((Number(item.duration_ms) || 0) / longestDuration) * 100))}%` }} /><em>{item.duration_ms || 0} ms</em></span></button>) : <div className="browser-network-no-match">没有匹配的请求</div>}</div>
      </div>
    </div>
    <div className="browser-network-detail-pane">{selected ? <>
      <div className="browser-network-detail-title"><div><Badge tone={selected.status >= 400 || selected.error ? "warning" : "success"}>{selected.status || "ERR"}</Badge><strong title={selected.url}>{selected.method || "GET"} {selected.url}</strong></div><span>{selected.protocol || "-"} · {formatBytes(selected.encoded_data_length || bodyLength(selected))} · {selected.duration_ms || 0} ms</span></div>
      <Tabs value={detailTab} onValueChange={setDetailTab} className="browser-network-detail-tabs"><TabsList><TabsTrigger value="headers">标头</TabsTrigger><TabsTrigger value="payload">载荷</TabsTrigger><TabsTrigger value="response">响应</TabsTrigger><TabsTrigger value="timing">时序</TabsTrigger></TabsList>
        <TabsContent value="headers"><NetworkHeaders item={selected} /></TabsContent>
        <TabsContent value="payload"><NetworkPayload item={selected} /></TabsContent>
        <TabsContent value="response"><NetworkResponse item={selected} /></TabsContent>
        <TabsContent value="timing"><NetworkTiming item={selected} /></TabsContent>
      </Tabs>
    </> : <div className="browser-debug-tab-empty">请选择一个请求</div>}</div>
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

function DetailSection({ title, children }) {
  return <section className="browser-network-detail-section"><h4>{title}</h4>{children}</section>;
}

function DetailRows({ entries, empty }) {
  if (!entries.length) return <div className="browser-network-detail-empty">{empty}</div>;
  return <dl>{entries.map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{String(value ?? "")}</dd></div>)}</dl>;
}

function requestName(value) {
  try { const url = new URL(value); return url.pathname.split("/").filter(Boolean).pop() || url.hostname; }
  catch { return value || "-"; }
}

function requestHost(value) {
  try { return new URL(value).host; } catch { return ""; }
}

function bodyLength(item) {
  return new TextEncoder().encode(String(item?.body || "")).length;
}

function formatBytes(value) {
  const bytes = Number(value) || 0;
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(bytes < 10240 ? 1 : 0)} KB`;
  return `${(bytes / 1048576).toFixed(1)} MB`;
}

function formatBody(value, contentType = "") {
  if (!String(contentType).toLowerCase().includes("json")) return value;
  try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
}

function withoutImageData(data) {
  if (!data?.data_url) return data;
  return { ...data, data_url: `[image data: ${data.data_url.length} characters]` };
}
