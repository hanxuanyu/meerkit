import React, { useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, ChevronUp, Circle, CircleStop, Copy, Download, Filter, Search, Trash2, X } from "lucide-react";
import { toast } from "sonner";
import { Card } from "../../components/ui/Card";
import { Checkbox } from "../../components/ui/Checkbox";
import { IconButton } from "../../components/ui/IconButton";
import { Input } from "../../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../components/ui/Tabs";

const resourceFilters = [
  ["all", "全部"], ["fetch", "Fetch/XHR"], ["document", "文档"], ["css", "CSS"], ["js", "JS"],
  ["font", "字体"], ["image", "图片"], ["media", "媒体"], ["manifest", "Manifest"], ["ws", "WS"], ["wasm", "Wasm"], ["other", "其他"]
];

const columns = [
  ["name", "名称"], ["status", "状态"], ["method", "方法"], ["type", "类型"],
  ["initiator", "发起程序"], ["size", "大小"], ["time", "耗时"]
];

export function NetworkPanel({ capture, items, selected, onSelect, onStart, onStop, onClear, canStart, targetLabel }) {
  const [query, setQuery] = useState("");
  const [resourceFilter, setResourceFilter] = useState("all");
  const [preserveLog, setPreserveLog] = useState(false);
  const [disableCache, setDisableCache] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const [maxBodyBytes, setMaxBodyBytes] = useState("262144");
  const [sort, setSort] = useState({ key: "index", direction: "asc" });
  const [pending, setPending] = useState(false);
  const rowsRef = useRef(null);

  const filteredItems = useMemo(() => {
    const filtered = items.filter((item) => resourceFilter === "all" || resourceGroup(item) === resourceFilter).filter((item) => matchesNetworkQuery(item, query));
    if (sort.key === "index") return filtered;
    return [...filtered].sort((left, right) => compareNetworkItems(left, right, sort) || items.indexOf(left) - items.indexOf(right));
  }, [items, query, resourceFilter, sort]);

  useEffect(() => {
    if (capture && autoScroll) rowsRef.current?.scrollTo({ top: rowsRef.current.scrollHeight });
  }, [autoScroll, capture, items.length]);

  const toggleSort = (key) => setSort((current) => current.key === key
    ? { key, direction: current.direction === "asc" ? "desc" : "asc" }
    : { key, direction: "asc" });
  const toggleCapture = async () => {
    setPending(true);
    try {
      if (capture) await onStop();
      else await onStart({ preserveLog, disableCache, maxBodyBytes: Number(maxBodyBytes) || 262144 });
    } finally { setPending(false); }
  };
  const clear = () => { onClear(); setSort({ key: "index", direction: "asc" }); };
  const totalSize = items.reduce((total, item) => total + (Number(item.encoded_data_length) || 0), 0);
  const visibleSize = filteredItems.reduce((total, item) => total + (Number(item.encoded_data_length) || 0), 0);
  const totalTime = items.reduce((largest, item) => Math.max(largest, Number(item.duration_ms) || 0), 0);

  return <Card className="browser-network-card">
    <div className="browser-network-control">
      <div className="browser-network-tools">
        <IconButton className="browser-network-record" data-recording={Boolean(capture)} title={capture ? "停止录制网络日志" : "开始录制网络日志"} onClick={() => void toggleCapture()} disabled={pending || (!capture && !canStart)}>
          {capture ? <CircleStop size={15} /> : <Circle size={15} fill="currentColor" />}
        </IconButton>
        <IconButton title="清除网络日志" onClick={clear} disabled={!items.length}><Trash2 size={14} /></IconButton>
        <span className="browser-network-divider" />
        <label className="browser-network-search"><Search size={13} /><Input aria-label="筛选网络请求" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="筛选（支持 domain:、method:、status:、mime-type:、larger-than:）" />{query ? <button type="button" aria-label="清除筛选" onClick={() => setQuery("")}><X size={12} /></button> : null}</label>
        <IconButton title="导出 HAR（含敏感标头和正文）" onClick={() => exportHAR(items)} disabled={!items.length}><Download size={14} /></IconButton>
      </div>
      <div className="browser-network-options">
        <NetworkCheck checked={preserveLog} onChange={setPreserveLog} label="保留日志" disabled={Boolean(capture)} />
        <NetworkCheck checked={disableCache} onChange={setDisableCache} label="禁用缓存" disabled={Boolean(capture)} />
        <NetworkCheck checked={autoScroll} onChange={setAutoScroll} label="自动滚动" />
        <Select value={maxBodyBytes} onValueChange={setMaxBodyBytes} disabled={Boolean(capture)}><SelectTrigger className="browser-network-body-limit" title="单个请求/响应正文捕获上限"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="65536">正文 64 KB</SelectItem><SelectItem value="262144">正文 256 KB</SelectItem><SelectItem value="524288">正文 512 KB</SelectItem><SelectItem value="1048576">正文 1 MB</SelectItem></SelectContent></Select>
      </div>
      <span className="browser-network-capture-state" data-recording={Boolean(capture)}>{pending ? capture ? "正在停止捕获…" : "正在启动捕获…" : capture ? `正在录制 · ${targetLabel}` : `已停止 · ${targetLabel || "未选择标签页"}`}</span>
    </div>
    <div className="browser-network-content" data-detail={Boolean(selected)}>
      <div className="browser-network-left-pane">
        <div className="browser-network-filterbar"><Filter size={12} />{resourceFilters.map(([value, label]) => <button type="button" key={value} data-active={resourceFilter === value} onClick={() => setResourceFilter(value)}>{label}</button>)}</div>
        <NetworkList ref={rowsRef} items={filteredItems} allItems={items} selected={selected} onSelect={onSelect} sort={sort} onSort={toggleSort} />
      </div>
      {selected ? <NetworkDetail key={`${selected.session_id}:${items.indexOf(selected)}`} item={selected} onClose={() => onSelect(null)} /> : null}
    </div>
    <div className="browser-network-summary">
      <span>{filteredItems.length === items.length ? `${items.length} 个请求` : `${filteredItems.length} / ${items.length} 个请求`}</span>
      <span>已传输 {formatBytes(visibleSize)}{visibleSize !== totalSize ? ` / ${formatBytes(totalSize)}` : ""}</span>
      <span>完成：{formatDuration(totalTime)}</span>
      {capture ? <span className="is-recording">● 正在录制</span> : null}
    </div>
  </Card>;
}

