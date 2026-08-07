import React, { useEffect, useState } from "react";
import { Activity, Bell, ChartNoAxesColumnIncreasing, Inbox, Layers3, PackageOpen, ScrollText, Settings2, X } from "lucide-react";
import { Button } from "../ui/Button";
import { IconButton } from "../ui/IconButton";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../ui/Tooltip";

const navigationGroups = [
  { label: "监控", items: [{ id: "overview", label: "总览", icon: Activity }, { id: "monitors", label: "监控项", icon: Layers3 }, { id: "statusBoard", label: "状态看板", icon: ChartNoAxesColumnIncreasing }] },
  { label: "通知", items: [{ id: "inbox", label: "通知中心", icon: Inbox }, { id: "notifications", label: "通知渠道", icon: Bell }] },
  { label: "系统", items: [{ id: "plugins", label: "监控插件", icon: PackageOpen }, { id: "logs", label: "日志", icon: ScrollText }, { id: "settings", label: "系统设置", icon: Settings2 }] }
];

export function Sidebar({ activePage, collapsed, mobileOpen = false, onCloseMobile, onNavigate, unreadCount = 0 }) {
  const [tooltipReady, setTooltipReady] = useState(false);
  const [openTooltip, setOpenTooltip] = useState("");

  useEffect(() => {
    if (!mobileOpen) return undefined;
    const closeOnEscape = (event) => { if (event.key === "Escape") onCloseMobile(); };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [mobileOpen, onCloseMobile]);

  useEffect(() => {
    setTooltipReady(false);
    setOpenTooltip("");
    if (!collapsed || window.matchMedia("(max-width: 640px)").matches) return undefined;
    const timer = window.setTimeout(() => setTooltipReady(true), 240);
    return () => window.clearTimeout(timer);
  }, [collapsed]);

  const tooltipsEnabled = collapsed && tooltipReady;

  return <TooltipProvider delayDuration={280} skipDelayDuration={100} disableHoverableContent><>
    <button type="button" className={`mobile-sidebar-backdrop ${mobileOpen ? "is-open" : ""}`} aria-label="关闭导航菜单" tabIndex={mobileOpen ? 0 : -1} onClick={onCloseMobile} />
    <aside className={`sidebar ${collapsed ? "collapsed" : ""} ${mobileOpen ? "mobile-open" : ""}`}>
    <div className="brand"><div className="brand-mark"><img src="/brand-mark.png" alt="Meerkit" /></div><div className="brand-copy"><strong>Meerkit</strong><span>observability</span></div><IconButton className="mobile-sidebar-close" size="default" title="关闭导航菜单" aria-label="关闭导航菜单" onClick={onCloseMobile}><X size={17} /></IconButton></div>
    <div className="sidebar-nav">{navigationGroups.map((group) => <section className="sidebar-group" key={group.label}><div className="sidebar-group-label">{group.label}</div><nav>{group.items.map((item) => <SidebarNavItem key={item.id} item={item} active={activePage === item.id} tooltipEnabled={tooltipsEnabled} tooltipOpen={tooltipsEnabled && openTooltip === item.id} unreadCount={unreadCount} onNavigate={onNavigate} onTooltipOpenChange={(open) => setOpenTooltip((current) => open ? item.id : current === item.id ? "" : current)} />)}</nav></section>)}</div>
    <div className="sidebar-footer"><div className="status-indicator"><span />服务运行中</div><span className="version">v0.1.0</span></div>
    </aside>
  </></TooltipProvider>;
}

function SidebarNavItem({ item, active, tooltipEnabled, tooltipOpen, unreadCount, onNavigate, onTooltipOpenChange }) {
  const Icon = item.icon;
  const button = <Button aria-label={item.label} variant="ghost" className={`nav-item ${active ? "active" : ""}`} onClick={() => onNavigate(item.id)}><Icon size={17} /><span className="nav-item-label">{item.label}</span>{item.id === "inbox" && unreadCount > 0 && <span className="nav-badge">{unreadCount > 99 ? "99+" : unreadCount}</span>}</Button>;
  return <Tooltip open={tooltipOpen} onOpenChange={(open) => { if (tooltipEnabled) onTooltipOpenChange(open); }}><TooltipTrigger asChild>{button}</TooltipTrigger>{tooltipEnabled && <TooltipContent side="right">{item.label}</TooltipContent>}</Tooltip>;
}
