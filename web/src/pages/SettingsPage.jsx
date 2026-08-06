import React, { useEffect, useState } from "react";
import { FileCog } from "lucide-react";
import { api } from "../lib/api";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge } from "../components/ui/Badge";
import { Card } from "../components/ui/Card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../components/ui/Table";

const sourceLabels = {
  command_line: "命令行",
  environment: "环境变量",
  config_file: "config.yaml",
  default: "默认值"
};

export function SettingsPage() {
  const [metadata, setMetadata] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    api("/api/v1/system/config")
      .then((value) => { if (!cancelled) setMetadata(value); })
      .catch((loadError) => { if (!cancelled) setError(loadError.message); });
    return () => { cancelled = true; };
  }, []);

  return <div className="page-stack"><PageHeader eyebrow="RUNTIME CONFIGURATION" title="系统设置" description="配置在服务启动时加载，当前生效值由默认值、config.yaml、环境变量和命令行参数合并而成。" /><Card className="section-card settings-config-card"><div className="section-header"><div><h2>当前配置</h2><p>{metadata?.config_file ? `配置文件：${metadata.config_file}` : "未加载 config.yaml，当前使用默认值和运行时覆盖。"}</p></div><div className="settings-config-icon"><FileCog size={17} /></div></div>{error ? <div className="settings-config-error">{error}</div> : metadata ? <ConfigTable items={metadata.items || []} /> : <div className="records-empty">正在加载配置...</div>}</Card></div>;
}

function ConfigTable({ items }) {
  return <Table><TableHeader><TableRow><TableHead>配置项</TableHead><TableHead>描述</TableHead><TableHead>默认值</TableHead><TableHead>生效值</TableHead><TableHead>来源</TableHead></TableRow></TableHeader><TableBody>{items.map((item) => { const defaultValue = formatValue(item.default); const effectiveValue = formatValue(item.value); return <TableRow key={item.path}><TableCell><div className="config-path"><code title={item.path}>{item.path}</code></div></TableCell><TableCell><span className="config-description" title={item.description}>{item.description}</span></TableCell><TableCell><code className="config-value config-default" title={defaultValue}>{defaultValue}</code></TableCell><TableCell><code className="config-value" title={effectiveValue}>{effectiveValue}</code></TableCell><TableCell><Badge variant="outline">{sourceLabels[item.source] || item.source}</Badge></TableCell></TableRow>; })}</TableBody></Table>;
}

function formatValue(value) {
  if (typeof value === "boolean") return value ? "true" : "false";
  if (value === null || value === undefined || value === "") return "-";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
