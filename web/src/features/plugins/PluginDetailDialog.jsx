import React, { useState } from "react";
import { ChevronDown, ExternalLink, KeyRound, LoaderCircle, ShieldAlert, ShieldCheck } from "lucide-react";
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
          <div className="plugin-detail-section-heading"><h3>模块能力</h3>{plugin.url && <a href={plugin.url} target="_blank" rel="noreferrer">源码与发布地址<ExternalLink size={13} /></a>}</div>
          <div className="plugin-capability-modules">{(plugin.modules || []).map((module, index) => <ModuleCapability key={module.type} module={module} descriptor={(plugin.module_descriptors || []).find((item) => item.type === module.type)} defaultOpen={index === 0} loading={loading} />)}</div>
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

function ModuleCapability({ module, descriptor, defaultOpen, loading }) {
  const [open, setOpen] = useState(defaultOpen);
  const parameters = descriptor?.parameters || [];
  const resultSets = descriptor?.result_sets?.length
    ? descriptor.result_sets
    : descriptor?.fields?.length
      ? [{ key: "result", label: "执行结果", fields: descriptor.fields }]
      : [];
  const resultCount = resultSets.reduce((total, set) => total + (set.fields?.length || 0), 0);
  return <details className="plugin-capability-module" open={open} onToggle={(event) => setOpen(event.currentTarget.open)}>
    <summary>
      <span className="plugin-capability-identity"><strong>{descriptor?.name || module.name || module.type}</strong><span>{descriptor?.description || "插件提供的监控采集模块"}</span></span>
      <span className="plugin-capability-summary"><Badge variant="outline">{module.type}</Badge><Badge variant="outline">模块 {module.version}</Badge><Badge tone="muted">输入 {parameters.length}</Badge><Badge tone="muted">输出 {resultCount}</Badge></span>
      <ChevronDown className="plugin-capability-chevron" size={15} />
    </summary>
    {descriptor ? <div className="plugin-capability-columns">
      <CapabilityGroup title="输入参数" count={parameters.length}>{parameters.length ? <div className="plugin-capability-fields">{parameters.map((parameter) => <ParameterRow key={parameter.key} parameter={parameter} />)}</div> : <CapabilityEmpty>无需配置参数</CapabilityEmpty>}</CapabilityGroup>
      <CapabilityGroup title="返回结果" count={resultCount}>{resultSets.length ? <div className="plugin-result-sets">{resultSets.map((set) => <div className="plugin-result-set" key={set.key}><div className="plugin-result-set-heading"><strong>{set.label || set.key}</strong><code>{set.key}</code>{set.description && <span>{set.description}</span>}</div><div className="plugin-capability-fields">{(set.fields || []).map((field) => <ResultFieldRow key={`${set.key}:${field.name}`} field={field} />)}</div></div>)}</div> : <CapabilityEmpty>未声明结构化返回字段</CapabilityEmpty>}</CapabilityGroup>
    </div> : <CapabilityEmpty className="plugin-capability-unavailable">{loading ? "正在加载模块能力..." : "该模块尚无描述快照，成功启用插件后即可查看输入输出能力。"}</CapabilityEmpty>}
  </details>;
}

function CapabilityGroup({ title, count, children }) {
  return <section className="plugin-capability-group"><div className="plugin-capability-group-heading"><h4>{title}</h4><span>{count} 项</span></div>{children}</section>;
}

function ParameterRow({ parameter }) {
  const metadata = [];
  if (parameter.required) metadata.push("必填");
  if (parameter.secret) metadata.push("敏感");
  if (parameter.default !== undefined && parameter.default !== null && parameter.default !== "") metadata.push(`默认 ${compactValue(parameter.default)}`);
  if (parameter.options?.length) metadata.push(`${parameter.options.length} 个选项`);
  if (parameter.unit) metadata.push(parameter.unit);
  return <div className="plugin-capability-field" title={parameter.description || ""}><div><strong>{parameter.label || parameter.key}</strong><code>{parameter.key}</code></div><div className="plugin-capability-field-meta"><Badge variant="outline">{parameter.type || "string"}</Badge>{metadata.map((item) => <span key={item}>{item}</span>)}</div></div>;
}

function ResultFieldRow({ field }) {
  const metadata = [];
  if (field.unit) metadata.push(field.unit);
  if (field.path) metadata.push("支持路径");
  if (field.operators?.length) metadata.push(`${field.operators.length} 种比较`);
  return <div className="plugin-capability-field" title={field.description || ""}><div><strong>{field.label || field.name}</strong><code>{field.name}</code></div><div className="plugin-capability-field-meta"><Badge variant="outline">{field.type || "unknown"}</Badge>{field.format && <span>{field.format}</span>}{metadata.map((item) => <span key={item}>{item}</span>)}</div></div>;
}

function CapabilityEmpty({ className = "", children }) {
  return <div className={`plugin-capability-empty ${className}`}>{children}</div>;
}

function compactValue(value) {
  if (typeof value === "boolean") return value ? "是" : "否";
  if (typeof value === "object") return "已配置";
  const text = String(value);
  return text.length > 18 ? `${text.slice(0, 18)}...` : text;
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
