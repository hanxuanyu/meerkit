import React from "react";
import { Activity, Bell, Layers3, Settings2 } from "lucide-react";
import { Button } from "../ui/Button";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../ui/Tooltip";

const navigationGroups = [
  { label: "监控", items: [{ id: "overview", label: "总览", icon: Activity }, { id: "monitors", label: "监控项", icon: Layers3 }] },
  { label: "通知与交付", items: [{ id: "notifications", label: "通知渠道", icon: Bell }] },
  { label: "系统", items: [{ id: "settings", label: "系统设置", icon: Settings2 }] }
];

export function Sidebar({ activePage, collapsed, onNavigate }) {
  return <TooltipProvider delayDuration={0}><aside className={`sidebar ${collapsed ? "collapsed" : ""}`}>
    <div className="brand"><div className="brand-mark"><img src="/brand-mark.png" alt="Meerkit" /></div><div><strong>Meerkit</strong><span>observability</span></div></div>
    <div className="sidebar-nav">{navigationGroups.map((group) => <section className="sidebar-group" key={group.label}><div className="sidebar-group-label">{group.label}</div><nav>{group.items.map((item) => <SidebarNavItem key={item.id} item={item} active={activePage === item.id} collapsed={collapsed} onNavigate={onNavigate} />)}</nav></section>)}</div>
    <div className="sidebar-footer"><div className="status-indicator"><span />服务运行中</div><span className="version">v0.1.0</span></div>
  </aside></TooltipProvider>;
}

function SidebarNavItem({ item, active, collapsed, onNavigate }) {
  const Icon = item.icon;
  const button = <Button aria-label={item.label} variant="ghost" className={`nav-item ${active ? "active" : ""}`} onClick={() => onNavigate(item.id)}><Icon size={17} /><span>{item.label}</span>{item.id === "notifications" && <span className="nav-count">·</span>}</Button>;
  if (!collapsed) return button;
  return <Tooltip><TooltipTrigger asChild>{button}</TooltipTrigger><TooltipContent side="right">{item.label}</TooltipContent></Tooltip>;
}
