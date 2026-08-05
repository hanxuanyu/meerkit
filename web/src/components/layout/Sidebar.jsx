import React, { useEffect } from "react";
import { Activity, Bell, Inbox, Layers3, Settings2, X } from "lucide-react";
import { Button } from "../ui/Button";
import { IconButton } from "../ui/IconButton";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../ui/Tooltip";

const navigationGroups = [
  { label: "监控", items: [{ id: "overview", label: "总览", icon: Activity }, { id: "monitors", label: "监控项", icon: Layers3 }] },
  { label: "通知与交付", items: [{ id: "inbox", label: "通知中心", icon: Inbox }, { id: "notifications", label: "通知渠道", icon: Bell }] },
  { label: "系统", items: [{ id: "settings", label: "系统设置", icon: Settings2 }] }
];

export function Sidebar({ activePage, collapsed, mobileOpen = false, onCloseMobile, onNavigate, unreadCount = 0 }) {
  useEffect(() => {
    if (!mobileOpen) return undefined;
    const closeOnEscape = (event) => { if (event.key === "Escape") onCloseMobile(); };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [mobileOpen, onCloseMobile]);

  return <TooltipProvider delayDuration={0}><>
    <button type="button" className={`mobile-sidebar-backdrop ${mobileOpen ? "is-open" : ""}`} aria-label="关闭导航菜单" tabIndex={mobileOpen ? 0 : -1} onClick={onCloseMobile} />
    <aside className={`sidebar ${collapsed ? "collapsed" : ""} ${mobileOpen ? "mobile-open" : ""}`}>
    <div className="brand"><div className="brand-mark"><img src="/brand-mark.png" alt="Meerkit" /></div><div><strong>Meerkit</strong><span>observability</span></div><IconButton className="mobile-sidebar-close" size="default" title="关闭导航菜单" aria-label="关闭导航菜单" onClick={onCloseMobile}><X size={17} /></IconButton></div>
    <div className="sidebar-nav">{navigationGroups.map((group) => <section className="sidebar-group" key={group.label}><div className="sidebar-group-label">{group.label}</div><nav>{group.items.map((item) => <SidebarNavItem key={item.id} item={item} active={activePage === item.id} collapsed={collapsed} unreadCount={unreadCount} onNavigate={onNavigate} />)}</nav></section>)}</div>
    <div className="sidebar-footer"><div className="status-indicator"><span />服务运行中</div><span className="version">v0.1.0</span></div>
    </aside>
  </></TooltipProvider>;
}

function SidebarNavItem({ item, active, collapsed, unreadCount, onNavigate }) {
  const Icon = item.icon;
  const button = <Button aria-label={item.label} variant="ghost" className={`nav-item ${active ? "active" : ""}`} onClick={() => onNavigate(item.id)}><Icon size={17} /><span>{item.label}</span>{item.id === "inbox" && unreadCount > 0 && <span className="nav-badge">{unreadCount > 99 ? "99+" : unreadCount}</span>}</Button>;
  if (!collapsed) return button;
  return <Tooltip><TooltipTrigger asChild>{button}</TooltipTrigger><TooltipContent side="right">{item.label}</TooltipContent></Tooltip>;
}
