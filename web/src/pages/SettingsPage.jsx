import React from "react";
import { Database, Server } from "lucide-react";
import { Card } from "../components/ui/Card";

export function SettingsPage() {
  return <div className="page-stack"><div className="page-heading"><div><div className="eyebrow">RUNTIME CONFIGURATION</div><h1>系统设置</h1><p>服务配置通过 config.yaml、环境变量和命令行管理。</p></div></div><div className="settings-grid"><Card className="settings-card"><div className="settings-icon"><Server size={18} /></div><div><h2>单二进制服务</h2><p>Web 资源已嵌入 Go 服务，默认监听 127.0.0.1:8080。</p><code>MEERKIT_SERVER__PORT</code></div></Card><Card className="settings-card"><div className="settings-icon"><Database size={18} /></div><div><h2>SQLite 存储</h2><p>监控配置、执行结果和通知渠道使用轻量化本地数据库保存。</p><code>MEERKIT_STORAGE__DATA_DIR</code></div></Card></div></div>;
}