function NetworkCheck({ checked, onChange, label, disabled = false }) {
  return <label className="browser-network-check"><Checkbox checked={checked} disabled={disabled} onCheckedChange={(value) => onChange(Boolean(value))} /><span>{label}</span></label>;
}

const NetworkList = React.forwardRef(function NetworkList({ items, allItems, selected, onSelect, sort, onSort }, ref) {
  const selectRelative = (direction) => {
    if (!items.length) return;
    const current = items.indexOf(selected);
    const next = current < 0 ? 0 : Math.max(0, Math.min(items.length - 1, current + direction));
    onSelect(items[next]);
  };
  return <div className="browser-network-list-pane">
    <div className="browser-network-list-head">{columns.map(([key, label]) => <button type="button" key={key} onClick={() => onSort(key)} data-sorted={sort.key === key}>{label}{sort.key === key ? sort.direction === "asc" ? <ChevronUp size={10} /> : <ChevronDown size={10} /> : null}</button>)}</div>
    <div className="browser-network-list" ref={ref} tabIndex={0} onKeyDown={(event) => { if (event.key === "ArrowDown") { event.preventDefault(); selectRelative(1); } else if (event.key === "ArrowUp") { event.preventDefault(); selectRelative(-1); } else if (event.key === "Escape") onSelect(null); }}>
      {items.length ? items.map((item) => <NetworkRow item={item} selected={item === selected} key={`${item.session_id}:${allItems.indexOf(item)}`} onSelect={onSelect} />) : <div className="browser-network-empty"><Search size={18} /><strong>{allItems.length ? "没有与筛选条件匹配的请求" : "正在等待网络请求"}</strong><span>{allItems.length ? "尝试清除文本或资源类型筛选" : "开始录制后，在目标标签页中重新加载或执行操作"}</span></div>}
    </div>
  </div>;
});

