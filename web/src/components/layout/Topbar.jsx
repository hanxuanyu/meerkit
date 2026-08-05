import React from "react";
import { ChevronRight, PanelLeft, RefreshCw } from "lucide-react";
import { Button } from "../ui/Button";
import { ThemeSwitcher } from "../theme/ThemeSwitcher";
import { pageTitle } from "../../lib/formatters";

export function Topbar({ activePage, onRefresh, sidebarCollapsed, onToggleSidebar }) {
  return <header className="topbar"><div className="breadcrumb"><Button variant="ghost" size="icon" className="sidebar-toggle" title={sidebarCollapsed ? "展开侧栏" : "折叠侧栏"} onClick={onToggleSidebar}><PanelLeft size={16} /></Button><span>Meerkit</span><ChevronRight size={15} /><strong>{pageTitle(activePage)}</strong></div><div className="topbar-actions"><ThemeSwitcher /><Button variant="ghost" size="icon" title="刷新数据" onClick={onRefresh}><RefreshCw size={16} /></Button><div className="avatar">MK</div></div></header>;
}
