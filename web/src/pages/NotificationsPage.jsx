import React, { useEffect, useMemo, useState } from "react";
import { Bell, BellRing, Copy, Edit3, ExternalLink, Inbox, LockKeyhole, Plus, RefreshCw, Search } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { ActionMenu } from "../components/ui/ActionMenu";
import { Button } from "../components/ui/Button";
import { Badge } from "../components/ui/Badge";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { IconButton } from "../components/ui/IconButton";
import { Input } from "../components/ui/Input";
import { Pagination } from "../components/ui/Pagination";
import { Switch } from "../components/ui/Switch";

export function NotificationsPage({ channels = [], onCreate, onRefresh, onEdit, onDuplicate, onToggleEnabled, togglingChannelId }) {
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
    <PageHeader className="page-heading-with-action" eyebrow="DELIVERY CHANNELS" title="通知渠道" description="在条件边沿触发时发送告警和恢复通知。" actions={<Button onClick={onCreate}><Plus size={16} />添加渠道</Button>} />
    <Card className="section-card">
      <div className="section-header">
        <div><h2>已配置渠道</h2><p>内置站内通知及外部渠道可被多个监控项复用。</p></div>
        <IconButton variant="outline" size="default" title="刷新" aria-label="刷新" onClick={onRefresh}><RefreshCw size={15} /></IconButton>
      </div>
      {hasChannels && <div className="channel-toolbar">
        <div className="channel-search"><Search size={15} /><Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索渠道名称或类型" aria-label="搜索通知渠道" /></div>
      </div>}
      {hasMatches ? <>
        <div className="channel-list-head"><span>通知渠道</span><span>状态</span><span>操作</span></div>
        <div className="channel-list">{visibleChannels.map((channel) => <div className="channel-row" key={channel.id}>
          <div className="channel-identity"><div className="channel-icon">{channel.notifier_type === "inapp" ? <BellRing size={17} /> : channel.notifier_type === "smtp" ? <Inbox size={17} /> : <ExternalLink size={17} />}</div><div className="channel-main"><strong title={channel.name}>{channel.name}</strong><span className="channel-desktop-meta">{channel.notifier_type.toUpperCase()} · {channel.built_in ? "内置渠道" : "自定义渠道"}</span><span className="channel-mobile-badges"><Badge variant="outline">{channel.notifier_type.toUpperCase()}</Badge><Badge tone={channel.enabled ? "success" : "muted"}>{channel.enabled ? "运行中" : "已停用"}</Badge><Badge variant="outline" tone="muted">{channel.built_in ? "内置" : "自定义"}</Badge></span></div></div>
          <Badge className="channel-status" tone={channel.enabled ? "success" : "muted"}>{channel.enabled ? "运行中" : "已停用"}</Badge>
          <div className="channel-actions"><span className="channel-switch" title={channel.enabled ? "停用通知渠道" : "启用通知渠道"}><Switch checked={channel.enabled} disabled={togglingChannelId === channel.id} aria-label={channel.enabled ? "停用通知渠道" : "启用通知渠道"} onCheckedChange={() => onToggleEnabled(channel)} /></span>{channel.built_in ? <span className="channel-built-in-lock" title="内置渠道配置不可修改" aria-label="内置渠道配置不可修改"><LockKeyhole size={14} /></span> : <ActionMenu label={`${channel.name} 操作`} items={[{ label: "编辑", icon: Edit3, onSelect: () => onEdit(channel) }, { label: "复制", icon: Copy, onSelect: () => onDuplicate(channel) }]} />}</div>
        </div>)}</div>
        <Pagination page={pageInfo.page} pageSize={pageInfo.pageSize} total={filteredChannels.length} onPageChange={(page) => setPageInfo((current) => ({ ...current, page }))} onPageSizeChange={(pageSize) => setPageInfo({ page: 1, pageSize })} />
      </> : hasChannels ? <EmptyState icon={Search} title="没有匹配的通知渠道" description="尝试调整搜索关键词。" /> : <EmptyState icon={Bell} title="还没有通知渠道" description="添加外部渠道，让变化及时抵达。" action={<Button onClick={onCreate}><Plus size={16} />添加渠道</Button>} />}
    </Card>
  </div>;
}
