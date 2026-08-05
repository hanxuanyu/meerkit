import React from "react";
import { ChevronRight, PanelLeft, RefreshCw } from "lucide-react";
import { IconButton } from "../ui/IconButton";
import { ThemeSwitcher } from "../theme/ThemeSwitcher";
import { pageTitle } from "../../lib/formatters";

export function Topbar({ activePage, onRefresh, sidebarCollapsed, onToggleSidebar }) {
  return <header className="topbar"><div className="breadcrumb"><IconButton variant="ghost" size="default" className="sidebar-toggle" title={sidebarCollapsed ? "展开侧栏" : "折叠侧栏"} aria-label={sidebarCollapsed ? "展开侧栏" : "折叠侧栏"} onClick={onToggleSidebar}><PanelLeft size={16} /></IconButton><span>Meerkit</span><ChevronRight size={15} /><strong>{pageTitle(activePage)}</strong></div><div className="topbar-actions"><ThemeSwitcher /><IconButton variant="ghost" size="default" title="刷新数据" aria-label="刷新数据" onClick={onRefresh}><RefreshCw size={16} /></IconButton><div className="avatar">MK</div></div></header>;
}