function NetworkRow({ item, selected, onSelect }) {
  const location = splitURL(item.url);
  const status = item.error ? "(失败)" : item.status || "(待处理)";
  return <button type="button" className="browser-network-list-row" data-selected={selected} data-error={Boolean(item.error || Number(item.status) >= 400)} onClick={() => onSelect(item)} title={item.error || item.url}>
    <span className="browser-network-name"><strong>{location.name}</strong><small>{location.path}</small></span>
    <span>{status}</span><span>{item.method || "GET"}</span><span>{displayResourceType(item)}</span><span>{item.initiator_type || "Other"}</span><span>{formatBytes(item.encoded_data_length)}</span><span>{formatDuration(item.duration_ms)}</span>
  </button>;
}

function NetworkDetail({ item, onClose }) {
  const hasPayload = Boolean(item.request_body);
  const general = {
    "Request URL": item.url,
    "Request Method": item.method || "GET",
    "Status Code": item.error ? item.error : `${item.status || 0}${item.status_text ? ` ${item.status_text}` : ""}`,
    "Remote Address": [item.remote_ip_address, item.remote_port].filter(Boolean).join(":"),
    "Referrer": headerValue(item.request_headers, "referer") || ""
  };
  const query = queryParameters(item.url);
  const timing = { "Total duration": formatDuration(item.duration_ms), Protocol: item.protocol, "From disk cache": item.from_disk_cache, "From service worker": item.from_service_worker, ...(item.timing || {}) };
  return <aside className="browser-network-detail">
    <div className="browser-network-detail-title"><div><strong title={item.url}>{splitURL(item.url).name}</strong><span>{item.status || "失败"} · {displayResourceType(item)} · {formatBytes(item.encoded_data_length)}</span></div><div><IconButton title="复制 URL" onClick={() => copyText(item.url, "已复制请求 URL")}><Copy size={13} /></IconButton><IconButton title="复制为 cURL" onClick={() => copyText(toCurl(item), "已复制 cURL")}><span className="browser-network-curl">cURL</span></IconButton><IconButton title="关闭详情" onClick={onClose}><X size={14} /></IconButton></div></div>
    <Tabs defaultValue="headers">
      <TabsList><TabsTrigger value="headers">标头</TabsTrigger>{hasPayload ? <TabsTrigger value="payload">载荷</TabsTrigger> : null}<TabsTrigger value="preview">预览</TabsTrigger><TabsTrigger value="response">响应</TabsTrigger><TabsTrigger value="initiator">发起程序</TabsTrigger><TabsTrigger value="timing">时间</TabsTrigger></TabsList>
      <TabsContent value="headers"><DetailObject title="常规" value={general} />{query.length ? <DetailEntries title="查询字符串参数" entries={query} /> : null}<DetailObject title="响应标头" value={item.headers} /><DetailObject title="请求标头" value={item.request_headers} /></TabsContent>
      {hasPayload ? <TabsContent value="payload"><DetailBody value={prettyBody(item.request_body, headerValue(item.request_headers, "content-type"))} truncated={item.request_body_truncated} /></TabsContent> : null}
      <TabsContent value="preview"><DetailBody value={previewBody(item)} error={item.error} truncated={item.truncated} /></TabsContent>
      <TabsContent value="response"><DetailBody value={item.body_base64 ? "[Base64 编码的二进制响应，未直接显示]" : item.body} error={item.error} truncated={item.truncated} /></TabsContent>
      <TabsContent value="initiator"><DetailObject title="发起程序" value={{ Type: item.initiator_type || "Other", URL: headerValue(item.request_headers, "referer") || "无可用调用栈信息" }} /></TabsContent>
      <TabsContent value="timing"><DetailObject title="连接与时间原始数据" value={timing} /></TabsContent>
    </Tabs>
  </aside>;
}

function DetailObject({ title, value }) {
  return <DetailEntries title={title} entries={Object.entries(value || {}).filter(([, item]) => item !== undefined && item !== "")} />;
}

function DetailEntries({ title, entries }) {
  return <section className="browser-network-detail-section"><h4>{title}<span>{entries.length}</span></h4>{entries.length ? <dl>{entries.map(([key, value], index) => <div key={`${key}:${index}`}><dt>{key}</dt><dd>{typeof value === "boolean" ? value ? "true" : "false" : String(value)}</dd></div>)}</dl> : <div className="browser-network-detail-empty">无数据</div>}</section>;
}

