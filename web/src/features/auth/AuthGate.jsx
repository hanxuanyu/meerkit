import React, { useEffect, useState } from "react";
import { KeyRound, LoaderCircle, ShieldCheck } from "lucide-react";
import { App } from "../../app/App";
import { Button } from "../../components/ui/Button";
import { Input, Label } from "../../components/ui/Input";
import { api, setCSRFToken } from "../../lib/api";

export function AuthGate() {
  const [state, setState] = useState("loading");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    api("/api/v1/auth/status").then(async ({ initialized }) => {
      if (!initialized) { if (active) setState("setup"); return; }
      try { const session = await api("/api/v1/auth/session"); setCSRFToken(session.csrf_token); if (active) setState("authenticated"); }
      catch { if (active) setState("login"); }
    }).catch((loadError) => { if (active) { setError(loadError.message); setState("login"); } });
    const unauthorized = () => { setCSRFToken(""); setState("login"); };
    window.addEventListener("meerkit:unauthorized", unauthorized);
    return () => { active = false; window.removeEventListener("meerkit:unauthorized", unauthorized); };
  }, []);
  if (state === "authenticated") return <App />;
  if (state === "loading") return <div className="auth-loading"><LoaderCircle className="spin" size={24} /></div>;
  return <AuthForm mode={state} busy={busy} error={error} onSubmit={async (values) => {
    setBusy(true); setError("");
    try { const session = await api(state === "setup" ? "/api/v1/auth/setup" : "/api/v1/auth/login", { method: "POST", body: JSON.stringify(values) }); setCSRFToken(session.csrf_token); setState("authenticated"); }
    catch (submitError) { setError(submitError.message); }
    finally { setBusy(false); }
  }} />;
}

function AuthForm({ mode, busy, error, onSubmit }) {
  const [accessKey, setAccessKey] = useState("");
  const [confirm, setConfirm] = useState("");
  const setup = mode === "setup";
  return <main className="auth-screen"><section className="auth-panel">
    <div className="auth-brand"><img src="/brand-mark.png" alt="Meerkit" /><div><strong>Meerkit</strong><span>observability</span></div></div>
    <div className="auth-heading">{setup ? <ShieldCheck size={22} /> : <KeyRound size={22} />}<div><h1>{setup ? "初始化管理员" : "管理员登录"}</h1><p>{setup ? "设置用于访问 Meerkit 管理界面的密钥。" : "输入管理员访问密钥继续。"}</p></div></div>
    <form onSubmit={(event) => { event.preventDefault(); void onSubmit(setup ? { access_key: accessKey, confirm } : { access_key: accessKey }); }}>
      <label><Label>访问密钥</Label><Input type="password" autoFocus minLength={12} required value={accessKey} onChange={(event) => setAccessKey(event.target.value)} /></label>
      {setup && <label><Label>确认访问密钥</Label><Input type="password" minLength={12} required value={confirm} onChange={(event) => setConfirm(event.target.value)} /></label>}
      {error && <p className="auth-error">{error}</p>}
      <Button type="submit" disabled={busy || (setup && accessKey !== confirm)}>{busy && <LoaderCircle className="spin" size={15} />}{setup ? "完成初始化" : "登录"}</Button>
    </form>
  </section></main>;
}
