import React from "react";
import { LogOut, PanelLeft, RefreshCw } from "lucide-react";
import { IconButton } from "../ui/IconButton";
import { ThemeSwitcher } from "../theme/ThemeSwitcher";
import { pageTitle } from "../../lib/formatters";
import { usePageChrome } from "./PageChrome";

export function Topbar({ activePage, mobileHidden = false, onLogout, onRefresh, onToggleSidebar, notificationBell }) {
  const { setTitleHost } = usePageChrome();

  return <header className={`topbar ${mobileHidden ? "mobile-hidden" : ""}`}>
    <div className="topbar-page"><IconButton variant="ghost" size="default" className="sidebar-toggle" title="切换导航菜单" aria-label="切换导航菜单" onClick={onToggleSidebar}><PanelLeft size={16} /></IconButton><div ref={setTitleHost} className="topbar-page-context"><div className="topbar-page-fallback"><h1>{pageTitle(activePage)}</h1></div></div></div>
    <div className="topbar-actions"><ThemeSwitcher />{notificationBell}<IconButton variant="ghost" size="default" className="topbar-refresh" title="刷新数据" aria-label="刷新数据" onClick={onRefresh}><RefreshCw size={16} /></IconButton><IconButton variant="ghost" size="default" title="退出登录" aria-label="退出登录" onClick={onLogout}><LogOut size={16} /></IconButton></div>
  </header>;
}
