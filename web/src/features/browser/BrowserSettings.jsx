import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Check, Copy, Laptop, RefreshCw, RotateCcw, Unplug, Wifi } from "lucide-react";
import { toast } from "sonner";
import { api } from "../../lib/api";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { IconButton } from "../../components/ui/IconButton";
import { PasswordInput } from "../../components/ui/PasswordInput";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "../../components/ui/AlertDialog";

const pollIntervalMS = 5000;

export function BrowserSettings() {
  const [status, setStatus] = useState(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [rotating, setRotating] = useState(false);
  const [confirmRotate, setConfirmRotate] = useState(false);
  const [copied, setCopied] = useState("");

  const load = useCallback(async ({ quiet = false } = {}) => {
    if (!quiet) setLoading(true);
    try {
      const value = await api("/api/v1/browser");
      setStatus(value);
      setError("");
    } catch (loadError) {
      setError(loadError.message);
    } finally {
      if (!quiet) setLoading(false);
    }
  }, []);

  useEffect(() => {
    let active = true;
    const refresh = async (quiet) => {
      if (!active || document.visibilityState !== "visible") return;
      await load({ quiet });
    };
    void refresh(false);
    const timer = window.setInterval(() => void refresh(true), pollIntervalMS);
    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") void refresh(true);
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => {
      active = false;
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, [load]);

  const websocketURL = useMemo(() => buildWebSocketURL(status?.websocket_path), [status?.websocket_path]);
  const agents = status?.connected_agents || [];

  const copy = async (key, value) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
      window.setTimeout(() => setCopied((current) => current === key ? "" : current), 1400);
      toast.success(key === "token" ? "配对令牌已复制" : "WebSocket 地址已复制");
    } catch {
      toast.error("浏览器未允许写入剪贴板");
    }
  };

  const rotateToken = async () => {
    setRotating(true);
    try {
      const value = await api("/api/v1/browser/pairing-token/rotate", { method: "POST" });
      setStatus((current) => ({ ...current, pairing_token: value.pairing_token, connected_agents: [] }));
      setConfirmRotate(false);
      toast.success("配对令牌已轮换", { description: "已断开所有浏览器执行节点。" });
    } catch (rotateError) {
      toast.error(rotateError.message);
    } finally {
      setRotating(false);
    }
  };

  return <>
    <section className="browser-settings">
      <div className="section-header"><div><h2>浏览器执行节点</h2><p>管理 Meerkit Browser Agent 与浏览器自动化能力。</p></div><div className="browser-settings-status"><Badge tone={agents.length ? "success" : "muted"}>{agents.length ? `${agents.length} 个在线` : "暂无在线节点"}</Badge><IconButton variant="outline" title="刷新浏览器节点" aria-label="刷新浏览器节点" disabled={loading} onClick={() => void load()}><RefreshCw className={loading ? "spin" : ""} size={14} /></IconButton></div></div>
      {error && <div className="browser-settings-error">{error}</div>}
      <div className="browser-settings-body">
        <section className="browser-connection-panel">
          <div className="browser-settings-subheading"><div><strong>连接信息</strong><span>协议版本 {status?.protocol || "-"}</span></div><Button variant="outline" size="sm" disabled={!status || rotating} onClick={() => setConfirmRotate(true)}><RotateCcw size={14} />轮换令牌</Button></div>
          <div className="browser-connection-fields">
            <label><span>WebSocket 地址</span><div><code title={websocketURL}>{websocketURL || "-"}</code><IconButton variant="ghost" title="复制 WebSocket 地址" aria-label="复制 WebSocket 地址" disabled={!websocketURL} onClick={() => void copy("endpoint", websocketURL)}>{copied === "endpoint" ? <Check size={14} /> : <Copy size={14} />}</IconButton></div></label>
            <label><span>配对令牌</span><div><PasswordInput readOnly value={status?.pairing_token || ""} aria-label="浏览器执行节点配对令牌" /><IconButton variant="ghost" title="复制配对令牌" aria-label="复制配对令牌" disabled={!status?.pairing_token} onClick={() => void copy("token", status.pairing_token)}>{copied === "token" ? <Check size={14} /> : <Copy size={14} />}</IconButton></div></label>
          </div>
        </section>
        <section className="browser-agents-panel">
          <div className="browser-settings-subheading"><div><strong>在线节点</strong><span>{agents.length ? "自动刷新连接与心跳状态" : "扩展连接后将在此显示"}</span></div></div>
          {loading && !status ? <div className="browser-agent-empty"><RefreshCw className="spin" size={16} />正在加载浏览器节点...</div> : agents.length ? <div className="browser-agent-list">{agents.map((agent) => <BrowserAgent key={agent.id} agent={agent} />)}</div> : <div className="browser-agent-empty"><Unplug size={17} />暂无已连接的浏览器执行节点</div>}
        </section>
      </div>
    </section>
    <AlertDialog open={confirmRotate} onOpenChange={(open) => { if (!rotating) setConfirmRotate(open); }}>
      <AlertDialogContent>
        <AlertDialogHeader><div className="alert-dialog-icon"><RotateCcw size={18} /></div><AlertDialogTitle>确认轮换配对令牌？</AlertDialogTitle><AlertDialogDescription>所有已连接的浏览器执行节点会立即断开。扩展需要保存新的配对令牌后才能重新连接。</AlertDialogDescription></AlertDialogHeader>
        <AlertDialogFooter><AlertDialogCancel disabled={rotating}>取消</AlertDialogCancel><AlertDialogAction disabled={rotating} onClick={(event) => { event.preventDefault(); void rotateToken(); }}>{rotating ? "轮换中..." : "确认轮换"}</AlertDialogAction></AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </>;
}

function BrowserAgent({ agent }) {
  const capabilities = agent.capabilities || [];
  return <article className="browser-agent-row">
    <span className="browser-agent-icon"><Laptop size={16} /></span>
    <div className="browser-agent-copy"><div><strong title={agent.name}>{agent.name || "未命名节点"}</strong><Badge tone="success"><Wifi size={10} />在线</Badge></div><code title={agent.id}>{agent.id}</code></div>
    <div className="browser-agent-version"><span>扩展版本</span><strong>{agent.version || "-"}</strong></div>
    <div className="browser-agent-capabilities"><span>能力</span><strong>{capabilities.length}</strong><small title={capabilities.join(", ")}>{capabilities.join(" · ") || "-"}</small></div>
    <div className="browser-agent-seen"><span>最近心跳</span><strong>{formatRelativeTime(agent.last_seen_at)}</strong><small>{formatDateTime(agent.connected_at)} 接入</small></div>
  </article>;
}

function buildWebSocketURL(path) {
  if (!path || typeof window === "undefined") return "";
  const url = new URL(path, window.location.origin);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}

function formatDateTime(value) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }).format(date);
}

function formatRelativeTime(value) {
  if (!value) return "-";
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
  if (!Number.isFinite(seconds)) return "-";
  if (seconds < 10) return "刚刚";
  if (seconds < 60) return `${seconds} 秒前`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`;
  return formatDateTime(value);
}