function DetailBody({ value, error, truncated }) {
  return <section className="browser-network-body"><pre className={error ? "is-error" : ""}>{value || error || "此请求没有可用正文"}</pre>{truncated ? <span>正文已按捕获上限截断</span> : null}</section>;
}

function resourceGroup(item) {
  const type = String(item.resource_type || "").toLowerCase();
  const mime = String(item.mime_type || "").toLowerCase();
  if (["xhr", "fetch"].includes(type)) return "fetch";
  if (type === "document") return "document";
  if (type === "stylesheet" || mime.includes("css")) return "css";
  if (type === "script" || mime.includes("javascript")) return "js";
  if (type === "font") return "font";
  if (type === "image" || mime.startsWith("image/")) return "image";
  if (type === "media" || mime.startsWith("audio/") || mime.startsWith("video/")) return "media";
  if (type === "manifest") return "manifest";
  if (type === "websocket") return "ws";
  if (type === "wasm") return "wasm";
  return "other";
}

function displayResourceType(item) {
  const group = resourceGroup(item);
  return { fetch: String(item.resource_type || "Fetch"), document: "document", css: "stylesheet", js: "script", font: "font", image: "image", media: "media", manifest: "manifest", ws: "websocket", wasm: "wasm", other: item.resource_type || item.mime_type || "other" }[group];
}

