import React from "react";

export function AppShell({ sidebar, topbar, tabs, sidebarCollapsed = false, children, overlays }) {
  return <div className={`app-shell ${sidebarCollapsed ? "sidebar-collapsed" : ""}`}>
    {sidebar}
    <main className="main-content">
      {topbar}
      {tabs}
      <div className="page-content">{children}</div>
    </main>
    {overlays}
  </div>;
}
