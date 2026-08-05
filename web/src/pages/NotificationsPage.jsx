import React from "react";
import { Bell, ExternalLink, Inbox, Plus, RefreshCw } from "lucide-react";
import { Button } from "../components/ui/Button";
import { Badge } from "../components/ui/Badge";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { IconButton } from "../components/ui/IconButton";

export function NotificationsPage({ channels, onCreate, onRefresh, onTest }) {
  return <div className="page-stack"><div className="page-heading"><div><div className="eyebrow">DELIVERY CHANNELS</div><h1>通知渠道</h1><p>在条件边沿触发时发送告警和恢复通知。</p></div><Button onClick={onCreate}><Plus size={16} />添加渠道</Button></div><Card className="section-card"><div className="section-header"><div><h2>已配置渠道</h2><p>Webhook 和 SMTP 渠道可被多个监控项复用。</p></div><IconButton variant="outline" size="default" title="刷新" aria-label="刷新" onClick={onRefresh}><RefreshCw size={15} /></IconButton></div>{channels.length ? <div className="channel-list">{channels.map((channel) => <div className="channel-row" key={channel.id}><div className="channel-icon">{channel.notifier_type === "smtp" ? <Inbox size={17} /> : <ExternalLink size={17} />}</div><div className="channel-main"><strong>{channel.name}</strong><span>{channel.notifier_type.toUpperCase()} · {channel.enabled ? "已启用" : "已停用"}</span></div><Badge tone={channel.enabled ? "success" : "muted"}>{channel.enabled ? "运行中" : "已停用"}</Badge><Button variant="outline" onClick={() => onTest(channel)}><Bell size={14} />测试</Button></div>)}</div> : <EmptyState icon={Bell} title="还没有通知渠道" description="添加 Webhook 或 SMTP，让变化及时抵达。" action={<Button onClick={onCreate}><Plus size={16} />添加渠道</Button>} />}</Card></div>;
}
