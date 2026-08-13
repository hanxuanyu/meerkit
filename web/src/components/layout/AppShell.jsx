import React from "react";
import { PageChromeProvider } from "./PageChrome";

export function AppShell({ sidebar, topbar, tabs, sidebarCollapsed = false, activePage = "", contentRef, onContentScroll, children, overlays }) {
  return <PageChromeProvider activePage={activePage}><div className={`app-shell ${sidebarCollapsed ? "sidebar-collapsed" : ""}`}>
    {sidebar}
    <main className="main-content">
      {topbar}
      {tabs}
      <div ref={contentRef} className="page-content" onScroll={onContentScroll}>{children}</div>
    </main>
    {overlays}
  </div></PageChromeProvider>;
}
