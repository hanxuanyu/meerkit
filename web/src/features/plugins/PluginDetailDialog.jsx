import React from "react";
import { ExternalLink, KeyRound, LoaderCircle, ShieldAlert, ShieldCheck } from "lucide-react";
import ReactMarkdown from "react-markdown";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "../../components/ui/Dialog";
import { pluginStatusMeta, pluginTrustMeta } from "./pluginPresentation";

export function PluginDetailDialog({ plugin, loading, trusting, onClose, onTrust }) {
  if (!plugin) return null;
  const manifest = plugin.manifest || {};
  const protocol = manifest.protocol || {};
  const status = pluginStatusMeta(plugin.status);
  const trust = pluginTrustMeta(plugin.trust_state);
  return <Dialog open onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="modal-wide plugin-detail-dialog">
      <DialogHeader><div className="plugin-dialog-heading"><span className="eyebrow">PLUGIN DETAIL</span><DialogTitle>{plugin.name}</DialogTitle><DialogDescription>{plugin.desp || "暂无插件描述"}</DialogDescription><div className="plugin-heading-badges"><Badge variant="outline">{plugin.id}</Badge><Badge variant="outline">v{plugin.version}</Badge>{plugin.vendor && <Badge variant="outline">{plugin.vendor}</Badge>}<Badge tone={trust.tone}>{trust.label}</Badge><Badge tone={status.tone}>{status.label}</Badge>{protocol.min !== undefined && <Badge variant="outline">协议 {protocol.min}-{protocol.max}</Badge>}</div></div></DialogHeader>
      <div className="modal-body plugin-detail-body">
        <TrustNotice plugin={plugin} trusting={trusting} onTrust={onTrust} />
        <section className="plugin-detail-meta-badges" aria-label="插件包与签名信息">
          <MetaBadge label={plugin.trust_state === "development" ? "运行来源" : "包文件"} value={plugin.package_name} mono />
          <MetaBadge label={plugin.trust_state === "development" ? "二进制 SHA-256" : "包 SHA-256"} value={plugin.package_sha256} mono wide />
          {plugin.signer_key_id && <MetaBadge label="签名 Key ID" value={plugin.signer_key_id} mono />}
          {plugin.signer_fingerprint && <MetaBadge label="签名指纹" value={plugin.signer_fingerprint} mono wide />}
        </section>
        <section className="plugin-detail-section">
          <div className="plugin-detail-section-heading"><h3>监控模块</h3>{plugin.url && <a href={plugin.url} target="_blank" rel="noreferrer">源码与发布地址<ExternalLink size={13} /></a>}</div>
          <div className="plugin-detail-modules">{(plugin.modules || []).map((module) => <div key={module.type}><strong>{module.name || module.type}</strong><code>{module.type}</code><span>模块 {module.version} · 配置 {module.config_version} · 结果 {module.result_schema_version}</span></div>)}</div>
        </section>
        <section className="plugin-detail-section">
          <div className="plugin-detail-section-heading"><h3>README</h3>{loading && <LoaderCircle className="spin" size={14} />}</div>
          <div className="plugin-readme">{loading ? "正在加载..." : <ReactMarkdown skipHtml components={{ a: ReadmeLink, img: ReadmeImage }}>{plugin.readme || "插件包未提供 README.md"}</ReactMarkdown>}</div>
        </section>
      </div>
      <DialogFooter><Button type="button" variant="outline" onClick={onClose}>关闭</Button></DialogFooter>
    </DialogContent>
  </Dialog>;
}

function TrustNotice({ plugin, trusting, onTrust }) {
  if (plugin.trust_state === "development") {
    return <div className="plugin-trust-notice is-warning"><ShieldCheck size={18} /><div><strong>正在使用本地插件源码</strong><span>当前二进制由开发启动流程直接构建，不对应可导出的签名插件包。</span></div><Badge tone="warning">开发模式</Badge></div>;
  }
  if (plugin.trust_state === "untrusted") {
    return <div className="plugin-trust-notice is-warning"><ShieldAlert size={18} /><div><strong>签名有效，发布者尚未信任</strong><span>确认前请从插件源码或发布页面核对签名指纹。信任后，同一公钥签名的后续插件将自动验证。</span></div><Button disabled={trusting} onClick={onTrust}>{trusting ? <LoaderCircle className="spin" size={15} /> : <KeyRound size={15} />}信任此发布者</Button></div>;
  }
  if (plugin.trust_state === "unsigned") {
    return <div className="plugin-trust-notice"><ShieldAlert size={18} /><div><strong>插件包未签名</strong><span>无法确认发布者身份，启用时仍需单独确认风险。</span></div><Badge tone="muted">未签名</Badge></div>;
  }
  return <div className="plugin-trust-notice is-trusted"><ShieldCheck size={18} /><div><strong>{plugin.trust_state === "official" ? "Meerkit 官方插件" : "已信任的插件发布者"}</strong><span>{plugin.signer_fingerprint || "随应用分发的官方插件"}</span></div><Badge tone="success">已验证</Badge></div>;
}

function MetaBadge({ label, value, mono = false, wide = false }) {
  return <span className={`plugin-meta-badge ${wide ? "is-wide" : ""}`}><span>{label}</span><strong className={mono ? "is-mono" : ""}>{value || "-"}</strong></span>;
}

function ReadmeLink({ node: _node, ...props }) {
  return <a {...props} target="_blank" rel="noreferrer" />;
}

function ReadmeImage({ node: _node, alt = "", ..._props }) {
  return <span className="plugin-readme-image">[图片：{alt || "未命名"}]</span>;
}
