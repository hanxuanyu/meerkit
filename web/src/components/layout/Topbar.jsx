import React from "react";
import { ChevronRight, PanelLeft, RefreshCw } from "lucide-react";
import { IconButton } from "../ui/IconButton";
import { ThemeSwitcher } from "../theme/ThemeSwitcher";
import { pageTitle } from "../../lib/formatters";

export function Topbar({ activePage, mobileHidden = false, onRefresh, sidebarCollapsed, onToggleSidebar, notificationBell }) {
  return <header className={`topbar ${mobileHidden ? "mobile-hidden" : ""}`}><div className="breadcrumb"><IconButton variant="ghost" size="default" className="sidebar-toggle" title="切换导航菜单" aria-label="切换导航菜单" onClick={onToggleSidebar}><PanelLeft size={16} /></IconButton><span>Meerkit</span><ChevronRight size={15} /><strong>{pageTitle(activePage)}</strong></div><div className="topbar-actions"><ThemeSwitcher />{notificationBell}<IconButton variant="ghost" size="default" className="topbar-refresh" title="刷新数据" aria-label="刷新数据" onClick={onRefresh}><RefreshCw size={16} /></IconButton><div className="avatar">MK</div></div></header>;
}
