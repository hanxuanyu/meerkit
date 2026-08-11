import React, { useCallback, useEffect, useState } from "react";
import { LoaderCircle, RadioTower, RefreshCw, ShieldCheck } from "lucide-react";
import { StatusBoardSnapshotView } from "./StatusBoardPage";
import { ThemeSwitcher } from "../components/theme/ThemeSwitcher";
import { IconButton } from "../components/ui/IconButton";
import { api } from "../lib/api";

const PUBLIC_REFRESH_INTERVAL = 15000;

export function PublicStatusBoardPage({ token }) {
  const [snapshot, setSnapshot] = useState(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async ({ quiet = false } = {}) => {
    if (quiet) setRefreshing(true); else setLoading(true);
    try {
      const result = await api(`/api/v1/public/status-board/${encodeURIComponent(token)}`);
      setSnapshot(result);
      setError("");
    } catch (loadError) { setError(loadError.message); }
    finally { setLoading(false); setRefreshing(false); }
  }, [token]);

  useEffect(() => {
    void load();
    const interval = window.setInterval(() => { if (document.visibilityState === "visible") void load({ quiet: true }); }, PUBLIC_REFRESH_INTERVAL);
    return () => window.clearInterval(interval);
  }, [load]);

  useEffect(() => {
    const previous = document.title;
    if (snapshot?.name) document.title = `${snapshot.name} · Meerkit`;
    return () => { document.title = previous; };
  }, [snapshot?.name]);

  return <main className="public-status-page">
    <header className="public-status-header"><div className="public-status-brand"><img src="/brand-mark.png" alt="" /><span><strong>Meerkit</strong></span></div><div className="public-status-spacer" aria-hidden="true" /><div className="public-status-refresh"><span>{snapshot?.generated_at ? `更新于 ${formatPublicTime(snapshot.generated_at)}` : "正在获取状态"}</span><ThemeSwitcher /><IconButton variant="outline" size="default" title="刷新状态" aria-label="刷新状态" disabled={refreshing} onClick={() => void load({ quiet: true })}>{refreshing ? <LoaderCircle className="spin" size={15} /> : <RefreshCw size={15} />}</IconButton></div></header>
    <section className="public-status-content">{loading ? <div className="public-status-state"><LoaderCircle className="spin" size={22} /><strong>正在加载状态看板</strong></div> : error ? <div className="public-status-state is-error"><ShieldCheck size={22} /><strong>{error}</strong><span>请向分享者确认链接是否仍然有效。</span></div> : snapshot?.groups?.length ? <StatusBoardSnapshotView snapshot={snapshot} readOnly /> : <div className="public-status-state"><RadioTower size={22} /><strong>当前分享范围内没有看板项</strong></div>}</section>
  </main>;
}

function formatPublicTime(value) {
  return new Date(value).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}
