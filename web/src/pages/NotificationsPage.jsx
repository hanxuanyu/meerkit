import React, { useEffect, useMemo, useState } from "react";
import { Bell, Edit3, ExternalLink, Inbox, Plus, RefreshCw, Search } from "lucide-react";
import { Button } from "../components/ui/Button";
import { Badge } from "../components/ui/Badge";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { IconButton } from "../components/ui/IconButton";
import { Input } from "../components/ui/Input";
import { Pagination } from "../components/ui/Pagination";
import { Switch } from "../components/ui/Switch";

export function NotificationsPage({ channels = [], onCreate, onRefresh, onTest, onEdit, onToggleEnabled, togglingChannelId }) {
  const channelList = Array.isArray(channels) ? channels : [];
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [pageInfo, setPageInfo] = useState({ page: 1, pageSize: 20 });

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(searchInput.trim().toLowerCase());
      setPageInfo((current) => current.page === 1 ? current : { ...current, page: 1 });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  const filteredChannels = useMemo(() => {
    if (!search) return channelList;
    return channelList.filter((channel) => {
      const searchable = [
        channel.name,
        channel.notifier_type,
        channel.enabled ? "已启用 运行中 enabled" : "已停用 disabled"
      ].filter(Boolean).join(" ").toLowerCase();
      return searchable.includes(search);
    });
  }, [channelList, search]);

  useEffect(() => {
    const totalPages = Math.max(1, Math.ceil(filteredChannels.length / pageInfo.pageSize));
    setPageInfo((current) => current.page > totalPages ? { ...current, page: totalPages } : current);
  }, [filteredChannels.length, pageInfo.pageSize]);

  const visibleChannels = useMemo(() => {
    const start = (pageInfo.page - 1) * pageInfo.pageSize;
    return filteredChannels.slice(start, start + pageInfo.pageSize);
  }, [filteredChannels, pageInfo.page, pageInfo.pageSize]);

  const hasChannels = channelList.length > 0;
  const hasMatches = filteredChannels.length > 0;

  return <div className="page-stack">
    <div className="page-heading">
      <div><div className="eyebrow">DELIVERY CHANNELS</div><h1>通知渠道</h1><p>在条件边沿触发时发送告警和恢复通知。</p></div>
      <Button onClick={onCreate}><Plus size={16} />添加渠道</Button>
    </div>
    <Card className="section-card">
      <div className="section-header">
        <div><h2>已配置渠道</h2><p>Webhook 和 SMTP 渠道可被多个监控项复用。</p></div>
        <IconButton variant="outline" size="default" title="刷新" aria-label="刷新" onClick={onRefresh}><RefreshCw size={15} /></IconButton>
      </div>
      {hasChannels && <div className="channel-toolbar">
        <div className="channel-search"><Search size={15} /><Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索渠道名称或类型" aria-label="搜索通知渠道" /></div>
      </div>}
      {hasMatches ? <>
        <div className="channel-list-head"><span>通知渠道</span><span>状态</span><span>测试</span><span>编辑</span><span>启用</span></div>
        <div className="channel-list">{visibleChannels.map((channel) => <div className="channel-row" key={channel.id}>
          <div className="channel-icon">{channel.notifier_type === "smtp" ? <Inbox size={17} /> : <ExternalLink size={17} />}</div>
          <div className="channel-main"><strong title={channel.name}>{channel.name}</strong><span>{channel.notifier_type.toUpperCase()} · {channel.enabled ? "已启用" : "已停用"}</span></div>
          <Badge tone={channel.enabled ? "success" : "muted"}>{channel.enabled ? "运行中" : "已停用"}</Badge>
          <Button variant="outline" onClick={() => onTest(channel)}><Bell size={14} />测试</Button>
          <div className="row-actions"><IconButton size="sm" title="编辑渠道" aria-label="编辑渠道" onClick={() => onEdit(channel)}><Edit3 size={15} /></IconButton></div><span className="channel-switch" title={channel.enabled ? "停用通知渠道" : "启用通知渠道"}><Switch checked={channel.enabled} disabled={togglingChannelId === channel.id} aria-label={channel.enabled ? "停用通知渠道" : "启用通知渠道"} onCheckedChange={() => onToggleEnabled(channel)} /></span>
        </div>)}</div>
        <Pagination page={pageInfo.page} pageSize={pageInfo.pageSize} total={filteredChannels.length} onPageChange={(page) => setPageInfo((current) => ({ ...current, page }))} onPageSizeChange={(pageSize) => setPageInfo({ page: 1, pageSize })} />
      </> : hasChannels ? <EmptyState icon={Search} title="没有匹配的通知渠道" description="尝试调整搜索关键词。" /> : <EmptyState icon={Bell} title="还没有通知渠道" description="添加 Webhook 或 SMTP，让变化及时抵达。" action={<Button onClick={onCreate}><Plus size={16} />添加渠道</Button>} />}
    </Card>
  </div>;
}
