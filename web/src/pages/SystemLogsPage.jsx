import React, { useEffect, useMemo, useRef, useState } from "react";
import { Check, Clipboard, RefreshCw, ScrollText, Terminal } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Card } from "../components/ui/Card";
import { IconButton } from "../components/ui/IconButton";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "../components/ui/Select";
import { Switch } from "../components/ui/Switch";
import { api, apiText } from "../lib/api";

const systemSources = [
  { id: "business", group: "system", label: "主应用", detail: "meerkit.log", path: "/api/v1/system/logs?source=business", streamPath: "/api/v1/system/logs/stream?source=business" },
  { id: "access", group: "system", label: "HTTP 访问", detail: "meerkit-access.log", path: "/api/v1/system/logs?source=access", streamPath: "/api/v1/system/logs/stream?source=access" }
];

export function SystemLogsPage() {
  const [plugins, setPlugins] = useState([]);
  const [sourceID, setSourceID] = useState("business");
  const [logs, setLogs] = useState("");
  const [error, setError] = useState("");
  const [sourceError, setSourceError] = useState("");
  const [live, setLive] = useState(true);
  const [follow, setFollow] = useState(true);
  const [connection, setConnection] = useState("connecting");
  const [copied, setCopied] = useState(false);
  const outputRef = useRef(null);

  useEffect(() => {
    let cancelled = false;
    api("/api/v1/plugins?page=1&page_size=100")
      .then((response) => { if (!cancelled) setPlugins(response?.items || []); })
      .catch((loadError) => { if (!cancelled) setSourceError(loadError.message); });
    return () => { cancelled = true; };
  }, []);

  const sources = useMemo(() => [
    ...systemSources,
    ...plugins.flatMap((plugin) => (plugin.modules || []).map((module) => ({
      id: `plugin:${plugin.id}:${plugin.version}:${module.type}`,
      group: "plugin",
      label: module.name || module.type,
      detail: `${plugin.name} · ${module.type}`,
      path: `/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}/logs`,
      streamPath: `/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}/logs/stream`,
      moduleType: (plugin.modules || []).length > 1 ? module.type : ""
    })))
  ], [plugins]);
  const source = sources.find((item) => item.id === sourceID) || systemSources[0];
  const visibleLogs = useMemo(() => filterModuleLogs(logs, source.moduleType), [logs, source.moduleType]);

  useEffect(() => {
    setLogs("");
    setError("");
    if (!live) {
      setConnection("paused");
      return undefined;
    }
    setConnection("connecting");
    const eventSource = new EventSource(source.streamPath);
    const snapshot = (event) => {
      setLogs(event.data);
      setError("");
      setConnection("connected");
    };
    const logError = (event) => {
      try { setError(JSON.parse(event.data)?.message || "日志暂不可用"); }
      catch { setError("日志暂不可用"); }
      setConnection("waiting");
    };
    eventSource.onopen = () => setConnection("connected");
    eventSource.onerror = () => setConnection("reconnecting");
    eventSource.addEventListener("snapshot", snapshot);
    eventSource.addEventListener("log-error", logError);
    return () => eventSource.close();
  }, [live, source.id, source.streamPath]);

  useEffect(() => {
    if (follow && outputRef.current) outputRef.current.scrollTop = outputRef.current.scrollHeight;
  }, [follow, visibleLogs]);

  const refresh = async () => {
    setError("");
    try { setLogs(await apiText(source.path)); }
    catch (refreshError) { setError(refreshError.message); }
  };
  const copy = async () => {
    if (!visibleLogs) return;
    try {
      await navigator.clipboard.writeText(visibleLogs);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      setError("浏览器未允许复制日志");
    }
  };
  const connectionMeta = ({ connected: ["实时", "success"], connecting: ["连接中", "warning"], reconnecting: ["重连中", "warning"], waiting: ["等待日志", "muted"], paused: ["已暂停", "muted"] })[connection] || ["未知", "muted"];

  return <div className="page-stack system-logs-page">
    <PageHeader eyebrow="SYSTEM LOGS" title="日志" description="集中查看主应用、HTTP 访问和插件模块的最近日志。" />
    <Card className="system-log-card">
      <div className="system-log-toolbar">
        <div className="system-log-source"><ScrollText size={16} /><Select value={source.id} onValueChange={setSourceID}><SelectTrigger aria-label="选择日志来源"><SelectValue>{source.label}</SelectValue></SelectTrigger><SelectContent className="system-log-source-menu"><SelectGroup><SelectLabel>系统</SelectLabel>{sources.filter((item) => item.group === "system").map((item) => <SelectItem key={item.id} value={item.id}>{item.label}</SelectItem>)}</SelectGroup>{sources.some((item) => item.group === "plugin") && <SelectGroup><SelectLabel>插件模块</SelectLabel>{sources.filter((item) => item.group === "plugin").map((item) => <SelectItem key={item.id} value={item.id}>{item.label} · {item.detail}</SelectItem>)}</SelectGroup>}</SelectContent></Select><span title={source.detail}>{source.detail}</span></div>
        <Badge tone={connectionMeta[1]}><span className="status-dot" />{connectionMeta[0]}</Badge>
        <div className="system-log-switches"><label><Switch checked={live} onCheckedChange={setLive} aria-label="实时追踪日志" /><span>实时追踪</span></label><label><Switch checked={follow} onCheckedChange={setFollow} aria-label="自动滚动到最新日志" /><span>跟随底部</span></label></div>
        <div className="system-log-actions"><IconButton variant="outline" size="default" title="刷新日志" aria-label="刷新日志" onClick={() => { void refresh(); }}><RefreshCw size={15} /></IconButton><IconButton variant="outline" size="default" title={copied ? "已复制" : "复制日志"} aria-label={copied ? "已复制" : "复制日志"} disabled={!visibleLogs} onClick={() => { void copy(); }}>{copied ? <Check size={15} /> : <Clipboard size={15} />}</IconButton></div>
      </div>
      {(sourceError || error) && <div className="system-log-error">{error || `插件日志来源加载失败：${sourceError}`}</div>}
      <pre ref={outputRef} className="plugin-log-output system-log-output" tabIndex={0}>{visibleLogs || <span className="plugin-log-empty"><Terminal size={17} />{logs && source.moduleType ? "当前模块暂无日志" : "等待日志输出..."}</span>}</pre>
    </Card>
  </div>;
}

function filterModuleLogs(logs, moduleType) {
  if (!moduleType || !logs) return logs;
  return logs.split("\n").filter((line) => logLineMatchesModule(line, moduleType)).join("\n");
}

function logLineMatchesModule(line, moduleType) {
  try {
    const value = JSON.parse(line);
    if (value?.module_type === moduleType) return true;
  } catch { /* Text and simple log formats are matched below. */ }
  return line.includes(`module_type=${moduleType}`) || line.includes(`module_type="${moduleType}"`);
}