function matchesNetworkQuery(item, input) {
  const tokens = input.match(/(?:[^\s"]+|"[^"]*")+/g) || [];
  return tokens.every((rawToken) => {
    let token = rawToken.replace(/^"|"$/g, "");
    const negative = token.startsWith("-");
    if (negative) token = token.slice(1);
    const separator = token.indexOf(":");
    const key = separator > 0 ? token.slice(0, separator).toLowerCase() : "";
    const expected = (separator > 0 ? token.slice(separator + 1) : token).toLowerCase();
    let matches;
    if (key === "domain") matches = splitURL(item.url).domain.toLowerCase().includes(expected);
    else if (key === "method") matches = String(item.method || "GET").toLowerCase() === expected;
    else if (["status", "status-code"].includes(key)) matches = String(item.status || 0) === expected;
    else if (key === "mime-type") matches = String(item.mime_type || "").toLowerCase().includes(expected);
    else if (["type", "resource-type"].includes(key)) matches = [resourceGroup(item), displayResourceType(item)].some((value) => value.toLowerCase().includes(expected));
    else if (key === "larger-than") matches = Number(item.encoded_data_length) > parseByteSize(expected);
    else if (key === "has-response-header") matches = Object.keys(item.headers || {}).some((name) => name.toLowerCase() === expected);
    else if (key === "is") matches = expected === "from-cache" ? Boolean(item.from_disk_cache) : expected === "service-worker" ? Boolean(item.from_service_worker) : false;
    else matches = [item.url, item.method, item.status, item.status_text, item.resource_type, item.mime_type, item.initiator_type, item.error].some((value) => String(value || "").toLowerCase().includes(expected));
    return negative ? !matches : matches;
  });
}

function compareNetworkItems(left, right, sort) {
  const values = {
    name: (item) => item.url || "", status: (item) => Number(item.status) || 0, method: (item) => item.method || "",
    type: displayResourceType, initiator: (item) => item.initiator_type || "", size: (item) => Number(item.encoded_data_length) || 0,
    time: (item) => Number(item.duration_ms) || 0
  };
  const getValue = values[sort.key] || (() => 0);
  const a = getValue(left); const b = getValue(right);
  const result = typeof a === "number" ? a - b : String(a).localeCompare(String(b), "zh-CN", { numeric: true });
  return sort.direction === "asc" ? result : -result;
}

function splitURL(value) {
  try { const url = new URL(value); return { name: url.pathname.split("/").filter(Boolean).pop() || url.hostname || value, path: `${url.hostname}${url.pathname === "/" ? "" : url.pathname}`, domain: url.hostname }; }
  catch { return { name: value || "(unknown)", path: "", domain: "" }; }
}

function queryParameters(value) {
  try { return [...new URL(value).searchParams.entries()]; } catch { return []; }
}

function headerValue(headers, name) {
  const entry = Object.entries(headers || {}).find(([key]) => key.toLowerCase() === name.toLowerCase());
  return entry?.[1] || "";
}

function prettyBody(value, mime = "") {
  if (!value) return "";
  if (mime.includes("json") || /^[\s]*[\[{]/.test(value)) { try { return JSON.stringify(JSON.parse(value), null, 2); } catch { /* show raw body */ } }
  try { const params = new URLSearchParams(value); if (mime.includes("x-www-form-urlencoded") && [...params].length) return [...params].map(([key, item]) => `${key}: ${item}`).join("\n"); } catch { /* show raw body */ }
  return value;
}

function previewBody(item) {
  if (item.body_base64) return "无法预览 Base64 编码的二进制响应";
  return prettyBody(item.body, item.mime_type);
}

function parseByteSize(value) {
  const match = String(value).trim().match(/^(\d+(?:\.\d+)?)\s*(b|kb|mb)?$/i);
  if (!match) return Number.POSITIVE_INFINITY;
  return Number(match[1]) * ({ b: 1, kb: 1024, mb: 1024 * 1024 }[String(match[2] || "b").toLowerCase()]);
}

function formatBytes(value) {
  const bytes = Number(value) || 0;
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(bytes < 10240 ? 1 : 0)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function formatDuration(value) {
  const milliseconds = Number(value) || 0;
  return milliseconds >= 1000 ? `${(milliseconds / 1000).toFixed(2)} s` : `${milliseconds} ms`;
}

async function copyText(value, success) {
  try { await navigator.clipboard.writeText(value); toast.success(success); } catch { toast.error("复制失败，请检查浏览器剪贴板权限"); }
}

function shellQuote(value) { return `'${String(value).replaceAll("'", `'\\''`)}'`; }
function toCurl(item) {
  const parts = ["curl", shellQuote(item.url), "-X", String(item.method || "GET")];
  Object.entries(item.request_headers || {}).forEach(([key, value]) => parts.push("-H", shellQuote(`${key}: ${value}`)));
  if (item.request_body) parts.push("--data-raw", shellQuote(item.request_body));
  return parts.join(" ");
}

function exportHAR(items) {
  const entries = items.map((item) => ({
    startedDateTime: new Date().toISOString(), time: Number(item.duration_ms) || 0,
    request: { method: item.method || "GET", url: item.url || "", httpVersion: item.protocol || "", headers: harHeaders(item.request_headers), queryString: queryParameters(item.url).map(([name, value]) => ({ name, value })), cookies: [], headersSize: -1, bodySize: item.request_body ? new Blob([item.request_body]).size : 0, ...(item.request_body ? { postData: { mimeType: headerValue(item.request_headers, "content-type"), text: item.request_body } } : {}) },
    response: { status: Number(item.status) || 0, statusText: item.status_text || item.error || "", httpVersion: item.protocol || "", headers: harHeaders(item.headers), cookies: [], content: { size: Number(item.encoded_data_length) || 0, mimeType: item.mime_type || "", text: item.body || "", ...(item.body_base64 ? { encoding: "base64" } : {}) }, redirectURL: headerValue(item.headers, "location"), headersSize: -1, bodySize: Number(item.encoded_data_length) || 0 },
    cache: {}, timings: { send: -1, wait: Number(item.duration_ms) || 0, receive: 0 }, serverIPAddress: item.remote_ip_address || "", connection: item.remote_port ? String(item.remote_port) : ""
  }));
  const blob = new Blob([JSON.stringify({ log: { version: "1.2", creator: { name: "Meerkit", version: "1" }, entries } }, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob); const link = document.createElement("a");
  link.href = url; link.download = `meerkit-network-${new Date().toISOString().replaceAll(":", "-")}.har`; document.body.appendChild(link); link.click(); link.remove(); URL.revokeObjectURL(url);
}

function harHeaders(headers) { return Object.entries(headers || {}).map(([name, value]) => ({ name, value: String(value) })); }
