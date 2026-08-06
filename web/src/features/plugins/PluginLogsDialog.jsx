import React, { useEffect, useRef, useState } from "react";
import { Check, Clipboard, RefreshCw, Terminal } from "lucide-react";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { IconButton } from "../../components/ui/IconButton";
import { Switch } from "../../components/ui/Switch";
import { apiText } from "../../lib/api";

export function PluginLogsDialog({ plugin, onClose }) {
  const [logs, setLogs] = useState("");
  const [error, setError] = useState("");
  const [live, setLive] = useState(true);
  const [follow, setFollow] = useState(true);
  const [connection, setConnection] = useState("connecting");
  const [copied, setCopied] = useState(false);
  const outputRef = useRef(null);
  const path = plugin ? `/api/v1/plugins/${encodeURIComponent(plugin.id)}/${encodeURIComponent(plugin.version)}/logs` : "";

  useEffect(() => {
    if (!plugin || !live) {
      setConnection("paused");
      return undefined;
    }
    setConnection("connecting");
    const source = new EventSource(`${path}/stream`);
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
    source.onopen = () => setConnection("connected");
    source.onerror = () => setConnection("reconnecting");
    source.addEventListener("snapshot", snapshot);
    source.addEventListener("log-error", logError);
    return () => source.close();
  }, [live, path, plugin]);

  useEffect(() => {
    if (follow && outputRef.current) outputRef.current.scrollTop = outputRef.current.scrollHeight;
  }, [follow, logs]);

  const refresh = async () => {
    setError("");
    try { setLogs(await apiText(path)); }
    catch (refreshError) { setError(refreshError.message); }
  };
  const copy = async () => {
    if (!logs) return;
    try {
      await navigator.clipboard.writeText(logs);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      setError("浏览器未允许复制日志");
    }
  };
  const connectionMeta = ({ connected: ["实时", "success"], connecting: ["连接中", "warning"], reconnecting: ["重连中", "warning"], waiting: ["等待日志", "muted"], paused: ["已暂停", "muted"] })[connection] || ["未知", "muted"];

  return <Dialog open={Boolean(plugin)} onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="modal-wide plugin-log-dialog">
      <DialogHeader><div className="plugin-dialog-heading"><span className="eyebrow">PLUGIN LOGS</span><DialogTitle>{plugin?.name} 日志</DialogTitle><DialogDescription>{plugin?.id} · {plugin?.version}</DialogDescription><div className="plugin-heading-badges"><Badge tone={connectionMeta[1]}><span className="status-dot" />{connectionMeta[0]}</Badge></div></div></DialogHeader>
      <div className="modal-body plugin-log-body">
        <div className="plugin-log-toolbar"><label><Switch checked={live} onCheckedChange={setLive} aria-label="实时追踪日志" /><span>实时追踪</span></label><label><Switch checked={follow} onCheckedChange={setFollow} aria-label="自动滚动到最新日志" /><span>跟随底部</span></label><span className="plugin-log-toolbar-spacer" /><IconButton variant="outline" size="sm" title="刷新日志" aria-label="刷新日志" onClick={() => { void refresh(); }}><RefreshCw size={14} /></IconButton><IconButton variant="outline" size="sm" title={copied ? "已复制" : "复制日志"} aria-label={copied ? "已复制" : "复制日志"} disabled={!logs} onClick={() => { void copy(); }}>{copied ? <Check size={14} /> : <Clipboard size={14} />}</IconButton></div>
        {error && <div className="plugin-log-error">{error}</div>}
        <pre ref={outputRef} className="plugin-log-output" tabIndex={0}>{logs || <span className="plugin-log-empty"><Terminal size={17} />等待插件输出日志...</span>}</pre>
      </div>
      <DialogFooter><Button type="button" variant="outline" onClick={onClose}>关闭</Button></DialogFooter>
    </DialogContent>
  </Dialog>;
}
